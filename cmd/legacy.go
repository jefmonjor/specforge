package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"specforge/internal/adapters/fsys"
	"specforge/internal/adapters/workspace"
	"specforge/internal/app/clarify"
	"specforge/internal/app/migrate"
	"specforge/internal/app/specs"
	"specforge/internal/config"
	"specforge/internal/domain/spec"
	"specforge/internal/ui"
)

func (a *App) legacyCommand() *cobra.Command {
	c := &cobra.Command{
		Use:   "legacy",
		Short: "Rewrite a legacy system: inventory, capability map, specifications",
		Long: `legacy prepares the rewrite of a legacy system (Java 6 to 21, for example)
from a new project next to it. The legacy repository is read-only: the agent
reads it and SpecForge checks that no file there changed.

  legacy scan      measure the legacy code (no agent): docs/legacy/INVENTORY.md
  legacy map       the agent maps its business capabilities: docs/legacy/CAPABILITIES.md
  spec from-legacy one specification per capability, with the legacy sources cited

Every legacy source the agent cites is opened by SpecForge: a file that does
not exist or a line past its end sends the document back. Set the path once
in specforge.yaml:

  migration:
    legacy: ../payroll-legacy
    java_release: 21`,
	}
	c.AddCommand(a.legacyScanCommand(), a.legacyMapCommand())
	return c
}

func (a *App) legacyScanCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "scan [legacy-path]",
		Short:   "Measure the legacy code and write docs/legacy/INVENTORY.md",
		Example: "  specforge legacy scan ../payroll-legacy",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			p, err := a.openProject(config.Overrides{}, false)
			if err != nil {
				return err
			}
			o, err := a.migrateOptions(p, argOrEmpty(args))
			if err != nil {
				return err
			}
			inv, path, err := migrate.Scan(fsys.OS{}, o)
			if err != nil {
				return err
			}
			if ok, err := a.emit(inv); ok {
				return err
			}
			con := a.console()
			con.OK(con.T("legacy.scanned", p.layout.Rel(path)))
			release := inv.JavaRelease
			if release == "" {
				release = "?"
			}
			var found []string
			for _, f := range inv.Frameworks {
				found = append(found, f.Name)
			}
			con.Info(con.T("legacy.summary", inv.Build, release, inv.SourceFiles, inv.TestFiles, inv.Lines))
			if len(found) > 0 {
				con.Info(con.T("legacy.frameworks", strings.Join(found, ", ")))
			}
			con.Info(con.T("legacy.next.scan"))
			con.Data(p.layout.Rel(path))
			return nil
		},
	}
}

func (a *App) legacyMapCommand() *cobra.Command {
	var o config.Overrides
	c := &cobra.Command{
		Use:   "map [legacy-path]",
		Short: "Let the agent map the business capabilities of the legacy code",
		Long: `map scans the legacy code, then asks the agent to read it and list what
it does for the business, one capability per section, each rule with the
source that implements it. The result, docs/legacy/CAPABILITIES.md, is the
list of specifications to write, in the order to migrate them.`,
		Example: "  specforge legacy map ../payroll-legacy",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := a.openProject(o, true)
			if err != nil {
				return err
			}
			mo, err := a.migrateOptions(p, argOrEmpty(args))
			if err != nil {
				return err
			}
			deps, events, err := a.migrateDeps(p)
			if err != nil {
				return err
			}
			con := a.console()
			con.Title(con.T("legacy.map.title", mo.Legacy))
			path, err := migrate.Map(cmd.Context(), deps, mo)
			events.Done()
			if err != nil {
				return err
			}
			con.OK(con.T("legacy.mapped", p.layout.Rel(path)))
			con.Info(con.T("legacy.next.map"))
			con.Data(p.layout.Rel(path))
			return nil
		},
	}
	c.Flags().StringVar(&o.Agent, "agent", "", "claude | gemini (default: configured)")
	c.Flags().StringVar(&o.Model, "model", "", "model passed to the agent")
	return c
}

func (a *App) specFromLegacyCommand() *cobra.Command {
	var o config.Overrides
	var legacyPath string
	c := &cobra.Command{
		Use:   "from-legacy <capability>",
		Short: "Draft a specification from what the legacy code really does",
		Long: `from-legacy creates the next specification for one capability of the
legacy system and asks the agent to fill it in from the legacy code: the
rules as invariants, the behaviour as scenarios with the real values, the
real error messages, and a last section citing the source of each rule.
What the code does that nobody can explain goes to the open questions:
answer them with spec clarify (or edit them), then approve as usual.

Run it again with the same capability to continue a draft (after a pending
question, for example) instead of creating another specification.`,
		Example: `  specforge spec from-legacy "Pay an employee"
  specforge spec from-legacy --legacy ../payroll-legacy "Pay an employee"`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := a.openProject(o, true)
			if err != nil {
				return err
			}
			mo, err := a.migrateOptions(p, legacyPath)
			if err != nil {
				return err
			}
			capability := strings.Join(strings.Fields(strings.Join(args, " ")), " ")
			svc := p.specs(a)
			entry, reused, err := draftFor(svc, capability)
			if err != nil {
				return err
			}
			deps, events, err := a.migrateDeps(p)
			if err != nil {
				return err
			}
			con := a.console()
			if !reused {
				con.OK(con.T("spec.created", entry.Rel))
			}
			con.Title(con.T("legacy.spec.title", capability))
			err = migrate.Draft(cmd.Context(), deps, mo, entry.Path, capability)
			events.Done()
			if err != nil {
				return err
			}
			text, err := os.ReadFile(entry.Path)
			if err != nil {
				return err
			}
			open := spec.OpenQuestions(string(text))
			con.OK(con.T("legacy.spec.written", entry.Rel))
			if len(open) > 0 {
				con.Info(con.T("legacy.spec.open", len(open), entry.ID))
			} else {
				con.Info(con.T("legacy.spec.next", entry.ID))
			}
			con.Data(entry.Rel)
			return nil
		},
	}
	c.Flags().StringVar(&legacyPath, "legacy", "", "legacy repository (default: migration.legacy in specforge.yaml)")
	c.Flags().StringVar(&o.Agent, "agent", "", "claude | gemini (default: configured)")
	c.Flags().StringVar(&o.Model, "model", "", "model passed to the agent")
	return c
}

// draftFor returns the draft specification of a capability, creating it
// the first time.
func draftFor(svc specs.Service, capability string) (specs.Entry, bool, error) {
	all, err := svc.List()
	if err != nil {
		return specs.Entry{}, false, err
	}
	for _, e := range all {
		if strings.EqualFold(e.Title, capability) {
			if e.State != specs.StateDraft {
				return e, true, fmt.Errorf("%s is already approved: edit it and approve it again instead", e.Rel)
			}
			return e, true, nil
		}
	}
	e, err := svc.New(capability)
	return e, false, err
}

func (a *App) migrateOptions(p project, arg string) (migrate.Options, error) {
	dir := p.legacyDir(arg)
	if dir == "" {
		return migrate.Options{}, fmt.Errorf("%w: pass the legacy repository's path or set migration.legacy in specforge.yaml", migrate.ErrNoLegacy)
	}
	if err := migrate.CheckLegacy(dir); err != nil {
		return migrate.Options{}, err
	}
	m := p.settings.Migration
	return migrate.Options{
		Root: p.root, Legacy: dir, Language: p.settings.Language, Model: p.settings.Model,
		AgentTimeout: p.settings.AgentTimeout, MaxAttempts: p.settings.MaxAttempts,
		JavaRelease: m.JavaRelease, ForbiddenImports: m.ForbiddenImports,
	}, nil
}

func (a *App) migrateDeps(p project) (migrate.Deps, *ui.PlanEvents, error) {
	proc := a.NewProcess(a.log)
	ag, err := a.NewAgent(p.settings.Agent, proc, a.log)
	if err != nil {
		return migrate.Deps{}, nil, err
	}
	files := fsys.OS{}
	events := &ui.PlanEvents{C: a.console(), Agent: ag.Name(), Phase: "LEGACY"}
	return migrate.Deps{
		Agent:     ag,
		Workspace: workspace.New(proc),
		Files:     files,
		Asker:     &clarify.Asker{Prompter: a.prompter(), Files: files, Now: a.Now, Lang: p.settings.Language},
		Events:    events,
		Log:       a.log,
	}, events, nil
}

// rewriteLegacy returns the legacy repository configured for a rewrite,
// or "" when the project is not one. A configured path that does not exist
// is an error: the agent would work without its reference.
func (a *App) rewriteLegacy(p project) (string, error) {
	dir := p.legacyDir("")
	if dir == "" {
		return "", nil
	}
	return dir, migrate.CheckLegacy(dir)
}
