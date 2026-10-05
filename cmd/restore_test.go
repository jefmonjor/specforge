package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jefmonjor/specforge/v6/internal/adapters/process"
	"github.com/jefmonjor/specforge/v6/internal/adapters/vcs"
)

func TestRestoreListsAndBringsBackACheckpoint(t *testing.T) {
	requireTool(t, "git")
	h := newHarness(t)
	h.write("main.go", "package main\n")
	gitInit(t, h.root)
	h.expect(0, "init", "--agent", "claude", "--language", "en")
	h.expect(0, "restore")
	if !strings.Contains(h.err.String()+h.out.String(), "no checkpoint yet") {
		t.Fatalf("an empty list says why:\n%s", h.err)
	}

	h.write("work.go", "package main // uncommitted work\n")
	cp, err := vcs.New(process.NewRunner(nil)).Save(context.Background(), h.root, "0001 · scenario 1 (SDD_0001_001) · before GREEN")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(h.root, "work.go")); err != nil {
		t.Fatal(err)
	}
	h.expect(0, "restore")
	if !strings.Contains(h.err.String()+h.out.String(), cp.ID+" · ") {
		t.Fatalf("the list shows the checkpoint:\n%s", h.err)
	}
	h.expect(0, "restore", "latest")
	if data, _ := os.ReadFile(filepath.Join(h.root, "work.go")); string(data) != "package main // uncommitted work\n" {
		t.Fatalf("the work is back: %q", data)
	}
	if !strings.Contains(h.err.String(), "undoes this") {
		t.Fatalf("it says how to undo:\n%s", h.err)
	}
	h.expect(1, "restore", "no-such-checkpoint")
}
