package tddloop

import (
	"context"
	"fmt"
	"strings"

	"github.com/jefmonjor/specforge/v6/internal/domain/tdd"
)

// startBaseline adopts the seed of a parallel loop's scenario, or takes
// the baseline.
func (s *Service) startBaseline(ctx context.Context, r *run) error {
	if r.o.Seed != nil {
		b := *r.o.Seed
		r.st.Baseline = &b
		return s.save(r)
	}
	return s.takeBaseline(ctx, r)
}

// takeBaseline runs the whole suite once before the first RED of a new
// loop. What fails there already failed on the branch: it is recorded as
// known, shown to the developer, and never blamed on the agent.
func (s *Service) takeBaseline(ctx context.Context, r *run) error {
	out, err := s.runTests(ctx, r, "")
	if err != nil {
		return err
	}
	b := tdd.NewBaseline(out, r.o.Profile.TestCommand(""), s.d.Now())
	r.st.Baseline = b
	switch {
	case !out.Compiled:
		r.st.Record("baseline", "does not build", out.Output, s.d.Now())
	case b == nil:
		r.st.Record("baseline", "unnamed", "the runner cannot name failures: every failure blocks", s.d.Now())
	default:
		r.st.Record("baseline", "recorded", fmt.Sprintf("%d known failure(s)", len(b.Failures)), s.d.Now())
	}
	s.d.Events.Baseline(b, out.Compiled)
	return s.save(r)
}

// runSuite runs the whole suite and judges it against the baseline. It
// returns what blocks REFACTOR ("" when nothing does): only failures that
// are not known. Known failures that pass now leave the baseline.
func (s *Service) runSuite(ctx context.Context, r *run) (string, error) {
	out, err := s.runTests(ctx, r, "")
	if err != nil {
		return "", err
	}
	v := r.st.Baseline.Judge(out)
	if len(v.Fixed) > 0 {
		r.st.Baseline.Retire(v.Fixed)
		r.st.Record("baseline", "fixed", tdd.Names(v.Fixed), s.d.Now())
	}
	if len(v.Known) > 0 {
		r.st.Record("baseline", "still failing", tdd.Names(v.Known), s.d.Now())
	}
	if v.OK {
		return "", nil
	}
	if r.st.Baseline == nil || len(v.Fresh) == 0 {
		return out.Output, nil
	}
	return strings.TrimSpace(feedback(r.o.Language, RejectNewFailures, tdd.Names(v.Fresh)) + "\n\n" + out.Output), nil
}

// knownFailures names the baseline failures for the prompt.
func knownFailures(st *tdd.State) []string {
	if st.Baseline == nil {
		return nil
	}
	names := make([]string, len(st.Baseline.Failures))
	for i, f := range st.Baseline.Failures {
		names[i] = f.String()
	}
	return names
}
