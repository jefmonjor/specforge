package verifier

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"specforge/internal/adapters/fsys"
	"specforge/internal/adapters/process"
	"specforge/internal/adapters/scratch"
	"specforge/internal/adapters/workspace"
	"specforge/internal/ports"
)

type agent struct {
	replies []func(req ports.AgentRequest) string
	reqs    []ports.AgentRequest
}

func (a *agent) Name() string { return "fake" }
func (a *agent) Run(_ context.Context, r ports.AgentRequest) (string, error) {
	a.reqs = append(a.reqs, r)
	if len(a.replies) == 0 {
		return "", errors.New("unexpected call")
	}
	f := a.replies[0]
	a.replies = a.replies[1:]
	return f(r), nil
}
func (a *agent) Interactive(context.Context, ports.AgentRequest) error { return nil }

func says(s string) func(ports.AgentRequest) string {
	return func(ports.AgentRequest) string { return "```json\n" + s + "\n```" }
}

const complete = `{"verdicts":[{"id":"INV-01","status":"unmet","command":"echo net: -5","observed":"net: -5"},{"id":"SDD_0001_001","status":"met"}],` +
	`"blockers":[{"id":"INV-01","command":"echo net: -5","observed":"net: -5","expected":"error"}],"advisories":[],` +
	`"regression_tests":[{"path":"net_regression_test.go","covers":["INV-01"],"content":"package m"}]}`

func setup(t *testing.T, replies ...func(ports.AgentRequest) string) (*Service, *agent, Request) {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "net.go"), []byte("package m\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := &agent{replies: replies}
	proc := process.NewRunner(nil)
	svc := New(Deps{Agent: a, Proc: proc, Scratch: scratch.New(proc, 0), Workspace: workspace.New(proc), Files: fsys.OS{}})
	req := Request{Root: root, Language: "en", Stack: "go", SpecTitle: "Net pay", Spec: "INV-01: net ≥ 0",
		Required: []string{"INV-01", "SDD_0001_001"}, Report: filepath.Join(root, "specs", "verify.json"),
		Model: func(p string) string { return "m-" + p }}
	return svc, a, req
}

func TestVerifyRunsInACopyAndKeepsTheReport(t *testing.T) {
	svc, a, req := setup(t, func(r ports.AgentRequest) string {
		_ = os.WriteFile(filepath.Join(r.Dir, "probe_test.go"), []byte("package m"), 0o644) // allowed: it is the copy
		return says(complete)(r)
	})
	res, err := svc.Verify(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if a.reqs[0].Dir == req.Root || a.reqs[0].Model != "m-verify" || !strings.Contains(a.reqs[0].Prompt, "`INV-01`, `SDD_0001_001`") {
		t.Fatalf("the verifier works in a copy: %+v", a.reqs[0].Dir)
	}
	if _, err := os.Stat(a.reqs[0].Dir); !os.IsNotExist(err) {
		t.Fatal("the copy is removed")
	}
	if len(res.Report.Blockers) != 1 || res.Report.RegressionTests[0].Covers[0] != "INV-01" {
		t.Fatalf("report = %+v", res.Report)
	}
	if data, _ := os.ReadFile(req.Report); !strings.Contains(string(data), `"INV-01"`) {
		t.Fatal("the report is written")
	}
}

func TestAnIncompleteReportIsRetriedThenFailsClosed(t *testing.T) {
	missing := `{"verdicts":[{"id":"INV-01","status":"met"}],"blockers":[],"advisories":[],"regression_tests":[]}`
	svc, a, req := setup(t, says(missing), says(missing))
	_, err := svc.Verify(context.Background(), req)
	var step *StepError
	if !errors.As(err, &step) || !strings.Contains(err.Error(), "no verdict for SDD_0001_001") {
		t.Fatalf("want StepError, got %v", err)
	}
	if !strings.Contains(a.reqs[1].Prompt, "no verdict for SDD_0001_001") {
		t.Fatal("the retry says what is missing")
	}
}

func TestAVerifierThatTouchesTheProjectIsRefused(t *testing.T) {
	var root string
	svc, _, req := setup(t, func(r ports.AgentRequest) string {
		_ = os.WriteFile(filepath.Join(root, "net.go"), []byte("package m // patched\n"), 0o644)
		return says(complete)(r)
	})
	root = req.Root
	_, err := svc.Verify(context.Background(), req)
	var touched *TouchedError
	if !errors.As(err, &touched) || touched.Files[0] != "net.go" {
		t.Fatalf("want TouchedError, got %v", err)
	}
}

func TestATooLargeProjectIsSkippedWithTheReason(t *testing.T) {
	svc, a, req := setup(t)
	svc.d.Scratch = scratch.New(process.NewRunner(nil), 1)
	res, err := svc.Verify(context.Background(), req)
	if err != nil || !strings.Contains(res.Skipped, "too large") || len(a.reqs) != 0 {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}

func TestABlockerSpecForgeCannotReproduceIsRefused(t *testing.T) {
	reasoned := `{"verdicts":[{"id":"INV-01","status":"unmet"},{"id":"SDD_0001_001","status":"met"}],` +
		`"blockers":[{"id":"INV-01","command":"static review of net.go","observed":"deductions are ignored","expected":"0"}],"advisories":[],"regression_tests":[]}`
	reproducible := `{"verdicts":[{"id":"INV-01","status":"unmet"},{"id":"SDD_0001_001","status":"met"}],` +
		`"blockers":[{"id":"INV-01","command":"echo net: -5","observed":"net:   -5\nmore lines","expected":"an error"}],"advisories":[],"regression_tests":[]}`
	svc, a, req := setup(t, says(reasoned), says(reproducible))
	res, err := svc.Verify(context.Background(), req)
	if err != nil {
		t.Fatalf("the second, reproducible answer is accepted: %v", err)
	}
	if !strings.Contains(a.reqs[1].Prompt, "SpecForge ran `static review of net.go`") || len(res.Report.Blockers) != 1 {
		t.Fatalf("the retry says which blocker did not reproduce:\n%s", a.reqs[1].Prompt)
	}
}
