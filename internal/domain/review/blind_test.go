package review

import "testing"

func TestBlindCorroboratesWhatBothPassesProve(t *testing.T) {
	d, _ := ParseDiff(sampleDiff)
	a := []Finding{
		finding("REL-001", Critical, Inferential, Introduced, ProofRef{Kind: ProofChangedHunk, Path: "internal/pay/net.go", Line: 11}),
		finding("REL-002", Blocker, Deterministic, Introduced, ProofRef{Kind: ProofNewFile, Path: "internal/pay/bonus.go"}),
	}
	b := []Finding{
		finding("REL-001", Critical, Inferential, Introduced, ProofRef{Kind: ProofChangedHunk, Path: "internal/pay/net.go", Line: 12}), // same hunk
		finding("REL-002", Warning, Deterministic, Introduced, ProofRef{Kind: ProofChangedHunk, Path: "internal/pay/net.go", Line: 42}),
	}
	got := Blind(a, b, d)
	if len(got) != 3 {
		t.Fatalf("Blind = %+v", got)
	}
	if got[0].ID != "REL-001" || got[0].Evidence != Deterministic {
		t.Errorf("both passes proved it on the same hunk: %+v", got[0])
	}
	if got[1].ID != "REL-002" || got[1].Evidence != Inferential {
		t.Errorf("one pass only: a severe finding goes to the refuter: %+v", got[1])
	}
	if got[2].ID != "REL-002b" || got[2].Evidence != Deterministic {
		t.Errorf("the second pass's own finding keeps its evidence when not severe, with a distinct ID: %+v", got[2])
	}
	v := Verify(got, d)
	if ids(v.Inferential()) != "REL-002" {
		t.Errorf("only the uncorroborated severe finding needs the refuter: %s", ids(v.Inferential()))
	}
}
