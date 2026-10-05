package testrun

import (
	"bufio"
	"context"
	"encoding/json"
	"strings"

	"specforge/internal/domain/tdd"
	"specforge/internal/ports"
)

// goEvent is one line of `go test -json` (cmd/test2json).
type goEvent struct {
	Action      string
	Package     string
	Test        string
	Output      string
	FailedBuild string
}

func (r *Runner) goTest(ctx context.Context, req ports.TestRequest) (tdd.Outcome, error) {
	args := []string{"test", "-json", "-count=1"}
	if req.Filter != "" {
		args = append(args, "-run", req.Filter)
	}
	args = append(args, "./...")
	res, err := r.run(ctx, req, "go", args...)
	if err != nil {
		return tdd.Outcome{}, err
	}
	return parseGoTest(res.Stdout, res.Stderr), nil
}

// parseGoTest reads a `go test -json` stream. Only top-level tests are
// counted; a subtest failure already fails its parent.
func parseGoTest(stdout, stderr string) tdd.Outcome {
	o := tdd.Outcome{Compiled: true, Exact: true}
	outputs := map[string]*strings.Builder{}
	var failedTests []string
	var buildOut strings.Builder

	sc := bufio.NewScanner(strings.NewReader(stdout))
	sc.Buffer(make([]byte, 1024*1024), 16*1024*1024)
	for sc.Scan() {
		line := sc.Text()
		var ev goEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			// Non-JSON lines are build output from older toolchains.
			if strings.TrimSpace(line) != "" {
				buildOut.WriteString(line + "\n")
			}
			continue
		}
		if ev.FailedBuild != "" || ev.Action == "build-fail" {
			o.Compiled = false
		}
		switch ev.Action {
		case "build-output":
			buildOut.WriteString(ev.Output)
		case "output":
			if ev.Test != "" {
				key := ev.Package + "." + ev.Test
				if outputs[key] == nil {
					outputs[key] = &strings.Builder{}
				}
				outputs[key].WriteString(ev.Output)
			} else if strings.Contains(ev.Output, "[setup failed]") || strings.Contains(ev.Output, "[build failed]") {
				o.Compiled = false
				buildOut.WriteString(ev.Output)
			}
		case "pass", "fail", "skip":
			if ev.Test == "" || strings.Contains(ev.Test, "/") {
				continue
			}
			switch ev.Action {
			case "pass":
				o.Passed++
			case "fail":
				o.Failed++
				o.Failures = append(o.Failures, tdd.TestRef{Suite: ev.Package, Name: ev.Test})
				failedTests = append(failedTests, ev.Package+"."+ev.Test)
			case "skip":
				o.Skipped++
			}
		}
	}

	// Older toolchains print build failures as plain text, outside the JSON
	// stream, ending with "FAIL pkg [build failed]".
	if hasBuildFailure(stderr) || hasBuildFailure(buildOut.String()) {
		o.Compiled = false
	}

	var out strings.Builder
	if !o.Compiled {
		out.WriteString(buildOut.String())
		out.WriteString(stderr)
	}
	for _, name := range failedTests {
		if b := outputs[name]; b != nil {
			out.WriteString(b.String())
		}
	}
	o.Output = clip(out.String())
	return o
}

func hasBuildFailure(s string) bool {
	return strings.Contains(s, "[build failed]") || strings.Contains(s, "[setup failed]")
}
