package tddloop

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/jefmonjor/specforge/v6/internal/domain/tdd"
	"github.com/jefmonjor/specforge/v6/internal/ports"
)

type fakeCheckpoints struct {
	mu     sync.Mutex
	labels []string
	err    error
}

func (f *fakeCheckpoints) Save(_ context.Context, _, label string) (ports.Checkpoint, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.labels = append(f.labels, label)
	return ports.Checkpoint{ID: "x", Label: label}, f.err
}
func (f *fakeCheckpoints) List(context.Context, string) ([]ports.Checkpoint, error) { return nil, nil }
func (f *fakeCheckpoints) Restore(context.Context, string, string) error            { return nil }

func TestACheckpointIsSavedBeforeEveryAgentTurn(t *testing.T) {
	h := newHarness(t, specBody)
	cps := &fakeCheckpoints{}
	h.svc.d.Checkpoints = cps
	h.agent.turns = append(happyScenario("SDD_0001_001", test1, "reset.go"), happyScenario("SDD_0001_002", test2, "expiry.go")...)
	h.tests.outcomes = []tdd.Outcome{baseline(), red(1), green(), green(), red(1), green(), green()}
	if _, err := h.run(); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(cps.labels) != len(h.agent.prompts) {
		t.Fatalf("checkpoints %d, agent turns %d", len(cps.labels), len(h.agent.prompts))
	}
	if !strings.Contains(cps.labels[0], "scenario 1 (SDD_0001_001) · before RED") || !strings.Contains(cps.labels[3], "scenario 2 (SDD_0001_002) · before GREEN") {
		t.Fatalf("labels say where: %q", cps.labels)
	}
}

func TestAFailedCheckpointIsRecordedNotFatal(t *testing.T) {
	h := newHarness(t, specBody)
	h.svc.d.Checkpoints = &fakeCheckpoints{err: errors.New("disk full")}
	h.agent.turns = happyScenario("SDD_0001_001", test1, "reset.go")
	h.tests.outcomes = []tdd.Outcome{baseline(), red(1), green(), green()}
	_, _ = h.run(func(o *Options) { o.Scenario = 1 })
	st := h.state(t)
	found := false
	for _, c := range st.Checkpoints {
		found = found || (c.Step == "checkpoint" && c.Details == "disk full")
	}
	if !found || !st.Scenarios[0].Done {
		t.Fatalf("recorded and the loop went on: done=%v", st.Scenarios[0].Done)
	}
	if len(h.events.checkpointFailed) != 1 || h.events.checkpointFailed[0] != "disk full" {
		t.Fatalf("the developer is told once: %q", h.events.checkpointFailed)
	}
}

func TestSandboxesTakeNoCheckpoints(t *testing.T) {
	h := parallelHarness(t)
	cps := &fakeCheckpoints{}
	h.svc.d.Checkpoints = cps
	if _, err := h.run(inParallel); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(cps.labels) != 0 {
		t.Fatalf("checkpoints in disposable sandboxes: %q", cps.labels)
	}
}
