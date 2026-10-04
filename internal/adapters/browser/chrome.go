// Package browser implements ports.Browser with chromedp on a local Chrome,
// Chromium or Edge. It never mutates the page under test: selectors are
// computed from what the page already has.
package browser

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"

	"specforge/internal/domain/e2e"
	"specforge/internal/ports"
)

// ErrNoBrowser reports that no Chrome-compatible browser was found.
var ErrNoBrowser = errors.New("no Chrome, Chromium or Edge found: install one or set CHROME_PATH")

// Options configure the browser.
type Options struct {
	Headless bool
	// ExecPath overrides discovery; CHROME_PATH is read when empty.
	ExecPath string
	// IgnoreCertErrors accepts invalid TLS certificates. Off by default:
	// silently trusting a man in the middle is not a test setting to default.
	IgnoreCertErrors bool
	// ActionTimeout bounds every browser call (default 15s).
	ActionTimeout time.Duration
}

// Chrome is a running browser tab.
type Chrome struct {
	tab         context.Context
	tabCancel   context.CancelFunc
	allocCancel context.CancelFunc
	stopWatch   func() bool
	timeout     time.Duration
}

var _ ports.Browser = (*Chrome)(nil)

//go:embed snapshot.js
var snapshotJS string

// Launch starts the browser. Cancelling ctx kills it.
func Launch(ctx context.Context, o Options) (*Chrome, error) {
	path := o.ExecPath
	if path == "" {
		path = os.Getenv("CHROME_PATH")
	}
	if path == "" {
		path = discover()
	}
	if path == "" {
		return nil, ErrNoBrowser
	}
	if o.ActionTimeout <= 0 {
		o.ActionTimeout = 15 * time.Second
	}

	opts := []chromedp.ExecAllocatorOption{
		chromedp.ExecPath(path),
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.WindowSize(1280, 800),
		chromedp.Flag("disable-background-networking", true),
		chromedp.Flag("disable-extensions", true),
		chromedp.Flag("disable-sync", true),
		chromedp.Flag("mute-audio", true),
	}
	if o.Headless {
		opts = append(opts, chromedp.Headless, chromedp.DisableGPU)
	}
	if runtime.GOOS == "linux" && os.Geteuid() == 0 {
		opts = append(opts, chromedp.NoSandbox) // Chrome refuses to sandbox as root (containers)
	}
	if o.IgnoreCertErrors {
		opts = append(opts, chromedp.Flag("ignore-certificate-errors", true))
	}

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	tab, tabCancel := chromedp.NewContext(allocCtx)
	c := &Chrome{tab: tab, tabCancel: tabCancel, allocCancel: allocCancel, timeout: o.ActionTimeout}
	c.stopWatch = context.AfterFunc(ctx, func() { _ = c.Close() })

	// The first Run must use the tab context itself: chromedp ties the
	// browser's lifetime to the context of the first Run, so starting it
	// with a derived timeout context would kill it when that one ends.
	if err := chromedp.Run(tab); err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("starting %s: %w", path, err)
	}
	return c, nil
}

// run executes actions within the caller's context and the action timeout.
// A context derived from the tab with WithTimeout never closes the tab.
func (c *Chrome) run(ctx context.Context, actions ...chromedp.Action) error {
	tctx, cancel := context.WithTimeout(c.tab, c.timeout)
	defer cancel()
	stop := context.AfterFunc(ctx, cancel)
	defer stop()
	if err := chromedp.Run(tctx, actions...); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if errors.Is(tctx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("browser action timed out after %s", c.timeout)
		}
		return err
	}
	return nil
}

// Navigate implements ports.Browser.
func (c *Chrome) Navigate(ctx context.Context, url string) error {
	return c.run(ctx, chromedp.Navigate(url), chromedp.WaitReady("body", chromedp.ByQuery))
}

// Snapshot implements ports.Browser.
func (c *Chrome) Snapshot(ctx context.Context) (e2e.Snapshot, error) {
	var raw string
	if err := c.run(ctx, chromedp.Evaluate(snapshotJS, &raw)); err != nil {
		return e2e.Snapshot{}, fmt.Errorf("reading the page: %w", err)
	}
	var s e2e.Snapshot
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return e2e.Snapshot{}, fmt.Errorf("decoding the page snapshot: %w", err)
	}
	return s, nil
}

// Do implements ports.Browser. Actions must be validated first.
func (c *Chrome) Do(ctx context.Context, a e2e.Action) error {
	switch a.Type {
	case e2e.Click:
		return c.run(ctx, chromedp.Click(a.Selector, chromedp.ByQuery), chromedp.Sleep(300*time.Millisecond))
	case e2e.Type:
		// Clear through the DOM with an input event so reactive frameworks
		// see the change, then type like a user.
		clear := fmt.Sprintf(`(() => { const e = document.querySelector(%q); if (!e) return false; e.focus();
			e.value = ''; e.dispatchEvent(new Event('input', {bubbles: true})); return true; })()`, a.Selector)
		var ok bool
		if err := c.run(ctx, chromedp.WaitVisible(a.Selector, chromedp.ByQuery), chromedp.Evaluate(clear, &ok)); err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("selector %q not found", a.Selector)
		}
		return c.run(ctx, chromedp.SendKeys(a.Selector, a.Value, chromedp.ByQuery))
	case e2e.Select:
		js := fmt.Sprintf(`(() => { const e = document.querySelector(%q); if (!e) return false; e.value = %q;
			e.dispatchEvent(new Event('input', {bubbles: true})); e.dispatchEvent(new Event('change', {bubbles: true})); return true; })()`,
			a.Selector, a.Value)
		var ok bool
		if err := c.run(ctx, chromedp.Evaluate(js, &ok)); err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("selector %q not found", a.Selector)
		}
		return nil
	case e2e.Press:
		return c.run(ctx, chromedp.KeyEvent(keyFor(a.Value)), chromedp.Sleep(300*time.Millisecond))
	case e2e.Scroll:
		return c.run(ctx, chromedp.Evaluate(`window.scrollBy(0, window.innerHeight * 0.8)`, nil))
	case e2e.Wait:
		return c.run(ctx, chromedp.Sleep(time.Second))
	case e2e.Navigate:
		return c.Navigate(ctx, a.Value)
	}
	return fmt.Errorf("the browser cannot perform %q", a.Type)
}

// Screenshot implements ports.Browser.
func (c *Chrome) Screenshot(ctx context.Context, path string) error {
	var buf []byte
	if err := c.run(ctx, chromedp.CaptureScreenshot(&buf)); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, buf, 0o644)
}

// Close implements ports.Browser. It is safe to call more than once.
func (c *Chrome) Close() error {
	if c.stopWatch != nil {
		c.stopWatch()
	}
	c.tabCancel()
	c.allocCancel()
	return nil
}

var namedKeys = map[string]string{
	"enter": kb.Enter, "tab": kb.Tab, "escape": kb.Escape, "esc": kb.Escape, "backspace": kb.Backspace,
	"arrowdown": kb.ArrowDown, "arrowup": kb.ArrowUp, "arrowleft": kb.ArrowLeft, "arrowright": kb.ArrowRight,
}

func keyFor(v string) string {
	if k, ok := namedKeys[strings.ToLower(strings.TrimSpace(v))]; ok {
		return k
	}
	return v
}

func discover() string {
	var candidates []string
	switch runtime.GOOS {
	case "windows":
		for _, base := range []string{os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)"), os.Getenv("LOCALAPPDATA")} {
			if base == "" {
				continue
			}
			candidates = append(candidates,
				filepath.Join(base, `Google\Chrome\Application\chrome.exe`),
				filepath.Join(base, `Microsoft\Edge\Application\msedge.exe`),
				filepath.Join(base, `Chromium\Application\chrome.exe`))
		}
	case "darwin":
		candidates = []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
			"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
		}
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	for _, name := range []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "microsoft-edge", "brave-browser", "chrome", "msedge"} {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	return ""
}
