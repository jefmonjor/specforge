package cmd

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jefmonjor/specforge/v6/internal/adapters/browser"
	"github.com/jefmonjor/specforge/v6/internal/adapters/process"
	"github.com/jefmonjor/specforge/v6/internal/config"
	"github.com/jefmonjor/specforge/v6/internal/ports"
)

// harness runs commands in a temporary project with a scripted agent.
type harness struct {
	t     *testing.T
	root  string
	home  config.Dirs
	agent *scriptedAgent
	out   *bytes.Buffer
	err   *bytes.Buffer
	stdin string
	tty   bool
	// browserLaunched records whether the e2e command got that far.
	browserLaunched bool
	// env is what Getenv sees.
	env map[string]string
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	home := t.TempDir()
	return &harness{
		t:     t,
		root:  t.TempDir(),
		home:  config.Dirs{Config: home, Logs: filepath.Join(home, "logs")},
		agent: &scriptedAgent{},
	}
}

func (h *harness) run(args ...string) int {
	h.t.Helper()
	h.out, h.err = &bytes.Buffer{}, &bytes.Buffer{}
	a := New()
	a.In = strings.NewReader(h.stdin)
	a.Out, a.Err = h.out, h.err
	a.Interactive = h.tty
	a.Getwd = func() (string, error) { return h.root, nil }
	a.Getenv = func(k string) string { return h.env[k] }
	a.Now = func() time.Time { return time.Date(2026, 10, 4, 10, 0, 0, 0, time.UTC) }
	a.UserDirs = func() (config.Dirs, error) { return h.home, nil }
	a.NewAgent = func(name string, _ ports.CommandRunner, _ *slog.Logger) (ports.Agent, error) {
		h.agent.root = h.root
		return h.agent, nil
	}
	a.LaunchBrowser = func(context.Context, browser.Options) (ports.Browser, error) {
		h.browserLaunched = true
		return nil, errors.New("no browser in unit tests")
	}
	return a.Run(context.Background(), args)
}

func (h *harness) write(rel, content string) {
	h.t.Helper()
	p := filepath.Join(h.root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		h.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		h.t.Fatal(err)
	}
}

func (h *harness) read(rel string) string {
	data, _ := os.ReadFile(filepath.Join(h.root, filepath.FromSlash(rel)))
	return string(data)
}

func (h *harness) expect(code int, args ...string) {
	h.t.Helper()
	if got := h.run(args...); got != code {
		h.t.Fatalf("specforge %s: exit %d, want %d\nstdout:\n%s\nstderr:\n%s", strings.Join(args, " "), got, code, h.out, h.err)
	}
}

// scriptedAgent answers each prompt with the first rule whose key the
// prompt contains, after applying the rule's file edits.
type scriptedAgent struct {
	mu      sync.Mutex
	root    string
	rules   []rule
	prompts []string
}

type rule struct {
	when  string
	files map[string]string
	reply string
}

func (s *scriptedAgent) Name() string { return "fake" }

func (s *scriptedAgent) Run(_ context.Context, req ports.AgentRequest) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prompts = append(s.prompts, req.Prompt)
	for _, r := range s.rules {
		if !strings.Contains(req.Prompt, r.when) {
			continue
		}
		for rel, content := range r.files {
			p := filepath.Join(s.root, filepath.FromSlash(rel))
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				return "", err
			}
			if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
				return "", err
			}
		}
		return r.reply, nil
	}
	return "", errors.New("unexpected prompt:\n" + req.Prompt)
}

func (s *scriptedAgent) Interactive(context.Context, ports.AgentRequest) error { return nil }

func TestMain(m *testing.M) {
	// Keep the real user configuration out of every test.
	os.Setenv(config.HomeEnv, os.TempDir())
	// And the developer's git configuration: a global include, a signing
	// key or a credential helper must not change what the tests see, on
	// any platform. Every git call, SpecForge's own included, inherits it.
	empty, err := os.CreateTemp("", "specforge-gitconfig-*")
	if err != nil {
		panic(err)
	}
	_ = empty.Close()
	os.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	os.Setenv("GIT_CONFIG_GLOBAL", empty.Name())
	code := m.Run()
	_ = os.Remove(empty.Name())
	os.Exit(code)
}

func requireTool(t *testing.T, name string) {
	t.Helper()
	if !process.Available(name) {
		t.Skipf("%s is not installed", name)
	}
}
