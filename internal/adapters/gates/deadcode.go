package gates

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"specforge/internal/domain/quality"
	"specforge/internal/domain/stack"
	"specforge/internal/ports"
)

// DeadCode runs Knip on Node projects.
type DeadCode struct{ proc ports.CommandRunner }

// Name implements ports.Gate.
func (*DeadCode) Name() string { return "dead-code" }

// Applies implements ports.Gate.
func (*DeadCode) Applies(p stack.Profile) bool { return p.Kind == stack.Node }

// Check implements ports.Gate.
func (k *DeadCode) Check(ctx context.Context, root string, _ stack.Profile) (quality.Result, error) {
	res, err := npxLocal(ctx, k.proc, root, "knip", "--reporter", "json", "--no-exit-code", "--no-progress")
	if err != nil {
		return toolError(ctx, k.Name(), err)
	}
	return k.judge(res.Stdout)
}

// judge counts every finding in Knip's JSON report: unused files plus each
// issue array (dependencies, exports, types, unlisted…) of every file.
func (k *DeadCode) judge(stdout string) (quality.Result, error) {
	var rep struct {
		Files  []string                     `json:"files"`
		Issues []map[string]json.RawMessage `json:"issues"`
	}
	start := strings.Index(stdout, "{")
	if start < 0 || json.Unmarshal([]byte(stdout[start:]), &rep) != nil {
		return skipped(k.Name(), "unreadable knip report: "+firstLine(stdout)), nil
	}

	counts := map[string]int{}
	if len(rep.Files) > 0 {
		counts["unused files"] = len(rep.Files)
	}
	for _, issue := range rep.Issues {
		for kind, raw := range issue {
			if kind == "file" || kind == "owners" {
				continue
			}
			var list []json.RawMessage
			var obj map[string]json.RawMessage
			switch {
			case json.Unmarshal(raw, &list) == nil:
				counts[kind] += len(list)
			case json.Unmarshal(raw, &obj) == nil:
				counts[kind] += len(obj)
			}
		}
	}

	total := 0
	var parts []string
	for kind, n := range counts {
		if n > 0 {
			total += n
			parts = append(parts, fmt.Sprintf("%s: %d", kind, n))
		}
	}
	sort.Strings(parts)
	if total == 0 {
		return quality.Result{Gate: k.Name(), Status: quality.Passed, Summary: "knip: no unused code or dependencies"}, nil
	}
	details := strings.Join(rep.Files, "\n")
	return quality.Result{Gate: k.Name(), Status: quality.Failed,
		Summary: fmt.Sprintf("knip found %d issue(s) — %s", total, strings.Join(parts, ", ")), Details: details}, nil
}
