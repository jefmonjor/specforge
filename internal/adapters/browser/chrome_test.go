package browser

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"specforge/internal/domain/e2e"
)

const resetPage = `<!doctype html><html><head><title>Reset</title></head><body>
<h1>Reset your password</h1>
<form onsubmit="event.preventDefault(); document.getElementById('msg').textContent = 'Link sent to ' + document.getElementById('email').value;">
  <input id="email" placeholder="Email">
  <button type="submit">Send link</button>
  <button type="button">Send link</button>
</form>
<p id="msg" role="alert"></p>
</body></html>`

func launch(t *testing.T, timeout time.Duration) (*Chrome, string) {
	t.Helper()
	if os.Getenv("CHROME_PATH") == "" && discover() == "" {
		t.Skip("no Chrome-compatible browser installed")
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(resetPage))
	}))
	t.Cleanup(srv.Close)
	c, err := Launch(context.Background(), Options{Headless: true, ActionTimeout: timeout})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c, srv.URL
}

func TestDriveARealPageWithoutMutatingIt(t *testing.T) {
	c, url := launch(t, 10*time.Second)
	ctx := context.Background()
	if err := c.Navigate(ctx, url); err != nil {
		t.Fatal(err)
	}

	var before, after string
	_ = c.run(ctx, evaluate(`document.documentElement.outerHTML`, &before))
	snap, err := c.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_ = c.run(ctx, evaluate(`document.documentElement.outerHTML`, &after))
	if before != after {
		t.Fatal("a snapshot must not change the page under test")
	}

	if !snap.HasSelector("#email") || snap.Title != "Reset" {
		t.Fatalf("snapshot = %+v", snap)
	}
	// Two identical buttons get distinct, unique selectors.
	var buttons []string
	for _, el := range snap.Elements {
		if el.Tag == "button" {
			buttons = append(buttons, el.Selector)
		}
	}
	if len(buttons) != 2 || buttons[0] == buttons[1] {
		t.Fatalf("button selectors = %v", buttons)
	}

	if err := c.Do(ctx, e2e.Action{Type: e2e.Type, Selector: "#email", Value: "ana@example.com"}); err != nil {
		t.Fatal(err)
	}
	if err := c.Do(ctx, e2e.Action{Type: e2e.Click, Selector: buttons[0]}); err != nil {
		t.Fatal(err)
	}
	snap, _ = c.Snapshot(ctx)
	if !e2e.Verify(e2e.Evidence{Kind: e2e.EvidenceText, Value: "Link sent to ana@example.com"}, snap) {
		t.Fatalf("evidence not on the page: %q", snap.Text)
	}

	shot := filepath.Join(t.TempDir(), "s", "step.png")
	if err := c.Screenshot(ctx, shot); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(shot); err != nil || info.Size() == 0 {
		t.Fatal("screenshot not written")
	}
}

func TestAMissingSelectorTimesOutInsteadOfHanging(t *testing.T) {
	// Regression: WaitVisible on a hallucinated selector blocked forever.
	c, url := launch(t, 2*time.Second)
	ctx := context.Background()
	if err := c.Navigate(ctx, url); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	err := c.Do(ctx, e2e.Action{Type: e2e.Click, Selector: "#does-not-exist"})
	if err == nil || !strings.Contains(err.Error(), "timed out") || time.Since(start) > 6*time.Second {
		t.Fatalf("want a prompt timeout, got %v after %s", err, time.Since(start))
	}
}

func TestCancellingTheContextStopsTheBrowserCall(t *testing.T) {
	c, url := launch(t, 30*time.Second)
	if err := c.Navigate(context.Background(), url); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(300 * time.Millisecond); cancel() }()
	start := time.Now()
	err := c.Do(ctx, e2e.Action{Type: e2e.Click, Selector: "#does-not-exist"})
	if !errors.Is(err, context.Canceled) || time.Since(start) > 5*time.Second {
		t.Fatalf("want context.Canceled promptly, got %v after %s", err, time.Since(start))
	}
}
