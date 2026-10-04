package testrun

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"specforge/internal/adapters/process"
	"specforge/internal/domain/stack"
	"specforge/internal/domain/tdd"
	"specforge/internal/ports"
)

var goProfile = stack.Profile{Kind: stack.Go, Runner: stack.RunnerGo}

func goModule(t *testing.T, files map[string]string) string {
	t.Helper()
	if !process.Available("go") {
		t.Skip("go toolchain not available")
	}
	root := t.TempDir()
	files["go.mod"] = "module example.com/m\n\ngo 1.22\n"
	for name, content := range files {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func runGo(t *testing.T, root, filter string) tdd.Outcome {
	t.Helper()
	o, err := New(process.NewRunner(nil)).Run(context.Background(), ports.TestRequest{Root: root, Profile: goProfile, Filter: filter})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return o
}

const impl = "package m\n\nfunc Balance(b, w int) int { return 0 }\n"

func TestGoFailingAssertionIsAValidRed(t *testing.T) {
	root := goModule(t, map[string]string{
		"m.go": impl,
		"m_test.go": "package m\nimport \"testing\"\n" +
			"func TestSDD_0001_001_Withdraw(t *testing.T) { if Balance(100, 30) != 70 { t.Fatal(\"want 70\") } }\n" +
			"func TestUnrelated(t *testing.T) {}\n",
	})
	o := runGo(t, root, "SDD_0001_001")
	if o.Red() != tdd.RedValid || o.Failed != 1 || o.Passed != 0 || !o.Exact {
		t.Fatalf("outcome = %+v", o)
	}
	if !strings.Contains(o.Output, "want 70") {
		t.Fatalf("failure output missing: %q", o.Output)
	}
}

func TestGoCompileErrorIsNotRed(t *testing.T) {
	// Regression: a test calling a function that does not exist used to
	// count as RED, and GREEN then burnt its attempts on a build error.
	root := goModule(t, map[string]string{
		"m.go":      impl,
		"m_test.go": "package m\nimport \"testing\"\nfunc TestSDD_0001_001_X(t *testing.T) { _ = Missing() }\n",
	})
	o := runGo(t, root, "SDD_0001_001")
	if o.Red() != tdd.RedNotCompiled {
		t.Fatalf("outcome = %+v", o)
	}
	if !strings.Contains(o.Output, "Missing") {
		t.Fatalf("build error not reported: %q", o.Output)
	}
}

func TestGoFilterThatMatchesNothingRunsNothing(t *testing.T) {
	root := goModule(t, map[string]string{
		"m.go":      impl,
		"m_test.go": "package m\nimport \"testing\"\nfunc TestOther(t *testing.T) { t.Fatal(\"must not run\") }\n",
	})
	if o := runGo(t, root, "SDD_0001_001"); o.Red() != tdd.RedNothingRan {
		t.Fatalf("outcome = %+v", o)
	}
}

func TestGoGreenCountsTopLevelTestsOnly(t *testing.T) {
	root := goModule(t, map[string]string{
		"m.go": "package m\n\nfunc Balance(b, w int) int { return b - w }\n",
		"m_test.go": "package m\nimport \"testing\"\nfunc TestSDD_0001_001_Withdraw(t *testing.T) {\n" +
			"\tfor _, c := range []int{1, 2} { t.Run(\"case\", func(t *testing.T) { _ = c }) }\n" +
			"\tif Balance(100, 30) != 70 { t.Fatal(\"want 70\") }\n}\n",
	})
	o := runGo(t, root, "")
	if !o.Green() || o.Passed != 1 {
		t.Fatalf("outcome = %+v", o)
	}
}

func TestParseGoTestLegacyBuildOutput(t *testing.T) {
	stdout := "# example.com/m [example.com/m.test]\n./m_test.go:3:5: undefined: Missing\n" +
		`{"Action":"fail","Package":"example.com/m","Elapsed":0}` + "\n"
	stderr := "FAIL\texample.com/m [build failed]\n"
	o := parseGoTest(stdout, stderr)
	if o.Compiled {
		t.Fatalf("legacy build failure not detected: %+v", o)
	}
}

func TestParseJestReport(t *testing.T) {
	load, _ := os.ReadFile("testdata/vitest-load-error.json")
	if o := parseJestReport(load); o.Compiled || !strings.Contains(o.Output, "Failed to load") {
		t.Errorf("load error must not compile: %+v", o)
	}
	mixed, _ := os.ReadFile("testdata/jest-mixed.json")
	o := parseJestReport(mixed)
	if !o.Compiled || o.Passed != 2 || o.Failed != 1 || o.Skipped != 1 || !strings.Contains(o.Output, "Received: false") {
		t.Errorf("mixed report = %+v", o)
	}
	if o := parseJestReport([]byte("{")); o.Compiled || o.Exact {
		t.Errorf("garbage report = %+v", o)
	}
}

func TestJUnitTotals(t *testing.T) {
	p, f, s, out, ok := junitTotals([]string{"testdata/surefire.xml"})
	if !ok || p != 1 || f != 2 || s != 0 || !strings.Contains(out, "Not implemented yet") {
		t.Errorf("surefire: %d %d %d ok=%v\n%s", p, f, s, ok, out)
	}
	p, f, s, _, ok = junitTotals([]string{"testdata/pytest.xml"})
	if !ok || p != 2 || f != 1 || s != 1 {
		t.Errorf("pytest wrapper: %d %d %d ok=%v", p, f, s, ok)
	}
	if _, _, _, _, ok := junitTotals([]string{"testdata/missing.xml"}); ok {
		t.Error("missing report must not be ok")
	}
}

type fakeProc struct {
	calls  []ports.Command
	result ports.CommandResult
	err    error
	onRun  func(ports.Command)
}

func (f *fakeProc) Run(_ context.Context, c ports.Command) (ports.CommandResult, error) {
	f.calls = append(f.calls, c)
	if f.onRun != nil {
		f.onRun(c)
	}
	return f.result, f.err
}

func TestMavenFilterAndCompileError(t *testing.T) {
	proc := &fakeProc{result: ports.CommandResult{ExitCode: 1, Stdout: "[ERROR] COMPILATION ERROR :\n[ERROR] cannot find symbol"}}
	o, err := New(proc).Run(context.Background(), ports.TestRequest{
		Root: t.TempDir(), Filter: "SDD_0001_002", Profile: stack.Profile{Kind: stack.Maven, Runner: stack.RunnerMaven},
	})
	if err != nil || o.Compiled {
		t.Fatalf("compile error must be reported: %+v %v", o, err)
	}
	if args := proc.calls[0].Args; !slices.Contains(args, "-Dtest=*SDD_0001_002*,*#*SDD_0001_002*") {
		t.Fatalf("filter not passed: %v", args)
	}
}

func TestJVMWithoutReportFallsBackToExitCode(t *testing.T) {
	proc := &fakeProc{result: ports.CommandResult{ExitCode: 1, Stdout: "BUILD FAILURE"}}
	o, err := New(proc).Run(context.Background(), ports.TestRequest{
		Root: t.TempDir(), Profile: stack.Profile{Kind: stack.Gradle, Runner: stack.RunnerGradle},
	})
	if err != nil || o.Exact || o.Failed != 1 || !o.Compiled {
		t.Fatalf("outcome = %+v %v", o, err)
	}
	if proc.calls[0].Name != "gradle" {
		t.Fatalf("expected gradle without a wrapper, got %s", proc.calls[0].Name)
	}
}

func TestVitestMissingIsToolNotFound(t *testing.T) {
	proc := &fakeProc{result: ports.CommandResult{ExitCode: 1, Stderr: "npm error npx canceled due to missing packages and no YES option: [\"vitest\"]"}}
	_, err := New(proc).Run(context.Background(), ports.TestRequest{
		Root: t.TempDir(), Profile: stack.Profile{Kind: stack.Node, Runner: stack.RunnerVitest},
	})
	if !errors.Is(err, ports.ErrToolNotFound) {
		t.Fatalf("want ErrToolNotFound, got %v", err)
	}
}

func TestVitestReadsTheReportItWasToldToWrite(t *testing.T) {
	report, _ := os.ReadFile("testdata/jest-mixed.json")
	proc := &fakeProc{onRun: func(c ports.Command) {
		for _, a := range c.Args {
			if path, ok := strings.CutPrefix(a, "--outputFile="); ok {
				_ = os.WriteFile(path, report, 0o600)
			}
		}
	}, result: ports.CommandResult{ExitCode: 1}}
	o, err := New(proc).Run(context.Background(), ports.TestRequest{
		Root: t.TempDir(), Filter: "SDD_0001_001", Profile: stack.Profile{Kind: stack.Node, Runner: stack.RunnerVitest},
	})
	if err != nil || o.Failed != 1 || o.Passed != 2 {
		t.Fatalf("outcome = %+v %v", o, err)
	}
	if args := proc.calls[0].Args; !slices.Contains(args, "-t") || !slices.Contains(args, "--no-install") {
		t.Fatalf("args = %v", args)
	}
}

func TestNPMIsInexact(t *testing.T) {
	proc := &fakeProc{result: ports.CommandResult{ExitCode: 0}}
	o, err := New(proc).Run(context.Background(), ports.TestRequest{Profile: stack.Profile{Kind: stack.Node, Runner: stack.RunnerNPM}})
	if err != nil || o.Exact || !o.Green() {
		t.Fatalf("outcome = %+v %v", o, err)
	}
}

func TestUnknownRunnerIsUnsupported(t *testing.T) {
	_, err := New(&fakeProc{}).Run(context.Background(), ports.TestRequest{})
	if !errors.Is(err, tdd.ErrUnsupportedStack) {
		t.Fatalf("want ErrUnsupportedStack, got %v", err)
	}
}

func TestClipKeepsHeadAndTail(t *testing.T) {
	long := "HEAD" + strings.Repeat("x", 3*maxOutput) + "TAIL"
	got := clip(long)
	if len(got) > maxOutput+100 || !strings.HasPrefix(got, "HEAD") || !strings.HasSuffix(got, "TAIL") {
		t.Fatalf("clip lost the ends (len %d)", len(got))
	}
}
