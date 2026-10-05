// Package setup prepares a repository for SpecForge without touching its
// code: a project configuration, the rules every agent session reads, and a
// git-ignored working directory. Running it twice changes nothing.
package setup

import (
	"errors"
	"fmt"
	"io/fs"
	"strconv"
	"strings"

	"specforge/assets"
	"specforge/internal/app/guardhook"
	"specforge/internal/app/layout"
	"specforge/internal/config"
	"specforge/internal/domain/stack"
	"specforge/internal/ports"
)

// Markers delimit the block SpecForge owns inside CLAUDE.md or GEMINI.md.
// Everything outside them belongs to the developer and is never changed.
const (
	BeginMarker = "<!-- specforge:begin (managed by `specforge setup`; edits inside are overwritten) -->"
	EndMarker   = "<!-- specforge:end -->"
	beginPrefix = "<!-- specforge:begin"
)

// MemoryFile is the file each agent loads on its own at every session.
var MemoryFile = map[string]string{"claude": "CLAUDE.md", "gemini": "GEMINI.md"}

// Options configure a setup.
type Options struct {
	Layout   layout.Layout
	Language string
	// Agents receive the managed block; at least one.
	Agents []string
	// Stack is the detected or chosen stack; nil when there is none yet.
	Stack *stack.Profile
	// Legacy, when set, makes the project the rewrite of that legacy
	// repository (path as written in specforge.yaml) targeting JavaRelease.
	Legacy      string
	JavaRelease int
	// GuardBinary, when set, installs the destructive-command guard as each
	// agent's pre-tool hook, run as `<GuardBinary> guard --hook <agent>`.
	GuardBinary string
}

// Action is what happened to one file.
type Action string

const (
	Created   Action = "created"
	Updated   Action = "updated"
	Unchanged Action = "unchanged"
)

// Change records one file setup looked at.
type Change struct {
	Path   string // project-relative
	Action Action
}

// Run applies the setup and reports every file it looked at.
func Run(files ports.Files, o Options) ([]Change, error) {
	if len(o.Agents) == 0 {
		return nil, errors.New("setup needs at least one agent to write the rules for")
	}
	block, err := Block(o.Language, o.Stack)
	if err != nil {
		return nil, err
	}

	var changes []Change
	record := func(rel string, a Action, err error) error {
		if err != nil {
			return fmt.Errorf("%s: %w", rel, err)
		}
		changes = append(changes, Change{Path: rel, Action: a})
		return nil
	}

	a, err := writeProjectConfig(files, o)
	if err := record(config.ProjectFile, a, err); err != nil {
		return changes, err
	}
	for _, agent := range o.Agents {
		name, ok := MemoryFile[agent]
		if !ok {
			return changes, fmt.Errorf("unknown agent %q", agent)
		}
		a, err := upsertBlock(files, o.Layout.Abs(name), block)
		if err := record(name, a, err); err != nil {
			return changes, err
		}
	}
	if o.GuardBinary != "" {
		for _, agent := range o.Agents {
			changed, err := guardhook.Install(files, o.Layout, agent, o.GuardBinary)
			a := Unchanged
			if changed {
				a = Updated
			}
			if err := record(guardhook.Settings[agent], a, err); err != nil {
				return changes, err
			}
		}
	}
	a, err = ensureLine(files, o.Layout.Abs(".gitignore"), ".specforge/")
	return changes, record(".gitignore", a, err)
}

// Block renders the managed block: the working rules and, when the stack is
// known, its coding standard.
func Block(lang string, p *stack.Profile) (string, error) {
	if lang != "es" {
		lang = "en"
	}
	rules, err := fs.ReadFile(assets.FS, "agent/"+lang+"/rules.md")
	if err != nil {
		return "", err
	}
	parts := []string{BeginMarker, strings.TrimSpace(string(rules))}
	if p != nil && p.Standard != "" {
		std, err := fs.ReadFile(assets.FS, "standards/"+lang+"/"+p.Standard+".md")
		if err != nil {
			return "", err
		}
		parts = append(parts, strings.TrimSpace(string(std)))
	}
	parts = append(parts, EndMarker)
	return strings.Join(parts, "\n\n") + "\n", nil
}

// upsertBlock replaces the managed block in path, or appends it.
func upsertBlock(files ports.Files, path, block string) (Action, error) {
	if !files.Exists(path) {
		return Created, files.WriteFile(path, []byte(block))
	}
	data, err := files.ReadFile(path)
	if err != nil {
		return "", err
	}
	current := strings.ReplaceAll(string(data), "\r\n", "\n")
	var next string
	start := strings.Index(current, beginPrefix)
	end := strings.Index(current, EndMarker)
	switch {
	case start >= 0 && end > start:
		end += len(EndMarker)
		if end < len(current) && current[end] == '\n' {
			end++
		}
		next = current[:start] + block + current[end:]
	case start >= 0 || end >= 0:
		return "", errors.New("the SpecForge block is damaged (one marker without the other): fix or remove it and run setup again")
	default:
		switch {
		case strings.TrimSpace(current) == "":
			next = block
		case strings.HasSuffix(current, "\n"):
			next = current + "\n" + block
		default:
			next = current + "\n\n" + block
		}
	}
	if next == current {
		return Unchanged, nil
	}
	return Updated, files.WriteFile(path, []byte(next))
}

// ensureLine appends line to a line-based file such as .gitignore.
func ensureLine(files ports.Files, path, line string) (Action, error) {
	if !files.Exists(path) {
		return Created, files.WriteFile(path, []byte(line+"\n"))
	}
	data, err := files.ReadFile(path)
	if err != nil {
		return "", err
	}
	for _, l := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
		l = strings.TrimSpace(l)
		if l == line || l == strings.TrimSuffix(line, "/") || l == "/"+line {
			return Unchanged, nil
		}
	}
	prefix := ""
	if len(data) > 0 && !strings.HasSuffix(string(data), "\n") {
		prefix = "\n"
	}
	return Updated, files.AppendFile(path, []byte(prefix+line+"\n"))
}

// writeProjectConfig creates specforge.yaml; an existing one is the team's
// and is left alone.
func writeProjectConfig(files ports.Files, o Options) (Action, error) {
	path := o.Layout.Abs(config.ProjectFile)
	if files.Exists(path) {
		return Unchanged, nil
	}
	return Created, files.WriteFile(path, []byte(ProjectConfig(o.Language, o.Stack, o.Legacy, o.JavaRelease)))
}

// ProjectConfig renders a commented specforge.yaml with the defaults and,
// for a rewrite, the migration section.
func ProjectConfig(lang string, p *stack.Profile, legacy string, javaRelease int) string {
	stackLine := "# stack: go            # go | maven | gradle | node | python (detected when unset)"
	if p != nil {
		stackLine = "stack: " + string(p.Kind)
	}
	return strings.Join([]string{
		"# SpecForge project configuration. Commit it: it is shared by the team.",
		"# Every key is optional; the commented values are the defaults.",
		"language: " + lang + "  # es | en: prompts, templates and messages",
		stackLine,
		"# agent: claude         # claude | gemini; usually each developer's choice (`specforge init`)",
		"# max_attempts: 3       # GREEN attempts per scenario",
		"# review: scenario      # scenario: you review each finished scenario | risk: only medium and high | off",
		"# commit: true          # one commit per finished scenario",
		"# model: \"\"             # the agent's model; models.<phase> overrides it per phase:",
		"# models:               # interview plan legacy red green refactor review review2 refute verify audit e2e",
		"#   review: claude-opus-5-5",
		"# timeouts:",
		"#   agent: 20m",
		"#   tests: 10m",
		"# quality:",
		"#   strict: false                # true: a gate that cannot run blocks, and every change is high risk",
		"#   max_duplication_percent: 0   # jscpd",
		"#   min_mutation_score: 80       # Stryker",
		"#   mutation_from: medium        # lowest risk tier that runs the mutation gate",
		"# risk:                          # how each scenario's change is classified",
		"#   max_lines: 400               # larger changes are high risk",
		"#   floor: passive               # the lowest tier any change gets",
		"#   high_paths: [...]            # regular expressions; default: auth, security, tokens, payments, migrations, CI, build files…",
		"# lenses: auto                   # review lenses after REFACTOR: auto (by risk) | off | [risk, reliability, readability, resilience]",
		"# blind_review: false            # high risk: run the lenses twice, independently",
		"# verify: high                   # independent verifier: high | always | feature | off",
		"# verify_max_mb: 500             # largest project the verifier copies",
		"# plan:",
		"#   surfaces: ask                # a file outside the approved plan: ask | strict | off",
		"# delivery:",
		"#   budget_lines: 400            # deliver proposes stacked slices above it",
		"# loop:",
		"#   parallel: 1                  # scenarios with disjoint plan surfaces run side by side",
		"# guard:                         # the destructive-command guard (the agents' pre-tool hook)",
		"#   mode: block                  # block | confirm | off",
		"#   allow: []                    # command patterns let through, * matches anything",
		migrationConfig(legacy, javaRelease),
	}, "\n")
}

func migrationConfig(legacy string, javaRelease int) string {
	if legacy == "" {
		return strings.Join([]string{
			"# migration:                     # rewriting a legacy system (see `specforge legacy`)",
			"#   legacy: ../old-system        # read-only reference for the agent",
			"#   java_release: 21             # the build must declare it",
			"#   forbidden_imports: [javax.servlet, javax.persistence, org.apache.log4j]",
			"",
		}, "\n")
	}
	lines := []string{
		"migration:",
		"  legacy: " + strconv.Quote(legacy) + "  # read-only reference for the agent",
	}
	if javaRelease > 0 {
		lines = append(lines,
			"  java_release: "+strconv.Itoa(javaRelease)+"  # the build must declare it",
			"  # forbidden_imports: [...]   # default: the javax.* that Jakarta renamed, Log4j 1, JUnit 3, Vector, Hashtable")
	}
	return strings.Join(append(lines, ""), "\n")
}
