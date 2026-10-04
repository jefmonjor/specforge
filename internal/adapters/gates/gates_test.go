package gates

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"specforge/internal/domain/legacy"
	"specforge/internal/domain/quality"
	"specforge/internal/domain/stack"
	"specforge/internal/ports"
)

type fakeProc struct {
	results map[string]ports.CommandResult
	errs    map[string]error
	calls   []string
}

func (f *fakeProc) Run(_ context.Context, c ports.Command) (ports.CommandResult, error) {
	key := c.Name
	if c.Name == "npx" && len(c.Args) > 1 {
		key = "npx " + c.Args[1]
	}
	f.calls = append(f.calls, c.String())
	return f.results[key], f.errs[key]
}

var (
	goP   = stack.Profile{Kind: stack.Go, Runner: stack.RunnerGo}
	nodeP = stack.Profile{Kind: stack.Node, Runner: stack.RunnerVitest}
)

func TestForProfileSelectsGates(t *testing.T) {
	names := func(gs []ports.Gate) string {
		var n []string
		for _, g := range gs {
			n = append(n, g.Name())
		}
		return strings.Join(n, ",")
	}
	if got := names(ForProfile(goP, nil, quality.DefaultThresholds())); got != "lint,duplication" {
		t.Errorf("go gates = %s", got)
	}
	if got := names(ForProfile(nodeP, nil, quality.DefaultThresholds())); got != "lint,duplication,dead-code,mutation" {
		t.Errorf("node gates = %s", got)
	}
}

func TestMissingToolIsSkippedNeverPassed(t *testing.T) {
	// Regression: jscpd, knip and stryker reported "passed" whenever npx
	// could not run them.
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, "package.json"), []byte(`{}`), 0o644)
	_ = os.WriteFile(filepath.Join(root, "stryker.conf.json"), []byte(`{}`), 0o644)
	missing := ports.CommandResult{ExitCode: 1, Stderr: "npm error npx canceled due to missing packages and no YES option"}
	proc := &fakeProc{results: map[string]ports.CommandResult{"npx jscpd": missing, "npx knip": missing, "npx stryker": missing}}

	for _, g := range []ports.Gate{&Duplication{proc: proc}, &DeadCode{proc: proc}, &Mutation{proc: proc, min: 80}} {
		res, err := g.Check(context.Background(), root, nodeP)
		if err != nil {
			t.Fatalf("%s: %v", g.Name(), err)
		}
		if res.Status != quality.Skipped {
			t.Errorf("%s: status = %s, want skipped (%s)", g.Name(), res.Status, res.Summary)
		}
	}
}

func TestDuplicationJudge(t *testing.T) {
	data, _ := os.ReadFile("testdata/jscpd-report.json")
	res, _ := (&Duplication{max: 5}).judge(data)
	if res.Status != quality.Passed {
		t.Errorf("4.2%% under a 5%% max must pass: %+v", res)
	}
	res, _ = (&Duplication{max: 0}).judge(data)
	if res.Status != quality.Failed || !strings.Contains(res.Details, "src/a.ts ↔ src/b.ts") {
		t.Errorf("4.2%% over 0%% must fail with the clone list: %+v", res)
	}
	if res, _ := (&Duplication{}).judge([]byte("nope")); res.Status != quality.Skipped {
		t.Errorf("garbage report must be skipped: %+v", res)
	}
}

func TestDeadCodeJudge(t *testing.T) {
	data, _ := os.ReadFile("testdata/knip.json")
	res, _ := (&DeadCode{}).judge("Some banner\n" + string(data))
	if res.Status != quality.Failed || !strings.Contains(res.Summary, "4 issue(s)") {
		t.Errorf("knip report = %+v", res)
	}
	res, _ = (&DeadCode{}).judge(`{"files":[],"issues":[{"file":"a.ts","exports":[]}]}`)
	if res.Status != quality.Passed {
		t.Errorf("clean knip report = %+v", res)
	}
}

func TestMutationJudge(t *testing.T) {
	data, _ := os.ReadFile("testdata/mutation.json")
	res, _ := (&Mutation{min: 80}).judge(data)
	if res.Status != quality.Failed || !strings.Contains(res.Summary, "50.0%") || !strings.Contains(res.Details, "src/reset.ts:5 BooleanLiteral") {
		t.Errorf("mutation report = %+v", res)
	}
	if res, _ := (&Mutation{min: 50}).judge(data); res.Status != quality.Passed {
		t.Errorf("50%% meets a 50%% minimum: %+v", res)
	}
}

func TestMutationWithoutConfigIsSkipped(t *testing.T) {
	res, err := (&Mutation{proc: &fakeProc{}}).Check(context.Background(), t.TempDir(), nodeP)
	if err != nil || res.Status != quality.Skipped {
		t.Fatalf("res = %+v %v", res, err)
	}
}

func TestNodeLintWithoutScriptIsSkipped(t *testing.T) {
	// Regression: `npm run lint --if-present` passed when there was no script.
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"scripts":{"test":"vitest"}}`), 0o644)
	res, err := (&Lint{proc: &fakeProc{}}).Check(context.Background(), root, nodeP)
	if err != nil || res.Status != quality.Skipped {
		t.Fatalf("res = %+v %v", res, err)
	}
}

func TestPythonLintDistinguishesIssuesFromCrashes(t *testing.T) {
	py := stack.Profile{Kind: stack.Python}
	for code, want := range map[int]quality.Status{0: quality.Passed, 1: quality.Failed, 2: quality.Skipped} {
		proc := &fakeProc{results: map[string]ports.CommandResult{"ruff": {ExitCode: code}}}
		res, err := (&Lint{proc: proc}).Check(context.Background(), t.TempDir(), py)
		if err != nil || res.Status != want {
			t.Errorf("ruff exit %d: got %+v %v, want %s", code, res, err, want)
		}
	}
	proc := &fakeProc{errs: map[string]error{"ruff": ports.ErrToolNotFound}}
	if res, _ := (&Lint{proc: proc}).Check(context.Background(), t.TempDir(), py); res.Status != quality.Skipped {
		t.Errorf("missing ruff must be skipped: %+v", res)
	}
}

func TestJVMLintIsSkippedNotPassed(t *testing.T) {
	res, _ := (&Lint{}).Check(context.Background(), t.TempDir(), stack.Profile{Kind: stack.Maven})
	if res.Status != quality.Skipped {
		t.Fatalf("res = %+v", res)
	}
}

func TestCancelledContextStopsTheGates(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	proc := &fakeProc{errs: map[string]error{"ruff": context.Canceled}}
	if _, err := Run(ctx, []ports.Gate{&Lint{proc: proc}}, t.TempDir(), stack.Profile{Kind: stack.Python}); err == nil {
		t.Fatal("cancellation must surface as an error")
	}
}

func TestMigrationGate(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "pom.xml"), []byte("<maven.compiler.release>21</maven.compiler.release>"), 0o644)
	os.MkdirAll(filepath.Join(root, "src/main/java/a"), 0o755)
	g := &Migration{Target: legacy.Target{JavaRelease: 21, ForbiddenImports: []string{"javax.servlet"}}}
	maven := stack.Profile{Kind: stack.Maven}
	if !g.Applies(maven) || g.Applies(stack.Profile{Kind: stack.Go}) || (&Migration{}).Applies(maven) {
		t.Fatal("applies")
	}
	res, err := g.Check(context.Background(), root, maven)
	if err != nil || res.Status != quality.Passed {
		t.Fatalf("%+v %v", res, err)
	}
	os.WriteFile(filepath.Join(root, "src/main/java/a/W.java"), []byte("package a;\nimport javax.servlet.Filter;\n"), 0o644)
	res, _ = g.Check(context.Background(), root, maven)
	if res.Status != quality.Failed || !strings.Contains(res.Details, "src/main/java/a/W.java:2") {
		t.Fatalf("%+v", res)
	}
}

func TestMavenLintRunsPMDOnlyWhenTheBuildDeclaresIt(t *testing.T) {
	root := t.TempDir()
	maven := stack.Profile{Kind: stack.Maven, Runner: stack.RunnerMaven}
	os.WriteFile(filepath.Join(root, "pom.xml"), []byte("<project/>"), 0o644)
	proc := &fakeProc{results: map[string]ports.CommandResult{}}
	g := &Lint{proc: proc}
	if res, _ := g.Check(context.Background(), root, maven); res.Status != quality.Skipped || len(proc.calls) != 0 {
		t.Fatalf("%+v %v", res, proc.calls)
	}
	os.WriteFile(filepath.Join(root, "pom.xml"), []byte("<artifactId>maven-pmd-plugin</artifactId>"), 0o644)
	proc.results["mvn"] = ports.CommandResult{ExitCode: 1, Stdout: "[WARNING] PMD Failure: a.Payroll:6 Rule:LooseCoupling Priority:3 Avoid Vector.\n[ERROR] PMD 7.17.0 has found 1 violation."}
	res, _ := g.Check(context.Background(), root, maven)
	if res.Status != quality.Failed || res.Details != "a.Payroll:6 Rule:LooseCoupling Priority:3 Avoid Vector." {
		t.Fatalf("%+v", res)
	}
	proc.results["mvn"] = ports.CommandResult{ExitCode: 1, Stdout: "[ERROR] COMPILATION ERROR"}
	if res, _ := g.Check(context.Background(), root, maven); res.Status != quality.Skipped {
		t.Fatalf("%+v", res)
	}
	proc.results["mvn"] = ports.CommandResult{}
	if res, _ := g.Check(context.Background(), root, maven); res.Status != quality.Passed {
		t.Fatalf("%+v", res)
	}
}
