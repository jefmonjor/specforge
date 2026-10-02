package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/chromedp/chromedp"
	"specforge/internal/adapters/storage"
	"specforge/internal/domain"
)

type ChromedpDriver struct {
	allocCtx context.Context
	cancel   context.CancelFunc
	taskCtx  context.Context
}

// NewChromedpDriver inicializa el driver buscando Chrome o Edge local
func NewChromedpDriver(headless bool) (*ChromedpDriver, error) {
	logger := storage.GetLogger()

	execPath := findBrowserExecutable()
	if execPath != "" {
		logger.Info("Navegador localizado para E2E: %s", execPath)
	} else {
		logger.Warn("No se encontró ruta explícita de navegador; chromedp intentará detección estándar.")
	}

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.DisableGPU,
		chromedp.Flag("ignore-certificate-errors", "1"),
		chromedp.WindowSize(1280, 800),
	)

	if headless {
		opts = append(opts, chromedp.Headless)
	} else {
		opts = append(opts, chromedp.Flag("headless", false))
	}

	if execPath != "" {
		opts = append(opts, chromedp.ExecPath(execPath))
	}

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	taskCtx, _ := chromedp.NewContext(allocCtx)

	return &ChromedpDriver{
		allocCtx: allocCtx,
		cancel:   cancel,
		taskCtx:  taskCtx,
	}, nil
}

// Close libera los recursos del navegador
func (d *ChromedpDriver) Close() {
	if d.cancel != nil {
		d.cancel()
	}
}

// Navigate navega a la URL y espera a que el body sea visible
func (d *ChromedpDriver) Navigate(ctx context.Context, targetURL string) error {
	return chromedp.Run(d.taskCtx,
		chromedp.Navigate(targetURL),
		chromedp.Sleep(1*time.Second),
		chromedp.WaitVisible("body", chromedp.ByQuery),
	)
}

// CaptureUISnapshot ejecuta JS para extraer el árbol de elementos interactivos
func (d *ChromedpDriver) CaptureUISnapshot(ctx context.Context) (*domain.UISnapshot, error) {
	const jsScript = `
(() => {
  const isVisible = (elem) => {
    if (!elem) return false;
    const style = window.getComputedStyle(elem);
    const rect = elem.getBoundingClientRect();
    return style.display !== 'none' && style.visibility !== 'hidden' && style.opacity !== '0' && rect.width > 0 && rect.height > 0;
  };

  const interactiveSelectors = 'a, button, input, select, textarea, [role="button"], [role="link"], [role="checkbox"], [role="menuitem"], [tabindex]:not([tabindex="-1"]), h1, h2, h3, p, span.error, div.alert, .message, [data-testid]';
  const nodes = Array.from(document.querySelectorAll(interactiveSelectors));
  const items = [];
  let counter = 1;

  for (const node of nodes) {
    if (!isVisible(node)) continue;
    const tag = node.tagName.toLowerCase();
    const text = (node.innerText || node.textContent || '').trim().replace(/\s+/g, ' ').slice(0, 80);
    const placeholder = node.getAttribute('placeholder') || '';
    const ariaLabel = node.getAttribute('aria-label') || '';
    const id = node.id || '';
    const name = node.getAttribute('name') || '';
    const type = node.getAttribute('type') || '';

    let selector = '';
    if (id) {
      selector = '#' + CSS.escape(id);
    } else if (name) {
      selector = tag + '[name="' + CSS.escape(name) + '"]';
    } else if (node.getAttribute('data-testid')) {
      selector = '[data-testid="' + CSS.escape(node.getAttribute('data-testid')) + '"]';
    } else if (placeholder) {
      selector = tag + '[placeholder="' + CSS.escape(placeholder) + '"]';
    } else if (ariaLabel) {
      selector = tag + '[aria-label="' + CSS.escape(ariaLabel) + '"]';
    } else {
      const dataAttr = 'data-sdd-id';
      let sddId = node.getAttribute(dataAttr);
      if (!sddId) {
        sddId = 'sdd-elem-' + counter++;
        node.setAttribute(dataAttr, sddId);
      }
      selector = '[' + dataAttr + '="' + sddId + '"]';
    }

    items.push({
      id: 'elem_' + items.length,
      tag: tag,
      type: type,
      text: text,
      placeholder: placeholder,
      aria_label: ariaLabel,
      selector: selector
    });
  }

  return JSON.stringify({
    url: window.location.href,
    title: document.title,
    elements: items
  });
})()
`

	var jsonStr string
	err := chromedp.Run(d.taskCtx, chromedp.Evaluate(jsScript, &jsonStr))
	if err != nil {
		return nil, fmt.Errorf("error extrayendo UI Tree de la página: %w", err)
	}

	var snapshot domain.UISnapshot
	if err := json.Unmarshal([]byte(jsonStr), &snapshot); err != nil {
		return nil, fmt.Errorf("error deserializando UI Tree: %w", err)
	}

	return &snapshot, nil
}

// ExecuteAction aplica una acción atómica en el DOM
func (d *ChromedpDriver) ExecuteAction(ctx context.Context, action domain.VisualAction) error {
	switch action.ActionType {
	case domain.ActionClick:
		if action.TargetSelector == "" {
			return fmt.Errorf("acción 'click' requiere target_selector")
		}
		return chromedp.Run(d.taskCtx,
			chromedp.WaitVisible(action.TargetSelector, chromedp.ByQuery),
			chromedp.Click(action.TargetSelector, chromedp.ByQuery),
			chromedp.Sleep(800*time.Millisecond),
		)

	case domain.ActionTypeInput:
		if action.TargetSelector == "" {
			return fmt.Errorf("acción 'type' requiere target_selector")
		}
		return chromedp.Run(d.taskCtx,
			chromedp.WaitVisible(action.TargetSelector, chromedp.ByQuery),
			chromedp.SetValue(action.TargetSelector, "", chromedp.ByQuery),
			chromedp.SendKeys(action.TargetSelector, action.Value, chromedp.ByQuery),
			chromedp.Sleep(400*time.Millisecond),
		)

	case domain.ActionWait:
		dur := 1 * time.Second
		return chromedp.Run(d.taskCtx, chromedp.Sleep(dur))

	case domain.ActionNavigate:
		if action.Value == "" {
			return fmt.Errorf("acción 'navigate' requiere URL en value")
		}
		return d.Navigate(ctx, action.Value)

	case domain.ActionAssert:
		return nil

	default:
		return fmt.Errorf("acción no soportada: %s", action.ActionType)
	}
}

// CaptureScreenshot captura la pantalla y la persiste en disco
func (d *ChromedpDriver) CaptureScreenshot(ctx context.Context, outputPath string) error {
	var buf []byte
	err := chromedp.Run(d.taskCtx, chromedp.CaptureScreenshot(&buf))
	if err != nil {
		return fmt.Errorf("error capturando screenshot: %w", err)
	}

	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("error creando directorio %s: %w", dir, err)
	}

	if err := os.WriteFile(outputPath, buf, 0644); err != nil {
		return fmt.Errorf("error guardando captura en %s: %w", outputPath, err)
	}

	return nil
}

func findBrowserExecutable() string {
	if runtime.GOOS == "windows" {
		candidates := []string{
			`C:\Program Files\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
			`C:\Program Files\Microsoft\Edge\Application\msedge.exe`,
		}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				return c
			}
		}
		if p, err := exec.LookPath("chrome.exe"); err == nil {
			return p
		}
		if p, err := exec.LookPath("msedge.exe"); err == nil {
			return p
		}
	} else if runtime.GOOS == "darwin" {
		candidates := []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
		}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				return c
			}
		}
	} else {
		// Linux
		for _, b := range []string{"google-chrome", "chromium-browser", "chromium", "microsoft-edge"} {
			if p, err := exec.LookPath(b); err == nil {
				return p
			}
		}
	}

	return ""
}
