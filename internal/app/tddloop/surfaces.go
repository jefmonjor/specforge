package tddloop

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"specforge/internal/domain/change"
	"specforge/internal/domain/tdd"
	"specforge/internal/ports"
)

// Surface modes (plan.surfaces in specforge.yaml).
const (
	// SurfacesAsk asks the developer about a file outside the plan.
	SurfacesAsk = "ask"
	// SurfacesStrict rejects the attempt.
	SurfacesStrict = "strict"
	// SurfacesOff does not check.
	SurfacesOff = "off"
)

// SurfaceError reports files outside the plan that the developer refused
// and the agent did not put back. Committing them would record a change
// nobody approved.
type SurfaceError struct{ Files []string }

func (e *SurfaceError) Error() string {
	return "files outside the plan were refused and not put back: " + strings.Join(e.Files, ", ")
}

// checkSurfaces looks at what an agent turn changed against the approved
// plan. A file outside it goes to the developer (ask) or rejects the
// attempt (strict). It returns the feedback of a rejected attempt; a
// question nobody can answer now is returned as an error.
func (s *Service) checkSurfaces(ctx context.Context, r *run, sc tdd.ScenarioRef, t turn) (string, error) {
	if r.o.Surfaces == SurfacesOff || r.surfaces.Empty() {
		return "", nil
	}
	refused := sortedKeys(r.st.Refused)
	stillRefused := r.st.Unreverted(t.After)
	putBack := slices.DeleteFunc(refused, func(p string) bool { return slices.Contains(stillRefused, p) })
	outside := slices.DeleteFunc(s.outsidePlan(r, t.Changed), func(p string) bool {
		return slices.Contains(putBack, p) || slices.Contains(stillRefused, p)
	})
	if len(stillRefused) > 0 {
		// Already refused once: no second question, the attempt fails.
		s.refuse(r, t, outside)
		return s.reject(r, RejectOutsidePlan, joinPaths(append(stillRefused, outside...)), ""), nil
	}
	if len(outside) == 0 {
		return "", nil
	}
	list := joinPaths(outside)
	if r.o.Surfaces == SurfacesStrict {
		s.refuse(r, t, outside)
		return s.reject(r, RejectOutsidePlan, list, ""), nil
	}
	q := question(r.o.Language, "surfaces")
	answer, err := s.d.Asker.Ask(ctx, s.originAs(r, sc, OriginVerify), ports.Question{Text: fmt.Sprintf(q.text, sc.Index, list), Options: q.options, Strict: true})
	if err != nil {
		return "", err
	}
	if pick(answer, q.options) == 0 {
		r.st.Accept(outside...)
		r.st.Record("surfaces", "accepted", list, s.d.Now())
		return "", s.save(r)
	}
	s.refuse(r, t, outside)
	return s.reject(r, RejectOutsidePlan, list, ""), nil
}

func (s *Service) refuse(r *run, t turn, paths []string) {
	for _, p := range paths {
		r.st.Refuse(p, t.Before[p])
	}
}

// outsidePlan returns the changed files the plan does not cover. SpecForge's
// own records and generated files (lock files) are never outside it.
func (s *Service) outsidePlan(r *run, changed []string) []string {
	specDir := r.lay.Rel(r.lay.SpecDir(r.o.SpecPath)) + "/"
	lessons := r.lay.Rel(r.lay.Lessons())
	var out []string
	for _, p := range changed {
		switch {
		case r.surfaces.Allows(p), slices.Contains(r.st.Surfaces, p),
			strings.HasPrefix(p, specDir), p == lessons, strings.HasPrefix(p, ".specforge/"),
			change.Generated(p):
			continue
		}
		out = append(out, p)
	}
	return out
}

// scenarioFiles returns the files the scenario changed that still differ
// from the last commit (a file put back is not part of it), and stops a
// scenario that still carries a refused file.
func (s *Service) scenarioFiles(ctx context.Context, r *run) ([]string, error) {
	now, err := s.d.Workspace.Snapshot(ctx, r.o.Root)
	if err != nil {
		return nil, err
	}
	if left := r.st.Unreverted(now); len(left) > 0 {
		return nil, &SurfaceError{Files: left}
	}
	var files []string
	for _, f := range r.st.FilesWritten {
		if _, changed := now[f]; changed {
			files = append(files, f)
		}
	}
	return sortedStrings(files), nil
}

// allowedSurfaces lists the plan's files plus those the developer accepted,
// for the prompt.
func (r *run) allowedSurfaces() []string {
	if r.surfaces.Empty() || r.o.Surfaces == SurfacesOff {
		return nil
	}
	return slices.Compact(sortedStrings(append(r.surfaces.List(), r.st.Surfaces...)))
}

// notRefused drops the files the developer refused: they are not part of
// the scenario and never go into its commit.
func notRefused(st *tdd.State, paths []string) []string {
	return slices.DeleteFunc(slices.Clone(paths), func(p string) bool {
		_, refused := st.Refused[p]
		return refused
	})
}

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return sortedStrings(out)
}
