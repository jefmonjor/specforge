package process

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"specforge/internal/ports"
)

// TestHelperProcess is not a real test: it is the child process that the
// other tests launch through the test binary itself, which keeps them
// independent of the host shell. See os/exec's own tests for the pattern.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("SPECFORGE_HELPER") != "1" {
		return
	}
	defer os.Exit(0)
	switch os.Getenv("SPECFORGE_HELPER_MODE") {
	case "echo-stdin":
		data, _ := readAll(os.Stdin)
		fmt.Fprint(os.Stdout, "got:"+data)
	case "exit3":
		fmt.Fprint(os.Stderr, "boom")
		os.Exit(3)
	case "sleep":
		time.Sleep(10 * time.Second)
	case "flood":
		fmt.Fprint(os.Stdout, strings.Repeat("x", 100)+"VERDICT")
	}
}

func helper(mode string) ports.Command {
	return ports.Command{
		Name: os.Args[0],
		Args: []string{"-test.run=TestHelperProcess"},
		Env:  []string{"SPECFORGE_HELPER=1", "SPECFORGE_HELPER_MODE=" + mode},
	}
}

func readAll(f *os.File) (string, error) {
	var sb strings.Builder
	buf := make([]byte, 512)
	for {
		n, err := f.Read(buf)
		sb.Write(buf[:n])
		if err != nil {
			return sb.String(), nil
		}
	}
}

func TestRunPassesStdin(t *testing.T) {
	cmd := helper("echo-stdin")
	cmd.Stdin = strings.NewReader("a very long prompt")
	res, err := NewRunner(nil).Run(context.Background(), cmd)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.Success() || res.Stdout != "got:a very long prompt" {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestRunReportsNonZeroExitWithoutError(t *testing.T) {
	res, err := NewRunner(nil).Run(context.Background(), helper("exit3"))
	if err != nil {
		t.Fatalf("a non-zero exit must not be an error: %v", err)
	}
	if res.ExitCode != 3 || res.Stderr != "boom" {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestRunMissingToolIsErrToolNotFound(t *testing.T) {
	_, err := NewRunner(nil).Run(context.Background(), ports.Command{Name: "specforge-definitely-not-installed"})
	if !errors.Is(err, ports.ErrToolNotFound) {
		t.Fatalf("want ErrToolNotFound, got %v", err)
	}
}

func TestRunTimeoutIsErrTimeout(t *testing.T) {
	cmd := helper("sleep")
	cmd.Timeout = 200 * time.Millisecond
	start := time.Now()
	_, err := NewRunner(nil).Run(context.Background(), cmd)
	if !errors.Is(err, ports.ErrTimeout) {
		t.Fatalf("want ErrTimeout, got %v", err)
	}
	if time.Since(start) > 8*time.Second {
		t.Fatalf("timeout did not stop the process promptly")
	}
}

func TestRunCancelledContextIsContextError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(200 * time.Millisecond); cancel() }()
	_, err := NewRunner(nil).Run(ctx, helper("sleep"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
}

func TestTailBufferKeepsTheEnd(t *testing.T) {
	b := newTailBuffer(10)
	_, _ = b.Write([]byte("0123456789"))
	_, _ = b.Write([]byte("VERDICT"))
	got := b.String()
	if !strings.HasSuffix(got, "789VERDICT") || !strings.Contains(got, "truncated") {
		t.Fatalf("tail not preserved: %q", got)
	}
}

func TestCombined(t *testing.T) {
	cases := []struct {
		out, err, want string
	}{
		{"a", "", "a"},
		{"", "b", "b"},
		{"a\n", " b", "a\nb"},
	}
	for _, c := range cases {
		if got := (ports.CommandResult{Stdout: c.out, Stderr: c.err}).Combined(); got != c.want {
			t.Errorf("Combined(%q,%q) = %q, want %q", c.out, c.err, got, c.want)
		}
	}
}
