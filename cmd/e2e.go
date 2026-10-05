package cmd

import (
	"errors"
	"net/url"
	"path/filepath"

	"github.com/spf13/cobra"

	"specforge/internal/adapters/browser"
	"specforge/internal/adapters/fsys"
	"specforge/internal/app/clarify"
	"specforge/internal/app/e2erun"
	"specforge/internal/config"
	"specforge/internal/domain/spec"
	"specforge/internal/ui"
)

func (a *App) e2eCommand() *cobra.Command {
	var (
		o           config.Overrides
		baseURL     string
		scenarios   []int
		minPassRate float64
		maxSteps    int
		headed      bool
		insecure    bool
	)
	c := &cobra.Command{
		Use:   "e2e [spec] --url <url>",
		Short: "Verify the scenarios of an approved specification in a real browser",
		Long: `e2e drives Chrome, Chromium or Edge (found on your system or through
CHROME_PATH) scenario by scenario. The agent sees a compact view of the page
and answers with one typed action at a time; SpecForge validates each action
(known element, same origin) before running it, and checks each Then itself
against the evidence the agent names. The page is untrusted input: its text
never becomes an instruction.

Results, a screenshot per step and REPORT.md go to docs/e2e/<spec>/.`,
		Example: "  specforge e2e 0001 --url http://localhost:3000\n  specforge e2e 0001 --url http://localhost:3000 --scenario 2 --headed",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			u, err := url.Parse(baseURL)
			if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
				return errors.New("--url must be an absolute http(s) URL, e.g. http://localhost:3000")
			}
			if minPassRate < 0 || minPassRate > 100 {
				return errors.New("--min-pass-rate is a percentage between 0 and 100")
			}
			p, err := a.openProject(o, true)
			if err != nil {
				return err
			}
			entry, err := a.resolveSpec(ctx, p, argOrEmpty(args))
			if err != nil {
				return err
			}
			files := fsys.OS{}
			data, err := files.ReadFile(entry.Path)
			if err != nil {
				return err
			}
			if err := spec.Verify(string(data)); err != nil {
				return err
			}
			proc := a.NewProcess(a.log)
			ag, err := a.NewAgent(p.settings.Agent, proc, a.log)
			if err != nil {
				return err
			}
			br, err := a.LaunchBrowser(ctx, browser.Options{Headless: !headed, IgnoreCertErrors: insecure})
			if err != nil {
				return err
			}
			defer func() {
				if err := br.Close(); err != nil {
					a.log.Warn("closing the browser", "err", err)
				}
			}()

			con := a.console()
			con.Title(con.T("e2e.title", entry.Title))
			svc := e2erun.New(e2erun.Deps{
				Agent:   ag,
				Browser: br,
				Files:   files,
				Asker:   &clarify.Asker{Prompter: a.prompter(), Files: files, Now: a.Now, Lang: p.settings.Language},
				Events:  &ui.E2EEvents{C: con},
				Log:     a.log,
			})
			report, err := svc.Run(ctx, e2erun.Options{
				Root:         p.root,
				SpecPath:     entry.Path,
				BaseURL:      baseURL,
				Language:     p.settings.Language,
				Scenarios:    scenarios,
				MaxSteps:     maxSteps,
				MinPassRate:  minPassRate / 100,
				Model:        p.settings.ModelFor("e2e"),
				AgentTimeout: p.settings.AgentTimeout,
			})
			if err != nil {
				return err
			}
			dir := filepath.Join("docs", "e2e", filepath.Base(p.layout.SpecDir(entry.Path)))
			con.OK(con.T("e2e.passed", report.PassRate()*100, filepath.ToSlash(dir)))
			return nil
		},
	}
	f := c.Flags()
	f.StringVar(&baseURL, "url", "", "base URL of the running application (required)")
	f.IntSliceVar(&scenarios, "scenario", nil, "run only these scenario numbers (repeatable)")
	f.Float64Var(&minPassRate, "min-pass-rate", 100, "percentage of scenarios that must pass")
	f.IntVar(&maxSteps, "max-steps", 15, "maximum browser actions per scenario")
	f.BoolVar(&headed, "headed", false, "show the browser window")
	f.BoolVar(&insecure, "insecure", false, "accept invalid TLS certificates (local test servers only)")
	f.StringVar(&o.Agent, "agent", "", "claude | gemini (default: configured)")
	f.StringVar(&o.Model, "model", "", "model passed to the agent")
	_ = c.MarkFlagRequired("url")
	return c
}
