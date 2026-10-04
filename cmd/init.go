package cmd

import (
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"specforge/internal/adapters/agent"
	"specforge/internal/adapters/process"
	"specforge/internal/config"
	"specforge/internal/ports"
)

func (a *App) initCommand() *cobra.Command {
	var u config.User
	c := &cobra.Command{
		Use:   "init",
		Short: "Choose your coding agent and language (once per machine)",
		Long: `init saves your personal choices: the coding agent SpecForge drives and the
language of prompts, templates and messages. Nothing else: the agent signs in
on its own, and SpecForge never asks for an API key or edits your PATH.

Flags answer the questions up front; without them init asks.`,
		Example: "  specforge init\n  specforge init --agent claude --language es",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			dirs, err := a.UserDirs()
			if err != nil {
				return err
			}
			current, err := config.LoadUser(dirs)
			if err != nil {
				return err
			}
			if current.Language != "" {
				a.lang = current.Language
			}
			ask := func(flag, key, def string, options []string) (string, error) {
				if v := strings.ToLower(strings.TrimSpace(flag)); v != "" {
					if !slices.Contains(options, v) {
						return "", fmt.Errorf("%q is not one of: %s", v, strings.Join(options, ", "))
					}
					return v, nil
				}
				if !a.canAsk() {
					if def != "" {
						return def, nil
					}
					return "", fmt.Errorf("%w: pass --%s", ports.ErrNonInteractive, key)
				}
				answer, err := a.prompter().Ask(cmd.Context(), ports.Question{Text: a.console().T("init.ask." + key), Options: options})
				if err != nil {
					return "", err
				}
				if !slices.Contains(options, answer) {
					return "", fmt.Errorf("%q is not one of: %s", answer, strings.Join(options, ", "))
				}
				return answer, nil
			}

			agentName, err := ask(u.Agent, "agent", current.Agent, config.Agents)
			if err != nil {
				return err
			}
			lang, err := ask(u.Language, "language", first(current.Language, config.DefaultLanguage), config.Languages)
			if err != nil {
				return err
			}
			next := config.User{Agent: agentName, Language: lang, Model: first(u.Model, current.Model)}
			if err := config.SaveUser(dirs, next); err != nil {
				return err
			}
			a.lang = lang
			con := a.console()
			con.OK(con.T("init.saved", dirs.UserFile()))
			if !process.Available(agent.Flavors[agentName].Binary) {
				con.Warn(con.T("init.missing", agent.Flavors[agentName].Binary))
			}
			con.Info(con.T("init.next"))
			return nil
		},
	}
	c.Flags().StringVar(&u.Agent, "agent", "", "claude | gemini")
	c.Flags().StringVar(&u.Language, "language", "", "es | en")
	c.Flags().StringVar(&u.Model, "model", "", "model passed to the agent (empty: the agent's default)")
	return c
}

func first(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
