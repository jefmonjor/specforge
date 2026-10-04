package gates

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"specforge/internal/domain/quality"
	"specforge/internal/domain/stack"
	"specforge/internal/ports"
)

// Duplication runs jscpd and compares the duplicated-lines percentage with
// the threshold.
type Duplication struct {
	proc ports.CommandRunner
	max  float64
}

// Name implements ports.Gate.
func (*Duplication) Name() string { return "duplication" }

// Applies implements ports.Gate.
func (*Duplication) Applies(stack.Profile) bool { return true }

// jscpdIgnore keeps tests, dependencies and build output out of the count.
const jscpdIgnore = "**/*_test.go,**/*.test.*,**/*.spec.*,**/test_*.py,**/src/test/**,**/node_modules/**,**/target/**,**/build/**,**/dist/**,**/vendor/**,**/.specforge/**"

type jscpdReport struct {
	Statistics struct {
		Total struct {
			Percentage float64 `json:"percentage"`
			Clones     int     `json:"clones"`
		} `json:"total"`
	} `json:"statistics"`
	Duplicates []struct {
		FirstFile  struct{ Name string } `json:"firstFile"`
		SecondFile struct{ Name string } `json:"secondFile"`
		Lines      int                   `json:"lines"`
	} `json:"duplicates"`
}

// Check implements ports.Gate.
func (d *Duplication) Check(ctx context.Context, root string, _ stack.Profile) (quality.Result, error) {
	outDir, err := os.MkdirTemp("", "specforge-jscpd-*")
	if err != nil {
		return quality.Result{}, err
	}
	defer os.RemoveAll(outDir)

	res, err := globalOrLocal(ctx, d.proc, root, "jscpd", ".", "--silent", "--reporters", "json", "--output", outDir, "--ignore", jscpdIgnore)
	if err != nil {
		return toolError(ctx, d.Name(), err)
	}
	data, err := os.ReadFile(filepath.Join(outDir, "jscpd-report.json"))
	if err != nil {
		return skipped(d.Name(), "jscpd produced no report: "+firstLine(res.Combined())), nil
	}
	return d.judge(data)
}

func (d *Duplication) judge(data []byte) (quality.Result, error) {
	var rep jscpdReport
	if err := json.Unmarshal(data, &rep); err != nil {
		return skipped(d.Name(), "unreadable jscpd report: "+err.Error()), nil
	}
	pct := rep.Statistics.Total.Percentage
	summary := fmt.Sprintf("%s duplicated (max %s), %d clone(s)", fmtPct(pct), fmtPct(d.max), rep.Statistics.Total.Clones)
	if pct <= d.max {
		return quality.Result{Gate: d.Name(), Status: quality.Passed, Summary: summary}, nil
	}
	var details strings.Builder
	for _, dup := range rep.Duplicates {
		fmt.Fprintf(&details, "%s ↔ %s (%d lines)\n", dup.FirstFile.Name, dup.SecondFile.Name, dup.Lines)
	}
	return quality.Result{Gate: d.Name(), Status: quality.Failed, Summary: summary, Details: details.String()}, nil
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
