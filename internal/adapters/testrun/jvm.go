package testrun

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"specforge/internal/domain/tdd"
	"specforge/internal/ports"
)

var (
	mavenCompileErrors  = []string{"COMPILATION ERROR", "Compilation failure"}
	gradleCompileErrors = []string{"Compilation failed", "compileTestJava FAILED", "compileJava FAILED", "compileTestKotlin FAILED", "compileKotlin FAILED"}
)

func (r *Runner) maven(ctx context.Context, req ports.TestRequest) (tdd.Outcome, error) {
	args := []string{"-B", "test", "-DfailIfNoTests=false", "-Dsurefire.failIfNoSpecifiedTests=false"}
	if req.Filter != "" {
		// Class names or method names carrying the marker.
		args = append(args, "-Dtest=*"+req.Filter+"*,*#*"+req.Filter+"*")
	}
	start := r.now()
	res, err := r.run(ctx, req, "mvn", args...)
	if err != nil {
		return tdd.Outcome{}, err
	}
	return jvmOutcome(res, mavenCompileErrors, newerThan(req.Root, "surefire-reports", ".xml", start)), nil
}

func (r *Runner) gradle(ctx context.Context, req ports.TestRequest) (tdd.Outcome, error) {
	bin := "gradle"
	wrapper := "gradlew"
	if runtime.GOOS == "windows" {
		wrapper = "gradlew.bat"
	}
	if _, err := os.Stat(filepath.Join(req.Root, wrapper)); err == nil {
		bin = filepath.Join(req.Root, wrapper)
	}
	args := []string{"test", "--console=plain"}
	if req.Filter != "" {
		args = append(args, "--tests", "*"+req.Filter+"*")
	}
	start := r.now()
	res, err := r.run(ctx, req, bin, args...)
	if err != nil {
		return tdd.Outcome{}, err
	}
	return jvmOutcome(res, gradleCompileErrors, newerThan(req.Root, "test-results", ".xml", start)), nil
}

func jvmOutcome(res ports.CommandResult, compileMarkers []string, reports []string) tdd.Outcome {
	combined := res.Combined()
	for _, m := range compileMarkers {
		if strings.Contains(combined, m) {
			return tdd.Outcome{Compiled: false, Exact: true, Output: clip(combined)}
		}
	}
	passed, failed, skipped, out, ok := junitTotals(reports)
	if !ok {
		// No report: the build failed before tests ran, or the filter
		// matched nothing. Trust only what the exit code says.
		o := tdd.Outcome{Compiled: true, Exact: false, Output: clip(combined)}
		if !res.Success() {
			o.Failed = 1
		}
		return o
	}
	return tdd.Outcome{Compiled: true, Exact: true, Passed: passed, Failed: failed, Skipped: skipped, Output: clip(out)}
}
