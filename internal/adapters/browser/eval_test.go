package browser

import "github.com/chromedp/chromedp"

func evaluate(js string, out any) chromedp.Action { return chromedp.Evaluate(js, out) }
