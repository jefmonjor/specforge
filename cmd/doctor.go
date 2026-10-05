package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/jefmonjor/specforge/v6/internal/adapters/agent"
	"github.com/jefmonjor/specforge/v6/internal/adapters/browser"
	"github.com/jefmonjor/specforge/v6/internal/adapters/fsys"
	"github.com/jefmonjor/specforge/v6/internal/adapters/gates"
	"github.com/jefmonjor/specforge/v6/internal/adapters/process"
	"github.com/jefmonjor/specforge/v6/internal/adapters/testrun"
	"github.com/jefmonjor/specforge/v6/internal/app/doctor"
	"github.com/jefmonjor/specforge/v6/internal/app/guardhook"
	"github.com/jefmonjor/specforge/v6/internal/app/layout"
	"github.com/jefmonjor/specforge/v6/internal/buildinfo"
	"github.com/jefmonjor/specforge/v6/internal/config"
	"github.com/jefmonjor/specforge/v6/internal/domain/legacy"
	"github.com/jefmonjor/specforge/v6/internal/domain/stack"
	"github.com/jefmonjor/specforge/v6/internal/ports"
)

func (a *App) doctorCommand() *cobra.Command {
	var o config.Overrides
	c := &cobra.Command{
		Use:   "doctor",
		Short: "Check that this machine and project have what SpecForge needs",
		Long: `doctor checks, without changing anything: your configuration, the agent
and git, the stack's test runner, the tools of every quality gate, a
browser for e2e, the legacy repository of a rewrite and the
destructive-command guard. Each line says what was found and, when
something is missing, how to install it.

Exit code 4 when something required is missing; 0 otherwise, warnings
included.`,
		Example: "  specforge doctor\n  specforge doctor --json",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			wd, err := a.Getwd()
			if err != nil {
				return err
			}
			p, cfgErr := a.openProject(o, false)
			if cfgErr != nil {
				s, _ := config.Resolve(config.User{}, config.Project{}, o, false)
				root := findRoot(wd)
				p = project{root: root, settings: s}
			}
			proc := a.NewProcess(a.log)
			profiles, stackErr := a.doctorStacks(p)
			target := legacy.Target{JavaRelease: p.settings.Migration.JavaRelease, ForbiddenImports: p.settings.Migration.ForbiddenImports}
			report, err := doctor.Run(cmd.Context(), doctor.Deps{
				Proc: proc,
				Find: func(name string) (string, bool) {
					path, err := process.Resolve(name)
					return path, err == nil
				},
				Files:  fsys.OS{},
				Runner: testrun.New(proc),
				Gates: func(prof stack.Profile) []ports.Probe {
					return gates.ProbesFor(prof, proc, p.settings.Quality, target)
				},
				Browser: browser.Find,
				Guard:   a.guardProbe(p),
			}, doctor.Options{
				Root:        p.root,
				Agent:       p.settings.Agent,
				AgentBinary: agent.Flavors[p.settings.Agent].Binary,
				Config:      cfgErr,
				Profiles:    profiles,
				StackErr:    stackErr,
				Legacy:      p.legacyDir(""),
				Commit:      p.settings.Commit,
				Version:     buildinfo.String(),
			})
			if ok, jerr := a.emit(report); ok {
				return firstErr(jerr, err)
			}
			a.printDoctor(report)
			return err
		},
	}
	c.Flags().StringVar(&o.Agent, "agent", "", "check this agent instead of the configured one")
	c.Flags().StringVar(&o.Stack, "stack", "", "check this stack instead of the detected ones")
	return c
}

// doctorStacks returns the configured stack, or every stack detected.
func (a *App) doctorStacks(p project) ([]stack.Profile, error) {
	candidates := stack.Detect(os.DirFS(p.root))
	if name := p.settings.Stack; name != "" {
		if prof, ok := stack.ByName(candidates, name); ok {
			return []stack.Profile{prof}, nil
		}
		return nil, errStackMissing(name, p.root)
	}
	return candidates, nil
}

func (a *App) printDoctor(r doctor.Report) {
	con := a.console()
	con.Title(con.T("doctor.title"))
	for _, c := range r {
		line := c.Name
		if c.Detail != "" {
			line += " · " + c.Detail
		}
		switch c.Status {
		case doctor.OK:
			con.OK(line)
		case doctor.Warn:
			con.Warn(line)
		default:
			con.Bad(line)
		}
		if c.Hint != "" {
			con.Detail("→ " + c.Hint)
		}
	}
	if len(r.Missing()) == 0 {
		con.Title(con.T("doctor.ready"))
	}
}

func firstErr(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// guardProbe checks the guard hook, unless the guard is off.
func (a *App) guardProbe(p project) func(string) ports.Readiness {
	if p.settings.GuardMode == guardhook.Off {
		return nil
	}
	return func(agent string) ports.Readiness {
		return guardhook.Readiness(fsys.OS{}, layout.Layout{Root: p.root}, agent)
	}
}
