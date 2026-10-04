package gates

import (
	"context"
	"fmt"
	"os"
	"strings"

	"specforge/internal/domain/legacy"
	"specforge/internal/domain/quality"
	"specforge/internal/domain/stack"
)

// Migration checks a rewrite against its target: the Java release the
// build declares and the imports the new code may not use (javax.* that
// Jakarta EE renamed, Log4j 1, JUnit 3…). It needs no tool, so it never
// skips.
type Migration struct{ Target legacy.Target }

// Name implements ports.Gate.
func (*Migration) Name() string { return "migration" }

// Applies implements ports.Gate.
func (m *Migration) Applies(p stack.Profile) bool {
	return (p.Kind == stack.Maven || p.Kind == stack.Gradle) && (m.Target.JavaRelease > 0 || len(m.Target.ForbiddenImports) > 0)
}

// Check implements ports.Gate.
func (m *Migration) Check(ctx context.Context, root string, _ stack.Profile) (quality.Result, error) {
	if err := ctx.Err(); err != nil {
		return quality.Result{}, err
	}
	v, err := legacy.Conformance(os.DirFS(root), m.Target)
	if err != nil {
		return skipped(m.Name(), "could not read the sources: "+err.Error()), nil
	}
	if len(v) == 0 {
		summary := "no forbidden import"
		if m.Target.JavaRelease > 0 {
			summary = fmt.Sprintf("Java %d declared, %s", m.Target.JavaRelease, summary)
		}
		return quality.Result{Gate: m.Name(), Status: quality.Passed, Summary: summary}, nil
	}
	lines := make([]string, len(v))
	for i, x := range v {
		lines[i] = x.String()
	}
	return quality.Result{Gate: m.Name(), Status: quality.Failed,
		Summary: fmt.Sprintf("%d place(s) do not meet the migration target", len(v)), Details: strings.Join(lines, "\n")}, nil
}
