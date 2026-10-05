package testrun

import (
	"encoding/xml"
	"os"
	"strings"

	"github.com/jefmonjor/specforge/v6/internal/domain/tdd"
)

// junitSuite covers the JUnit XML written by Maven Surefire, Gradle and
// pytest: either a <testsuite> root or a <testsuites> wrapper.
type junitSuite struct {
	XMLName  xml.Name
	Tests    int          `xml:"tests,attr"`
	Failures int          `xml:"failures,attr"`
	Errors   int          `xml:"errors,attr"`
	Skipped  int          `xml:"skipped,attr"`
	Suites   []junitSuite `xml:"testsuite"`
	Cases    []junitCase  `xml:"testcase"`
}

type junitCase struct {
	Name      string        `xml:"name,attr"`
	Classname string        `xml:"classname,attr"`
	Failure   *junitProblem `xml:"failure"`
	Error     *junitProblem `xml:"error"`
}

type junitProblem struct {
	Message string `xml:"message,attr"`
	Body    string `xml:",chardata"`
}

// junitOutcome sums the counts of every suite in the given report files,
// names the failing test cases and collects their messages. Errors count
// as failures: an exception such as NotImplementedError is a legitimate RED
// once the test compiled. ok is false when no report could be read.
func junitOutcome(paths []string) (o tdd.Outcome, ok bool) {
	o = tdd.Outcome{Compiled: true, Exact: true}
	var out strings.Builder
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var root junitSuite
		if xml.Unmarshal(data, &root) != nil {
			continue
		}
		ok = true
		for _, s := range flatten(root) {
			o.Failed += s.Failures + s.Errors
			o.Skipped += s.Skipped
			o.Passed += s.Tests - s.Failures - s.Errors - s.Skipped
			for _, c := range s.Cases {
				failed := false
				for _, prob := range []*junitProblem{c.Failure, c.Error} {
					if prob != nil {
						failed = true
						out.WriteString(c.Classname + "." + c.Name + ": " + prob.Message + "\n" + strings.TrimSpace(prob.Body) + "\n\n")
					}
				}
				if failed {
					o.Failures = append(o.Failures, tdd.TestRef{Suite: c.Classname, Name: c.Name})
				}
			}
		}
	}
	o.Output = clip(out.String())
	return o, ok
}

func flatten(s junitSuite) []junitSuite {
	if s.XMLName.Local == "testsuite" {
		return []junitSuite{s}
	}
	var out []junitSuite
	for _, child := range s.Suites {
		out = append(out, flatten(child)...)
	}
	return out
}
