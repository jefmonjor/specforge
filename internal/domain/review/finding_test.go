package review

import (
	"strings"
	"testing"
)

func finding(id string, sev Severity, ev Evidence, c Causal, proof ...ProofRef) Finding {
	return Finding{ID: id, Lens: "reliability", Severity: sev, Claim: id, Evidence: ev, Causal: c, Proof: proof}
}

var (
	inHunk  = ProofRef{Kind: ProofChangedHunk, Path: "internal/pay/net.go", Line: 12}
	outside = ProofRef{Kind: ProofChangedHunk, Path: "internal/pay/net.go", Line: 30}
	newFile = ProofRef{Kind: ProofNewFile, Path: "internal/pay/bonus.go"}
)

func ids(fs []Finding) string {
	var out []string
	for _, f := range fs {
		out = append(out, f.ID)
	}
	return strings.Join(out, ",")
}

func TestVerifySortsFindings(t *testing.T) {
	d, _ := ParseDiff(sampleDiff)
	v := Verify([]Finding{
		finding("B1", Blocker, Deterministic, Introduced, inHunk),
		finding("B2", Critical, Inferential, BehaviorActivated, newFile),
		finding("B3", Critical, Deterministic, Worsened, outside, inHunk), // one valid proof is enough
		finding("F1", Blocker, Deterministic, PreExisting, inHunk),
		finding("F2", Critical, Deterministic, BaseOnly, inHunk),
		finding("E1", Blocker, Deterministic, Unknown, inHunk),
		finding("E2", Critical, Insufficient, Introduced, inHunk),
		finding("I1", Warning, Deterministic, Introduced, inHunk),
		finding("I2", Suggestion, Inferential, Unknown, inHunk),
		finding("X1", Blocker, Deterministic, Introduced, outside),
		finding("X2", Blocker, Deterministic, Introduced),
		finding("X3", Blocker, Deterministic, Introduced, ProofRef{Kind: ProofNewFile, Path: "internal/pay/net.go"}),
		finding("X4", Blocker, Deterministic, Introduced, ProofRef{Kind: "vibes", Path: "a"}),
		finding("B1", Blocker, Deterministic, Introduced, inHunk),
	}, d)
	for name, c := range map[string][2]string{
		"blocking":  {ids(v.Blocking), "B1,B2,B3"},
		"followups": {ids(v.FollowUps), "F1,F2"},
		"escalated": {ids(v.Escalated), "E1,E2"},
		"info":      {ids(v.Info), "I1,I2"},
	} {
		if c[0] != c[1] {
			t.Errorf("%s = %s, want %s", name, c[0], c[1])
		}
	}
	var discarded []string
	for _, x := range v.Discarded {
		discarded = append(discarded, x.ID+": "+x.Reason)
	}
	got := strings.Join(discarded, "\n")
	for _, want := range []string{
		"X1: no proof inside the change: internal/pay/net.go:30 is not a changed line",
		"X2: no proof_refs",
		"X3: no proof inside the change: internal/pay/net.go is not a file the change created",
		"X4: no proof inside the change: unknown proof kind vibes",
		"B1: duplicate id B1",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing discard %q in:\n%s", want, got)
		}
	}
	if ids(v.Inferential()) != "B2" {
		t.Errorf("inferential = %s", ids(v.Inferential()))
	}
}

func TestRefute(t *testing.T) {
	d, _ := ParseDiff(sampleDiff)
	v := Verify([]Finding{
		finding("D", Blocker, Deterministic, Introduced, inHunk),
		finding("C", Critical, Inferential, Introduced, inHunk),
		finding("R", Critical, Inferential, Introduced, inHunk),
		finding("U", Critical, Inferential, Introduced, inHunk),
	}, d)
	v = v.Refute(map[string]bool{"C": true, "R": false}, map[string]string{"R": "bonus is never negative: Money rejects it"})
	if ids(v.Blocking) != "D,C" || ids(v.Escalated) != "U" {
		t.Fatalf("blocking=%s escalated=%s", ids(v.Blocking), ids(v.Escalated))
	}
	if len(v.Discarded) != 1 || v.Discarded[0].Reason != "refuted: bonus is never negative: Money rejects it" {
		t.Fatalf("discarded = %+v", v.Discarded)
	}
}

func TestCorrectionBudget(t *testing.T) {
	for lines, want := range map[int]int{0: 1, 1: 1, 3: 2, 120: 60, 399: 200, 2000: 200} {
		if got := CorrectionBudget(lines); got != want {
			t.Errorf("CorrectionBudget(%d) = %d, want %d", lines, got, want)
		}
	}
}
