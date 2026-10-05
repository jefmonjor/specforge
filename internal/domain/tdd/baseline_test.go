package tdd

import (
	"reflect"
	"testing"
	"time"
)

var (
	broken  = TestRef{Suite: "pay", Name: "TestLegacyRounding"}
	flaky   = TestRef{Suite: "pay", Name: "TestClock"}
	created = TestRef{Suite: "pay", Name: "TestSDD_0001_001_Net"}
)

func failing(refs ...TestRef) Outcome {
	return Outcome{Compiled: true, Exact: true, Passed: 5, Failed: len(refs), Failures: refs}
}

func TestNewBaselineNeedsNamedFailures(t *testing.T) {
	at := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		o    Outcome
		want bool
	}{
		{"named failures", failing(flaky, broken), true},
		{"clean suite", failing(), true},
		{"does not build", Outcome{Compiled: false, Exact: true}, false},
		{"exit code only", Outcome{Compiled: true, Exact: false, Failed: 1}, false},
		{"counted but unnamed", Outcome{Compiled: true, Exact: true, Failed: 2, Failures: []TestRef{broken}}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b := NewBaseline(c.o, "go test ./...", at)
			if (b != nil) != c.want {
				t.Fatalf("baseline = %v, want present=%v", b, c.want)
			}
		})
	}
	b := NewBaseline(failing(flaky, broken), "go test ./...", at)
	if want := []TestRef{flaky, broken}; !reflect.DeepEqual(b.Failures, want) {
		t.Fatalf("failures are sorted by suite and name: got %v, want %v", b.Failures, want)
	}
}

func TestJudgeBlocksOnlyFreshFailures(t *testing.T) {
	b := &Baseline{Failures: []TestRef{broken, flaky}}
	cases := []struct {
		name               string
		o                  Outcome
		ok                 bool
		fresh, known, fixd []TestRef
	}{
		{"known still fail", failing(broken, flaky), true, nil, []TestRef{flaky, broken}, nil},
		{"a new failure blocks", failing(broken, created), false, []TestRef{created}, []TestRef{broken}, []TestRef{flaky}},
		{"known now pass", failing(), true, nil, nil, []TestRef{flaky, broken}},
		{"unnamed failure blocks", Outcome{Compiled: true, Exact: true, Passed: 3, Failed: 2, Failures: []TestRef{broken}}, false, nil, []TestRef{broken}, []TestRef{flaky}},
		{"no build blocks", Outcome{Compiled: false, Exact: true}, false, nil, nil, nil},
		{"nothing ran blocks", Outcome{Compiled: true, Exact: true}, false, nil, nil, []TestRef{flaky, broken}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			v := b.Judge(c.o)
			if v.OK != c.ok || !reflect.DeepEqual(v.Fresh, c.fresh) || !reflect.DeepEqual(v.Known, c.known) || !reflect.DeepEqual(v.Fixed, c.fixd) {
				t.Fatalf("Judge = %+v, want ok=%v fresh=%v known=%v fixed=%v", v, c.ok, c.fresh, c.known, c.fixd)
			}
		})
	}
}

func TestJudgeWithoutBaselineIsThePlainVerdict(t *testing.T) {
	var b *Baseline
	if v := b.Judge(failing(broken)); v.OK || !reflect.DeepEqual(v.Fresh, []TestRef{broken}) {
		t.Fatalf("every failure blocks without a baseline: %+v", v)
	}
	if v := b.Judge(failing()); !v.OK {
		t.Fatalf("a green suite passes: %+v", v)
	}
}

func TestRetireDropsFixedTests(t *testing.T) {
	b := &Baseline{Failures: []TestRef{broken, flaky}}
	b.Retire([]TestRef{flaky})
	if !reflect.DeepEqual(b.Failures, []TestRef{broken}) {
		t.Fatalf("Retire left %v", b.Failures)
	}
	if b.Has(flaky) || !b.Has(broken) {
		t.Fatal("Has does not reflect the retired test")
	}
	var none *Baseline
	none.Retire([]TestRef{broken}) // no panic
}

func TestNamesRendersSuiteAndName(t *testing.T) {
	if got := Names([]TestRef{broken, {Name: "solo"}}); got != "pay › TestLegacyRounding\nsolo" {
		t.Fatalf("Names = %q", got)
	}
}
