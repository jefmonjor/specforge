package logging

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func readLog(t *testing.T, dir string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, FileName))
	if err != nil {
		t.Fatalf("reading log: %v", err)
	}
	return string(data)
}

func TestDefaultLevelsKeepPromptsOutOfTheFile(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "logs")
	var console bytes.Buffer
	log, closer, err := New(Options{Dir: dir, Console: &console})
	if err != nil {
		t.Fatal(err)
	}
	log.Debug("debug-line")
	log.Info("info-line")
	log.Warn("warn-line")
	Trace(context.Background(), log, "prompt", "SECRET SOURCE CODE")
	_ = closer.Close()

	file := readLog(t, dir)
	if strings.Contains(file, "debug-line") || strings.Contains(file, "SECRET") {
		t.Fatalf("default file level must exclude debug and trace:\n%s", file)
	}
	if !strings.Contains(file, "info-line") || !strings.Contains(file, "warn-line") {
		t.Fatalf("file must contain info and warn:\n%s", file)
	}
	if strings.Contains(console.String(), "info-line") || !strings.Contains(console.String(), "warn-line") {
		t.Fatalf("console must show warn and above only:\n%s", console.String())
	}
}

func TestTraceIOIsOptIn(t *testing.T) {
	dir := t.TempDir()
	log, closer, err := New(Options{Dir: dir, TraceIO: true, Console: &bytes.Buffer{}})
	if err != nil {
		t.Fatal(err)
	}
	Trace(context.Background(), log, "prompt", "the body")
	_ = closer.Close()
	if file := readLog(t, dir); !strings.Contains(file, "level=TRACE") || !strings.Contains(file, "the body") {
		t.Fatalf("trace record missing:\n%s", file)
	}
}

func TestFilesArePrivate(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permissions")
	}
	dir := filepath.Join(t.TempDir(), "logs")
	log, closer, err := New(Options{Dir: dir, Console: &bytes.Buffer{}})
	if err != nil {
		t.Fatal(err)
	}
	log.Info("x")
	_ = closer.Close()

	for path, want := range map[string]os.FileMode{dir: 0o700, filepath.Join(dir, FileName): 0o600} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != want {
			t.Errorf("%s has mode %o, want %o", path, got, want)
		}
	}
}
