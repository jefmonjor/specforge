package tddloop

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"specforge/internal/app/clarify"
	"specforge/internal/app/layout"
	"specforge/internal/domain/quality"
	"specforge/internal/domain/risk"
	"specforge/internal/domain/spec"
	"specforge/internal/domain/tdd"
	"specforge/internal/ports"
)

// batch returns the scenarios (indexes) that can run side by side from
// the current one: consecutive pending scenarios at RED whose plan
// surfaces overlap none of the others, at most Parallel of them. Fewer
// than two means one at a time.
func (s *Service) batch(r *run) []int {
	if r.o.Parallel < 2 || r.o.Single || s.d.Scratch == nil || r.plan == "" ||
		r.st.Phase != tdd.PhaseRed || r.st.Pending != nil || r.st.ReviewNote != "" {
		return nil
	}
	markers := make([]string, len(r.st.Scenarios))
	for i, sc := range r.st.Scenarios {
		markers[i] = sc.Marker
	}
	surfaces := spec.ScenarioSurfaces(r.plan, markers)
	var out []int
	for i := r.st.Current; i < len(r.st.Scenarios) && len(out) < r.o.Parallel; i++ {
		sc := r.st.Scenarios[i]
		if sc.Done || r.sequential[i] || slices.ContainsFunc(out, func(j int) bool {
			return surfaces[sc.Marker].Overlaps(surfaces[r.st.Scenarios[j].Marker])
		}) {
			break
		}
		out = append(out, i)
	}
	if len(out) < 2 {
		return nil
	}
	return out
}

// child is a scenario of a batch, run in its own sandbox.
type child struct {
	index int
	box   ports.Copies
	st    *tdd.State
	err   error
	// logs holds the length of each project log when the sandbox was made:
	// what the child wrote after it is what it brings back.
	logs map[string]int
}

// runBatch runs the scenarios of a batch side by side, each in a sandbox
// of the project, then brings their work back one by one, in order.
func (s *Service) runBatch(ctx context.Context, r *run, batch []int) error {
	var markers []string
	for _, i := range batch {
		markers = append(markers, r.st.Scenarios[i].Marker)
	}
	r.st.Record("parallel", "started", strings.Join(markers, ", "), s.d.Now())
	s.d.Events.Parallel(markers)

	var mu sync.Mutex
	asked := &lockedPrompter{mu: &mu, p: s.d.Asker.Prompter}
	events := &quietEvents{mu: &mu, e: s.d.Events}
	logs := s.logLengths(r)
	children := make([]child, len(batch))
	var wg sync.WaitGroup
	for n, i := range batch {
		wg.Add(1)
		go func() {
			defer wg.Done()
			children[n] = s.runChild(ctx, r, i, asked, events)
			children[n].logs = logs
		}()
	}
	wg.Wait()
	defer func() {
		for _, c := range children {
			if c.box.Remove != nil {
				_ = c.box.Remove()
			}
		}
	}()
	return s.integrate(ctx, r, children)
}

// runChild runs one scenario, alone, in a sandbox: no developer review
// and no commit there; both happen once its work is back.
func (s *Service) runChild(ctx context.Context, r *run, i int, asked ports.Prompter, events Events) child {
	c := child{index: i}
	c.box, c.err = s.d.Scratch.Sandbox(ctx, r.o.Root)
	if c.err != nil {
		return c
	}
	d := s.d
	d.Asker = &clarify.Asker{Prompter: asked, Files: s.d.Asker.Files, Now: s.d.Asker.Now, Lang: s.d.Asker.Lang}
	// A sandbox is disposable: no checkpoints there.
	d.Events, d.Scratch, d.Checkpoints = events, nil, nil
	o := r.o
	o.Root = c.box.Dir
	o.SpecPath = filepath.Join(c.box.Dir, filepath.FromSlash(r.lay.Rel(r.o.SpecPath)))
	o.Scenario, o.From, o.Single = i+1, "", true
	o.Restart, o.Resume = true, false
	o.Review, o.Commit, o.Parallel = ReviewOff, false, 1
	o.Seed = r.st.Baseline
	c.st, c.err = New(d).Run(ctx, o)
	return c
}

// arrival is a scenario whose work came back: its files and what they
// replaced, so it can be taken back out.
type arrival struct {
	index int
	ref   tdd.ScenarioRef
	child *tdd.State
	// before holds each file's content in the project before; nil when the
	// file did not exist.
	before map[string][]byte
}

// integrate brings the batch's work into the project in scenario order,
// runs the whole suite once over the combination (the seam check), and
// closes each scenario in turn: the developer's review and the commit
// stay sequential. A scenario that could not finish, whose files collide
// with another's, or that the seam check rejects runs again on its own.
func (s *Service) integrate(ctx context.Context, r *run, children []child) error {
	if r.sequential == nil {
		r.sequential = map[int]bool{}
	}
	var arrived []arrival
	taken := map[string]bool{}
	for _, c := range children {
		ref, ok := s.finished(r, c)
		if !ok {
			// The decisions the developer took there still count; the
			// attempt itself runs again, so its process decisions do not.
			if c.box.Dir != "" {
				if err := s.bringLogs(r, c, false); err != nil {
					return err
				}
			}
			continue
		}
		if clash := slices.DeleteFunc(r.workFiles(ref), func(f string) bool { return !taken[f] }); len(clash) > 0 {
			r.sequential[c.index] = true
			s.d.Events.ParallelSkipped(ref, "its files collide with another scenario's: "+joinPaths(clash))
			continue
		}
		a, err := s.bringBack(r, c, ref)
		if err != nil {
			return err
		}
		for f := range a.before {
			taken[f] = true
		}
		arrived = append(arrived, a)
	}
	if len(arrived) == 0 {
		return nil
	}
	if failure, err := s.runSuite(ctx, r); err != nil || failure != "" {
		for _, a := range arrived {
			r.sequential[a.index] = true
		}
		if rerr := s.takeBack(r, arrived); rerr != nil || err != nil {
			return errors.Join(err, rerr)
		}
		r.st.Record("parallel", "seam check failed", failure, s.d.Now())
		s.d.Events.SeamFailed(failure)
		return nil
	}
	for n, a := range arrived {
		if err := s.closeArrival(ctx, r, a); err != nil || !r.st.Scenarios[a.index].Done {
			// Pending review or sent back: the rest goes back out and
			// runs again later, in turn.
			for _, later := range arrived[n+1:] {
				r.sequential[later.index] = true
			}
			return errors.Join(err, s.takeBack(r, arrived[n+1:]))
		}
	}
	return nil
}

// finished returns the scenario a child completed, or records why not.
func (s *Service) finished(r *run, c child) (tdd.ScenarioRef, bool) {
	sc := r.st.Scenarios[c.index]
	if c.err == nil && c.st != nil && c.st.Scenarios[c.index].Done {
		return c.st.Scenarios[c.index], true
	}
	r.sequential[c.index] = true
	why := "it did not finish"
	if c.err != nil {
		// Run on its own, right after the batch, it stops (or asks) again
		// with its state in the project, where --resume finds it.
		why = c.err.Error()
	}
	r.st.Record("parallel", "not integrated", sc.Marker+": "+why, s.d.Now())
	s.d.Events.ParallelSkipped(sc, why)
	return tdd.ScenarioRef{}, false
}

// bringBack copies a child's files and records into the project.
func (s *Service) bringBack(r *run, c child, ref tdd.ScenarioRef) (arrival, error) {
	a := arrival{index: c.index, ref: ref, child: c.st, before: map[string][]byte{}}
	box := sandboxLayout(r, c)
	for _, f := range r.workFiles(ref) {
		a.before[f], _ = s.d.Files.ReadFile(r.lay.Abs(f)) // nil: it did not exist
		data, err := s.d.Files.ReadFile(box.Abs(f))
		if err != nil {
			err = s.d.Files.Remove(r.lay.Abs(f))
		} else {
			err = s.d.Files.WriteFile(r.lay.Abs(f), data)
		}
		if err != nil {
			return a, fmt.Errorf("bringing %s back from scenario %d: %w", f, ref.Index, err)
		}
	}
	return a, s.bringRecords(r, c, ref)
}

// workFiles are a scenario's files but SpecForge's own records, which
// come back through bringRecords: the decisions, lessons and questions are
// shared by every scenario and are appended to, never replaced.
func (r *run) workFiles(ref tdd.ScenarioRef) []string {
	records := r.lay.SpecDir(r.o.SpecPath) + string(filepath.Separator)
	return slices.DeleteFunc(slices.Clone(ref.Files), func(f string) bool {
		p := r.lay.Abs(f)
		return p == r.lay.Lessons() || strings.HasPrefix(p, records)
	})
}

// sandboxLayout is the project layout inside a child's sandbox.
func sandboxLayout(r *run, c child) layout.Layout {
	box := r.lay
	box.Root = c.box.Dir
	return box
}

// bringRecords brings a child's logs and copies its review and
// verification records.
func (s *Service) bringRecords(r *run, c child, ref tdd.ScenarioRef) error {
	if err := s.bringLogs(r, c, true); err != nil {
		return err
	}
	box := sandboxLayout(r, c)
	childSpec := box.Abs(r.lay.Rel(r.o.SpecPath))
	for _, pair := range [][2]string{
		{r.lay.ReviewRecord(r.o.SpecPath, ref.Marker), box.ReviewRecord(childSpec, ref.Marker)},
		{r.lay.VerifyRecord(r.o.SpecPath, ref.Marker), box.VerifyRecord(childSpec, ref.Marker)},
	} {
		if data, err := s.d.Files.ReadFile(pair[1]); err == nil {
			if err := s.d.Files.WriteFile(pair[0], data); err != nil {
				return err
			}
		}
	}
	return nil
}

// logLengths measures the project's logs, the start every sandbox shares.
func (s *Service) logLengths(r *run) map[string]int {
	out := map[string]int{}
	for _, p := range r.logs() {
		data, _ := s.d.Files.ReadFile(p)
		out[p] = len(data)
	}
	return out
}

// logs are the project's decisions, lessons and questions files.
func (r *run) logs() []string {
	return []string{r.lay.Decisions(r.o.SpecPath), r.lay.Lessons(), r.lay.Questions(r.o.SpecPath)}
}

// bringLogs appends what a child added to the decisions and lessons and,
// for a finished scenario, to the questions. An unfinished scenario runs
// again, so the process decisions of its attempt (risk, review,
// verification) stay behind.
func (s *Service) bringLogs(r *run, c child, finished bool) error {
	box := sandboxLayout(r, c)
	childSpec := box.Abs(r.lay.Rel(r.o.SpecPath))
	decisions, lessons, questions := r.lay.Decisions(r.o.SpecPath), r.lay.Lessons(), r.lay.Questions(r.o.SpecPath)
	pairs := [][2]string{{decisions, box.Decisions(childSpec)}, {lessons, box.Lessons()}}
	if finished {
		pairs = append(pairs, [2]string{questions, box.Questions(childSpec)})
	}
	for _, pair := range pairs {
		theirs, err := s.d.Files.ReadFile(pair[1])
		start := c.logs[pair[0]]
		if err != nil || len(theirs) <= start {
			continue
		}
		added := string(theirs[start:])
		if !finished && pair[0] == decisions {
			added = clarify.Without(added, ProcessOrigins...)
		}
		if added == "" {
			continue
		}
		if err := s.d.Files.AppendFile(pair[0], []byte(added)); err != nil {
			return err
		}
	}
	return nil
}

// takeBack restores the files of arrivals that will run again.
func (s *Service) takeBack(r *run, arrived []arrival) error {
	for _, a := range arrived {
		for f, old := range a.before {
			var err error
			if old == nil {
				err = s.d.Files.Remove(r.lay.Abs(f))
			} else {
				err = s.d.Files.WriteFile(r.lay.Abs(f), old)
			}
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// closeArrival makes an arrived scenario the current one, at the end of
// its REVIEW, and closes it: the developer's review and the commit.
func (s *Service) closeArrival(ctx context.Context, r *run, a arrival) error {
	r.st.Current, r.st.Phase = a.index, tdd.PhaseReview
	r.st.FilesWritten = slices.Clone(a.ref.Files)
	hashes, err := s.d.Workspace.HashFiles(r.o.Root, r.o.Profile.IsTestFile)
	if err != nil {
		return err
	}
	r.st.TestHashes = hashes
	r.st.Adopt(a.child, a.index)
	ref := r.st.Scenarios[a.index]
	r.st.Record("parallel", "integrated", joinPaths(ref.Files), s.d.Now())
	s.d.Events.Integrated(ref)
	return s.close(ctx, r, ref, lastGates(r.st, ref))
}

// lockedPrompter lets one scenario of a batch ask at a time.
type lockedPrompter struct {
	mu *sync.Mutex
	p  ports.Prompter
}

func (l *lockedPrompter) Ask(ctx context.Context, q ports.Question) (string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.p.Ask(ctx, q)
}

// quietEvents passes a batch scenario's events on, one at a time, without
// the loop-level ones (start, baseline, finish) that belong to the parent.
type quietEvents struct {
	mu *sync.Mutex
	e  Events
}

func (q *quietEvents) do(f func()) {
	q.mu.Lock()
	defer q.mu.Unlock()
	f()
}

func (q *quietEvents) Started(*tdd.State, *spec.Document) {}
func (q *quietEvents) Amended([]string)                   {}
func (q *quietEvents) Orphaned([]string)                  {}
func (q *quietEvents) CheckpointFailed(string)            {}
func (q *quietEvents) Baseline(*tdd.Baseline, bool)       {}
func (q *quietEvents) Finished(*tdd.State)                {}
func (q *quietEvents) Phase(st *tdd.State, sc tdd.ScenarioRef) {
	q.do(func() { q.e.Phase(st, sc) })
}
func (q *quietEvents) AgentWorking(p tdd.Phase) { q.do(func() { q.e.AgentWorking(p) }) }
func (q *quietEvents) RunningTests(c string)    { q.do(func() { q.e.RunningTests(c) }) }
func (q *quietEvents) Rejected(why Rejection, d string) {
	q.do(func() { q.e.Rejected(why, d) })
}
func (q *quietEvents) Answered(qu, a string)    { q.do(func() { q.e.Answered(qu, a) }) }
func (q *quietEvents) Gates(rep quality.Report) { q.do(func() { q.e.Gates(rep) }) }
func (q *quietEvents) Risk(sc tdd.ScenarioRef, a risk.Assessment) {
	q.do(func() { q.e.Risk(sc, a) })
}
func (q *quietEvents) GateNotRun(g string, t risk.Tier) { q.do(func() { q.e.GateNotRun(g, t) }) }
func (q *quietEvents) ReviewSkipped(sc tdd.ScenarioRef, a risk.Assessment) {
	q.do(func() { q.e.ReviewSkipped(sc, a) })
}
func (q *quietEvents) Reviewed(sc tdd.ScenarioRef, rec tdd.ReviewRecord) {
	q.do(func() { q.e.Reviewed(sc, rec) })
}
func (q *quietEvents) Verified(sc tdd.ScenarioRef, rec tdd.VerifyRecord) {
	q.do(func() { q.e.Verified(sc, rec) })
}
func (q *quietEvents) Accepted(p tdd.Phase, sc tdd.ScenarioRef) { q.do(func() { q.e.Accepted(p, sc) }) }
func (q *quietEvents) Satisfied(sc tdd.ScenarioRef)             { q.do(func() { q.e.Satisfied(sc) }) }
func (q *quietEvents) Committed(sc tdd.ScenarioRef, sha string) {
	q.do(func() { q.e.Committed(sc, sha) })
}
func (q *quietEvents) Parallel([]string)                       {}
func (q *quietEvents) ParallelSkipped(tdd.ScenarioRef, string) {}
func (q *quietEvents) SeamFailed(string)                       {}
func (q *quietEvents) Integrated(tdd.ScenarioRef)              {}
