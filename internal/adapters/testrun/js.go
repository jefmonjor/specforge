package testrun

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jefmonjor/specforge/v6/internal/domain/tdd"
	"github.com/jefmonjor/specforge/v6/internal/ports"
)

// jestReport is the JSON report shared by Jest (--json) and Vitest
// (--reporter=json).
type jestReport struct {
	TestResults []struct {
		Name             string `json:"name"`
		Status           string `json:"status"`
		Message          string `json:"message"`
		AssertionResults []struct {
			FullName        string   `json:"fullName"`
			Status          string   `json:"status"`
			FailureMessages []string `json:"failureMessages"`
		} `json:"assertionResults"`
	} `json:"testResults"`
}

// npxMissing are the messages npx prints when the package is not installed
// locally and --no-install forbids downloading it.
var npxMissing = []string{"missing packages", "could not determine executable", "not found"}

func (r *Runner) jsonJS(ctx context.Context, req ports.TestRequest, base []string, outFlag, filterFlag string) (tdd.Outcome, error) {
	report, cleanup, err := tempReport("specforge-js-*.json")
	if err != nil {
		return tdd.Outcome{}, err
	}
	defer cleanup()

	args := append(append([]string{}, base...), outFlag+report)
	if req.Filter != "" {
		args = append(args, filterFlag, req.Filter)
	}
	res, err := r.run(ctx, req, "npx", args...)
	if err != nil {
		return tdd.Outcome{}, err
	}
	data, readErr := os.ReadFile(report)
	if readErr != nil {
		lower := strings.ToLower(res.Combined())
		for _, m := range npxMissing {
			if strings.Contains(lower, m) {
				return tdd.Outcome{}, fmt.Errorf("%w: %s (install it in the project's devDependencies)", ports.ErrToolNotFound, base[1])
			}
		}
		// The runner crashed before writing a report: config or syntax error.
		return tdd.Outcome{Compiled: false, Exact: true, Output: clip(res.Combined())}, nil
	}
	return parseJestReport(data, req.Root), nil
}

// parseJestReport converts a Jest/Vitest JSON report. A test file that
// failed with a message and no assertion results could not be loaded
// (syntax, type or import error): the tests did not compile. Failing tests
// are named by their file, relative to root, and their full name.
func parseJestReport(data []byte, root string) tdd.Outcome {
	var rep jestReport
	if err := json.Unmarshal(data, &rep); err != nil {
		return tdd.Outcome{Compiled: false, Exact: false, Output: "unreadable test report: " + err.Error()}
	}
	o := tdd.Outcome{Compiled: true, Exact: true}
	var out strings.Builder
	for _, file := range rep.TestResults {
		if file.Status == "failed" && len(file.AssertionResults) == 0 && strings.TrimSpace(file.Message) != "" {
			o.Compiled = false
			out.WriteString(file.Name + ":\n" + file.Message + "\n\n")
			continue
		}
		for _, a := range file.AssertionResults {
			switch a.Status {
			case "passed":
				o.Passed++
			case "failed":
				o.Failed++
				o.Failures = append(o.Failures, tdd.TestRef{Suite: relTo(root, file.Name), Name: a.FullName})
				out.WriteString(a.FullName + "\n" + strings.Join(a.FailureMessages, "\n") + "\n\n")
			default:
				o.Skipped++
			}
		}
	}
	o.Output = clip(out.String())
	return o
}

// relTo returns path relative to root with forward slashes, or path as it
// is when it is not under root.
func relTo(root, path string) string {
	if root == "" {
		return filepath.ToSlash(path)
	}
	rel, err := filepath.Rel(root, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}

// npm runs `npm test` for projects without Jest or Vitest. Without a report
// the verdict is the exit code: Exact is false and filtering is impossible.
func (r *Runner) npm(ctx context.Context, req ports.TestRequest) (tdd.Outcome, error) {
	res, err := r.run(ctx, req, "npm", "test", "--silent")
	if err != nil {
		return tdd.Outcome{}, err
	}
	o := tdd.Outcome{Compiled: true, Exact: false, Output: clip(res.Combined())}
	if res.Success() {
		o.Passed = 1
	} else {
		o.Failed = 1
	}
	return o, nil
}
