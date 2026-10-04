package audit

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"specforge/internal/adapters/fsys"
	"specforge/internal/app/clarify"
	"specforge/internal/domain/security"
	"specforge/internal/ports"
)

const (
	recon      = "```json\n{\"units\":[{\"id\":\"U1\",\"file\":\"a.go\"}]}\n```"
	candidates = "```json\n{\"candidates\":[{\"id\":\"SEC-1\"}]}\n```"
)

func verdict(findings string) string {
	return "```json\n{\"generated_at\":\"x\",\"total_confirmed\":0,\"total_needs_validation\":0,\"total_rejected\":0,\"findings\":[" + findings + "]}\n```"
}

const confirmedHigh = `{"id":"SEC-1","title":"SQL injection","severity":"high","status":"confirmed","file":"a.go","line":3,"attack_class":"sqli","description":"d","proof_of_impact":"p"}`
const needsValidationCritical = `{"id":"SEC-2","title":"SSRF","severity":"critical","status":"needs_validation","file":"b.go","line":9,"attack_class":"ssrf","description":"d","proof_of_impact":"p"}`

type scriptedAgent struct {
	answers []string
	prompts []string
}

func (a *scriptedAgent) Name() string { return "fake" }
func (a *scriptedAgent) Run(_ context.Context, r ports.AgentRequest) (string, error) {
	a.prompts = append(a.prompts, r.Prompt)
	if len(a.answers) == 0 {
		return "", errors.New("unexpected call")
	}
	out := a.answers[0]
	a.answers = a.answers[1:]
	return out, nil
}
func (a *scriptedAgent) Interactive(context.Context, ports.AgentRequest) error { return nil }

type fakeVCS struct {
	diff  string
	files []string
	err   error
}

func (v fakeVCS) DefaultBase(context.Context, string) (string, error)  { return "main", nil }
func (v fakeVCS) Diff(context.Context, string, string) (string, error) { return v.diff, v.err }
func (v fakeVCS) Files(context.Context, string) ([]string, error)      { return v.files, v.err }

type prompter struct {
	answer string
	nonTTY bool
	asked  int
}

func (p *prompter) Ask(context.Context, ports.Question) (string, error) {
	p.asked++
	if p.nonTTY {
		return "", ports.ErrNonInteractive
	}
	return p.answer, nil
}

func service(agent ports.Agent, v ports.VCS, p ports.Prompter) *Service {
	now := func() time.Time { return time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC) }
	return New(Deps{Agent: agent, VCS: v, Files: fsys.OS{}, Now: now,
		Asker: &clarify.Asker{Prompter: p, Files: fsys.OS{}, Lang: "en", Now: now}})
}

func opts(root string) Options {
	return Options{Root: root, Scope: ScopeDiff, Threshold: security.Info, Language: "en"}
}

func TestUnparseableVerdictFailsClosed(t *testing.T) {
	// Regression: an unparseable verifier answer became an empty report and
	// the audit printed "passed".
	root := t.TempDir()
	agent := &scriptedAgent{answers: []string{recon, candidates, "Everything looks fine!", "Still fine, trust me."}}
	_, err := service(agent, fakeVCS{diff: "diff --git a/a.go b/a.go\n+x"}, &prompter{}).Run(context.Background(), opts(root))
	var se *StepError
	if !errors.As(err, &se) || se.Step != "validation" {
		t.Fatalf("want StepError at validation, got %v", err)
	}
	if !strings.Contains(agent.prompts[3], "did not contain the JSON") {
		t.Fatal("exactly one retry with a reminder expected")
	}
	if _, err := os.Stat(filepath.Join(root, "docs", "security", "findings.json")); err == nil {
		t.Fatal("no report may be written for a failed audit")
	}
}

func TestSchemaViolationsAreRejected(t *testing.T) {
	bad := `{"id":"SEC-1","title":"x","severity":"huge","status":"confirmed","file":"a","line":1,"attack_class":"a","description":"d","proof_of_impact":"p"}`
	agent := &scriptedAgent{answers: []string{recon, candidates, verdict(bad), verdict(bad)}}
	_, err := service(agent, fakeVCS{diff: "+x"}, &prompter{}).Run(context.Background(), opts(t.TempDir()))
	var se *StepError
	if !errors.As(err, &se) {
		t.Fatalf("a schema violation must fail the step, got %v", err)
	}
}

func TestConfirmedFindingBlocksAndReportsAreWritten(t *testing.T) {
	root := t.TempDir()
	agent := &scriptedAgent{answers: []string{recon, candidates, verdict(confirmedHigh)}}
	res, err := service(agent, fakeVCS{diff: "+x"}, &prompter{}).Run(context.Background(), opts(root))
	var blocked *BlockedError
	if !errors.As(err, &blocked) || len(blocked.Findings) != 1 || res.Report.TotalConfirmed != 1 {
		t.Fatalf("want BlockedError with one finding, got %v %+v", err, res)
	}
	for _, f := range []string{"findings.json", "coverage-ledger.json", "REPORT.md"} {
		if _, err := os.Stat(filepath.Join(root, "docs", "security", f)); err != nil {
			t.Errorf("%s not written", f)
		}
	}
}

func TestSevereNeedsValidationIsAskedAndBlocksWithoutTerminal(t *testing.T) {
	t.Run("developer rejects it", func(t *testing.T) {
		agent := &scriptedAgent{answers: []string{recon, candidates, verdict(needsValidationCritical)}}
		p := &prompter{answer: "No: reject it"}
		res, err := service(agent, fakeVCS{diff: "+x"}, p).Run(context.Background(), opts(t.TempDir()))
		if err != nil || p.asked != 1 || res.Report.Findings[0].Status != security.Rejected {
			t.Fatalf("err=%v asked=%d report=%+v", err, p.asked, res.Report)
		}
	})
	t.Run("no terminal", func(t *testing.T) {
		agent := &scriptedAgent{answers: []string{recon, candidates, verdict(needsValidationCritical)}}
		_, err := service(agent, fakeVCS{diff: "+x"}, &prompter{nonTTY: true}).Run(context.Background(), opts(t.TempDir()))
		var blocked *BlockedError
		if !errors.As(err, &blocked) || blocked.Findings[0].ID != "SEC-2" {
			t.Fatalf("an unsettled critical finding must block, got %v", err)
		}
	})
}

func TestNoChangesIsAnExplicitEmptyResult(t *testing.T) {
	res, err := service(&scriptedAgent{}, fakeVCS{diff: "  \n"}, &prompter{}).Run(context.Background(), opts(t.TempDir()))
	if err != nil || !res.Empty {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}

func TestFullScanChunksSourcesAndSkipsTests(t *testing.T) {
	root := t.TempDir()
	write := func(name, content string) {
		_ = os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0o755)
		_ = os.WriteFile(filepath.Join(root, name), []byte(content), 0o644)
	}
	write("a.go", strings.Repeat("a", 60))
	write("b.go", strings.Repeat("b", 60))
	write("a_test.go", "SECRET_TEST")
	write("node_modules/x.js", "DEP")
	write("logo.png", "PNG")
	files := []string{"a.go", "b.go", "a_test.go", "node_modules/x.js", "logo.png"}

	s := service(&scriptedAgent{}, fakeVCS{files: files}, &prompter{})
	chunks, _, err := s.targets(context.Background(), Options{Root: root, Scope: ScopeFull, MaxChunkBytes: 120})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(chunks, "|")
	if len(chunks) != 2 || strings.Contains(joined, "SECRET_TEST") || strings.Contains(joined, "DEP") || strings.Contains(joined, "PNG") {
		t.Fatalf("chunks = %q", chunks)
	}
}

func TestVCSErrorsPropagate(t *testing.T) {
	_, err := service(&scriptedAgent{}, fakeVCS{err: errors.New(`"--output=x" is not a commit`)}, &prompter{}).Run(context.Background(), opts(t.TempDir()))
	if err == nil {
		t.Fatal("a bad base must stop the audit")
	}
}
