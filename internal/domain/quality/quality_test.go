package quality

import (
	"strings"
	"testing"
)

func TestReportBlockingRespectsStrict(t *testing.T) {
	r := Report{
		{Gate: "lint", Status: Passed},
		{Gate: "dup", Status: Skipped, Summary: "jscpd not installed"},
	}
	if !r.OK(false) {
		t.Fatal("a skipped gate does not block by default")
	}
	if r.OK(true) || len(r.Blocking(true)) != 1 {
		t.Fatal("a skipped gate blocks in strict mode")
	}
	r = append(r, Result{Gate: "mutation", Status: Failed, Summary: "score 60 < 80", Details: "survivors"})
	if r.OK(false) {
		t.Fatal("a failed gate always blocks")
	}
	if got := r.Explain(false); !strings.Contains(got, "score 60 < 80") || strings.Contains(got, "jscpd") {
		t.Fatalf("Explain = %q", got)
	}
}

func TestMutationScore(t *testing.T) {
	if got := MutationScore(7, 1, 2, 0); got != 80 {
		t.Fatalf("score = %v", got)
	}
	if got := MutationScore(0, 0, 0, 0); got != 100 {
		t.Fatalf("no mutants = %v", got)
	}
}
