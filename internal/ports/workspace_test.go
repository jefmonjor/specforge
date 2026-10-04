package ports

import (
	"reflect"
	"testing"
)

func TestSnapshotChanged(t *testing.T) {
	before := Snapshot{"kept.go": "1", "edited.go": "1", "reverted.go": "1"}
	after := Snapshot{"kept.go": "1", "edited.go": "2", "new.go": "1"}
	want := []string{"edited.go", "new.go", "reverted.go"}
	if got := before.Changed(after); !reflect.DeepEqual(got, want) {
		t.Fatalf("Changed = %v, want %v", got, want)
	}
	if got := after.Changed(after); len(got) != 0 {
		t.Fatalf("no change expected, got %v", got)
	}
}
