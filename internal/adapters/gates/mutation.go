package gates

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"specforge/internal/domain/quality"
	"specforge/internal/domain/stack"
	"specforge/internal/ports"
)

// Mutation runs Stryker on Node projects that configure it and compares the
// mutation score with the threshold.
type Mutation struct {
	proc ports.CommandRunner
	min  float64
}

var strykerConfigs = []string{"stryker.conf.json", "stryker.conf.js", "stryker.conf.mjs", "stryker.conf.cjs", "stryker.config.json", "stryker.config.mjs"}

// Name implements ports.Gate.
func (*Mutation) Name() string { return "mutation" }

// Applies implements ports.Gate.
func (*Mutation) Applies(p stack.Profile) bool { return p.Kind == stack.Node }

// Check implements ports.Gate.
func (m *Mutation) Check(ctx context.Context, root string, _ stack.Profile) (quality.Result, error) {
	configured := false
	for _, c := range strykerConfigs {
		if hasFile(root, c) {
			configured = true
			break
		}
	}
	if !configured {
		return skipped(m.Name(), "Stryker is not configured (no stryker.conf.*)"), nil
	}

	report := filepath.Join(root, "reports", "mutation", "mutation.json")
	_ = os.Remove(report) // never judge a stale report
	res, err := npxLocal(ctx, m.proc, root, "stryker", "run", "--reporters", "json,clear-text")
	if err != nil {
		return toolError(ctx, m.Name(), err)
	}
	data, err := os.ReadFile(report)
	if err != nil {
		return skipped(m.Name(), "Stryker produced no report: "+firstLine(res.Combined())), nil
	}
	return m.judge(data)
}

// mutationReport follows the mutation-testing-report-schema.
type mutationReport struct {
	Files map[string]struct {
		Mutants []struct {
			MutatorName string `json:"mutatorName"`
			Status      string `json:"status"`
			Location    struct {
				Start struct{ Line int } `json:"start"`
			} `json:"location"`
		} `json:"mutants"`
	} `json:"files"`
}

func (m *Mutation) judge(data []byte) (quality.Result, error) {
	var rep mutationReport
	if err := json.Unmarshal(data, &rep); err != nil {
		return skipped(m.Name(), "unreadable Stryker report: "+err.Error()), nil
	}
	var killed, timeout, survived, noCoverage int
	var survivors []string
	for file, f := range rep.Files {
		for _, mu := range f.Mutants {
			switch mu.Status {
			case "Killed":
				killed++
			case "Timeout":
				timeout++
			case "Survived":
				survived++
				survivors = append(survivors, fmt.Sprintf("%s:%d %s", file, mu.Location.Start.Line, mu.MutatorName))
			case "NoCoverage":
				noCoverage++
				survivors = append(survivors, fmt.Sprintf("%s:%d %s (no coverage)", file, mu.Location.Start.Line, mu.MutatorName))
			}
		}
	}
	score := quality.MutationScore(killed, timeout, survived, noCoverage)
	summary := fmt.Sprintf("mutation score %s (min %s), %d survived", fmtPct(score), fmtPct(m.min), survived+noCoverage)
	if score >= m.min {
		return quality.Result{Gate: m.Name(), Status: quality.Passed, Summary: summary}, nil
	}
	sort.Strings(survivors)
	return quality.Result{Gate: m.Name(), Status: quality.Failed, Summary: summary, Details: strings.Join(survivors, "\n")}, nil
}
