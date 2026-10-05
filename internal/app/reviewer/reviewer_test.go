package reviewer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"specforge/internal/adapters/fsys"
	"specforge/internal/adapters/process"
	"specforge/internal/adapters/workspace"
	"specforge/internal/domain/review"
	"specforge/internal/domain/risk"
	"specforge/internal/ports"
)

const diff = "diff --git a/net.go b/net.go\n--- a/net.go\n+++ b/net.go\n@@ -10,2 +10,3 @@\n tax := gross.Times(rate)\n-return gross.Minus(tax)\n+net := gross.Minus(tax)\n+return net.Plus(bonus)\n"

type agent struct {
	root    string
	replies []func(prompt string) string
	prompts []string
	models  []string
}

func (a *agent) Name() string { return "fake" }
func (a *agent) Run(_ context.Context, r ports.AgentRequest) (string, error) {
	a.prompts = append(a.prompts, r.Prompt)
	a.models = append(a.models, r.Model)
	if len(a.replies) == 0 {
		return "", errors.New("unexpected call")
	}
	f := a.replies[0]
	a.replies = a.replies[1:]
	return f(r.Prompt), nil
}
func (a *agent) Interactive(context.Context, ports.AgentRequest) error { return nil }

func says(s string) func(string) string {
	return func(string) string { return "```json\n" + s + "\n```" }
}

func setup(t *testing.T, replies ...func(string) string) (*Service, *agent, Request) {
	t.Helper()
	root := t.TempDir()
	a := &agent{root: root, replies: replies}
	svc := New(Deps{Agent: a, Workspace: workspace.New(process.NewRunner(nil))})
	req := Request{Root: root, Language: "en", Stack: "go", SpecTitle: "Net pay", Diff: diff,
		Lenses: []review.Lens{review.LensReliability},
		Model:  func(phase string) string { return "model-" + phase }}
	return svc, a, req
}

const (
	realFinding  = `{"id":"REL-001","severity":"CRITICAL","location":{"path":"net.go","line":11},"claim":"a negative bonus makes the net negative","evidence_class":"deterministic","causal_disposition":"introduced","proof_refs":[{"kind":"changed-hunk","path":"net.go","line":11}]}`
	inventedOne  = `{"id":"REL-002","severity":"BLOCKER","location":{"path":"net.go","line":40},"claim":"rounding is wrong at line forty","evidence_class":"deterministic","causal_disposition":"introduced","proof_refs":[{"kind":"changed-hunk","path":"net.go","line":40}]}`
	inferential  = `{"id":"REL-003","severity":"CRITICAL","location":{"path":"net.go","line":12},"claim":"bonus may be negative when callers pass refunds","evidence_class":"inferential","causal_disposition":"introduced","proof_refs":[{"kind":"changed-hunk","path":"net.go","line":12}]}`
	lensWrapper  = `{"lens":"reliability","findings":[%s],"evidence":["read net.go"]}`
	refutedReply = `{"verdicts":[{"id":"REL-003","verdict":"refuted","reason":"Money cannot hold a negative amount"}]}`
)

func lensSays(findings ...string) func(string) string {
	return says(strings.Replace(lensWrapper, "%s", strings.Join(findings, ","), 1))
}

func TestReviewKeepsOnlyProvenFindings(t *testing.T) {
	svc, a, req := setup(t, lensSays(realFinding, inventedOne))
	res, err := svc.Review(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if res.Reported != 2 || len(res.Verdict.Blocking) != 1 || res.Verdict.Blocking[0].ID != "REL-001" || res.Verdict.Blocking[0].Lens != "reliability" {
		t.Fatalf("verdict = %+v", res.Verdict)
	}
	if len(res.Verdict.Discarded) != 1 || !strings.Contains(res.Verdict.Discarded[0].Reason, "net.go:40 is not a changed line") {
		t.Fatalf("the invented proof is discarded with its reason: %+v", res.Verdict.Discarded)
	}
	if !strings.Contains(a.prompts[0], "## Your lens: reliability") || !strings.Contains(a.prompts[0], "+return net.Plus(bonus)") || a.models[0] != "model-review" {
		t.Fatalf("prompt or model:\n%s\n%v", a.prompts[0], a.models)
	}
}

func TestInferentialFindingsAreRefutedFirst(t *testing.T) {
	svc, a, req := setup(t, lensSays(realFinding, inferential), says(refutedReply))
	res, err := svc.Review(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Verdict.Blocking) != 1 || res.Verdict.Discarded[0].Reason != "refuted: Money cannot hold a negative amount" {
		t.Fatalf("verdict = %+v", res.Verdict)
	}
	if !strings.Contains(a.prompts[1], "# Task: REFUTE") || !strings.Contains(a.prompts[1], "REL-003") || strings.Contains(a.prompts[1], "REL-001 ·") || a.models[1] != "model-refute" {
		t.Fatalf("only the inferential finding goes to the refuter:\n%s", a.prompts[1])
	}
}

func TestAnInvalidAnswerIsRetriedOnceThenFailsClosed(t *testing.T) {
	svc, a, req := setup(t, func(string) string { return "No issues, ship it." }, says(`{"lens":"risk","findings":[],"evidence":["x"]}`))
	_, err := svc.Review(context.Background(), req)
	var step *StepError
	if !errors.As(err, &step) || step.Step != "lens reliability" {
		t.Fatalf("want StepError, got %v", err)
	}
	if !strings.Contains(a.prompts[1], "did not end with a JSON object valid for the required schema") {
		t.Fatal("the retry says why")
	}
}

func TestALensThatWritesIsRefused(t *testing.T) {
	svc, _, req := setup(t)
	svc.d.Agent = &agent{replies: []func(string) string{func(string) string {
		_ = os.WriteFile(filepath.Join(req.Root, "fix.go"), []byte("package x\n"), 0o644)
		return "```json\n" + strings.Replace(lensWrapper, "%s", "", 1) + "\n```"
	}}}
	_, err := svc.Review(context.Background(), req)
	var ro *ReadOnlyError
	if !errors.As(err, &ro) || ro.Files[0] != "fix.go" {
		t.Fatalf("want ReadOnlyError, got %v", err)
	}
}

func TestValidateChecksOnlyTheCorrectedFindings(t *testing.T) {
	fixed := []review.Finding{{ID: "REL-001", Severity: review.Critical, Claim: "negative net"}, {ID: "REL-004", Severity: review.Blocker, Claim: "overflow"}}
	svc, a, req := setup(t, says(`{"results":[{"id":"REL-001","status":"resolved","reason":"net is clamped at zero"},{"id":"REL-999","status":"resolved","reason":"not asked"}]}`))
	got, err := svc.Validate(context.Background(), req, fixed)
	if err != nil {
		t.Fatal(err)
	}
	if got["REL-001"].Status != "resolved" || got["REL-004"].Status != "regression" || len(got) != 2 {
		t.Fatalf("validation = %+v", got)
	}
	if !strings.Contains(a.prompts[0], "# Task: VALIDATE A CORRECTION") {
		t.Fatal("validate prompt")
	}
}

type diffs struct{ text string }

func (d diffs) DefaultBase(context.Context, string) (string, error)  { return "main", nil }
func (d diffs) Diff(context.Context, string, string) (string, error) { return d.text, nil }
func (d diffs) Files(context.Context, string) ([]string, error)      { return nil, nil }

func TestBranchReviewWritesItsReportAndBlocks(t *testing.T) {
	svc, _, req := setup(t, lensSays(realFinding))
	report := filepath.Join(req.Root, "docs", "review", "branch.json")
	res, err := svc.Branch(context.Background(), diffs{diff}, fsys.OS{}, BranchOptions{
		Root: req.Root, Language: "en", Stack: "go", LensesAuto: true, Rules: risk.DefaultRules(), Report: report,
	})
	var blocked *BlockedError
	if !errors.As(err, &blocked) || len(blocked.Blocking) != 1 || res.Base != "main" || res.Risk.Tier != risk.Medium {
		t.Fatalf("err=%v res=%+v", err, res)
	}
	data, _ := os.ReadFile(report)
	if !strings.Contains(string(data), `"REL-001"`) || !strings.Contains(string(data), `"base": "main"`) {
		t.Fatalf("report:\n%s", data)
	}
}

func TestBranchReviewOfNothing(t *testing.T) {
	svc, a, req := setup(t)
	res, err := svc.Branch(context.Background(), diffs{""}, fsys.OS{}, BranchOptions{Root: req.Root, LensesAuto: true, Rules: risk.DefaultRules()})
	if err != nil || !res.Empty || len(a.prompts) != 0 {
		t.Fatalf("an empty branch costs nothing: %+v %v", res, err)
	}
}

func TestBlindReviewRunsTwoPassesAndCorroborates(t *testing.T) {
	second := strings.Replace(realFinding, `"line":11}]`, `"line":12}]`, 1) // same hunk, other line
	svc, a, req := setup(t, lensSays(realFinding, inventedOne), lensSays(second))
	req.Blind = true
	res, err := svc.Review(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.prompts) != 2 || a.models[0] != "model-review" || a.models[1] != "model-review2" {
		t.Fatalf("two independent passes with their own models: %v", a.models)
	}
	if len(res.Verdict.Blocking) != 1 || res.Verdict.Blocking[0].Evidence != review.Deterministic || !res.Blind {
		t.Fatalf("corroborated by both passes, no refuter: %+v", res.Verdict)
	}
}
