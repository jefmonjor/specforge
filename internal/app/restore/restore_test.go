package restore

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/jefmonjor/specforge/v6/internal/ports"
)

type fake struct {
	list     []ports.Checkpoint
	saved    []string
	restored string
}

func (f *fake) Save(_ context.Context, _, label string) (ports.Checkpoint, error) {
	f.saved = append(f.saved, label)
	return ports.Checkpoint{ID: "now", Label: label}, nil
}
func (f *fake) List(context.Context, string) ([]ports.Checkpoint, error) { return f.list, nil }
func (f *fake) Restore(_ context.Context, _, id string) error            { f.restored = id; return nil }

func TestRestoreSavesTheStateBeforeThenRestores(t *testing.T) {
	f := &fake{list: []ports.Checkpoint{{ID: "b", Label: "scenario 2"}, {ID: "a", Label: "scenario 1"}}}
	res, err := Run(context.Background(), f, "/repo", "a")
	if err != nil || f.restored != "a" || res.Before.ID != "now" || !slices.Equal(f.saved, []string{"before restoring a"}) {
		t.Fatalf("res=%+v err=%v saved=%v restored=%q", res, err, f.saved, f.restored)
	}
	if _, err := Run(context.Background(), f, "/repo", Latest); err != nil || f.restored != "b" {
		t.Fatalf("latest is the newest: %q %v", f.restored, err)
	}
}

func TestRestoreRefusesWhatDoesNotExist(t *testing.T) {
	if _, err := Run(context.Background(), &fake{}, "/repo", Latest); !errors.Is(err, ErrNone) {
		t.Fatalf("want ErrNone, got %v", err)
	}
	f := &fake{list: []ports.Checkpoint{{ID: "a"}}}
	if _, err := Run(context.Background(), f, "/repo", "zzz"); err == nil || f.restored != "" || len(f.saved) != 0 {
		t.Fatal("an unknown checkpoint changes nothing")
	}
}
