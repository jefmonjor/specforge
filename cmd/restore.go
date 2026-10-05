package cmd

import (
	"time"

	"github.com/spf13/cobra"

	"github.com/jefmonjor/specforge/v6/internal/adapters/vcs"
	"github.com/jefmonjor/specforge/v6/internal/app/restore"
	"github.com/jefmonjor/specforge/v6/internal/config"
)

func (a *App) restoreCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "restore [checkpoint|latest]",
		Short: "Bring back the work an agent destroyed, from a loop checkpoint",
		Long: `Before every agent turn the loop saves a checkpoint of your working tree:
uncommitted and untracked files included, ignored ones (node_modules, .env)
not. Checkpoints live in refs/specforge/checkpoints/: no branch, commit,
index or push carries them, and the newest 50 are kept.

Without an argument, restore lists them. With one, it brings back the files
of that checkpoint as they were: what was changed or deleted since comes
back, and nothing written since is deleted. Your working tree just before
is saved as a checkpoint first, so a restore can be undone the same way.

This is the way back from whatever the guard could not see: a program, a
compiled binary, a script the agent wrote outside the project.`,
		Example: "  specforge restore\n  specforge restore latest\n  specforge restore 20261005T142233.123456789Z",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := a.openProject(config.Overrides{}, false)
			if err != nil {
				return err
			}
			cps := vcs.New(a.NewProcess(a.log))
			con := a.console()
			if len(args) == 0 {
				list, err := cps.List(cmd.Context(), p.root)
				if err != nil {
					return err
				}
				if ok, err := a.emit(list); ok {
					return err
				}
				con.Title(con.T("restore.title"))
				if len(list) == 0 {
					con.Info(con.T("restore.none"))
					return nil
				}
				for _, c := range list {
					con.Info(c.ID + " · " + c.At.Local().Format(time.DateTime) + " · " + c.Label)
				}
				con.Info(con.T("restore.hint"))
				return nil
			}
			res, err := restore.Run(cmd.Context(), cps, p.root, args[0])
			if err != nil {
				return err
			}
			if ok, err := a.emit(res); ok {
				return err
			}
			con.OK(con.T("restore.done", res.Restored.ID, res.Restored.Label))
			con.Info(con.T("restore.undo", res.Before.ID))
			return nil
		},
	}
}
