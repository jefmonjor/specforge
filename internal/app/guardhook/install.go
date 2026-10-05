package guardhook

import (
	"encoding/json"
	"fmt"
	"strings"

	"specforge/internal/app/layout"
	"specforge/internal/ports"
)

// Settings is each agent's project settings file.
var Settings = map[string]string{"claude": ".claude/settings.json", "gemini": ".gemini/settings.json"}

// events are the pre-tool hook event of each agent.
var events = map[string]string{"claude": "PreToolUse", "gemini": "BeforeTool"}

// HookCommand is the command the agent runs before each shell command.
func HookCommand(binary, agent string) string { return binary + " guard --hook " + agent }

// Install adds the guard hook to the agent's project settings, keeping
// everything else in the file. It reports whether the file changed.
func Install(files ports.Files, lay layout.Layout, agent, binary string) (bool, error) {
	rel, ok := Settings[agent]
	if !ok {
		return false, fmt.Errorf("unknown agent %q", agent)
	}
	path := lay.Abs(rel)
	root := map[string]any{}
	if files.Exists(path) {
		data, err := files.ReadFile(path)
		if err != nil {
			return false, err
		}
		if strings.TrimSpace(string(data)) != "" {
			if err := json.Unmarshal(data, &root); err != nil {
				return false, fmt.Errorf("%s is not valid JSON: %w (fix it, then run setup again)", rel, err)
			}
		}
	}
	if hasHook(root, agent) {
		return false, nil
	}
	hooks, _ := root["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
	}
	event := events[agent]
	list, _ := hooks[event].([]any)
	hooks[event] = append(list, map[string]any{
		"matcher": shellTools[agent],
		"hooks":   []any{map[string]any{"type": "command", "command": HookCommand(binary, agent)}},
	})
	root["hooks"] = hooks
	data, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return false, err
	}
	return true, files.WriteFile(path, append(data, '\n'))
}

// Installed reports whether the agent's project settings run the guard.
func Installed(files ports.Files, lay layout.Layout, agent string) bool {
	data, err := files.ReadFile(lay.Abs(Settings[agent]))
	if err != nil {
		return false
	}
	root := map[string]any{}
	return json.Unmarshal(data, &root) == nil && hasHook(root, agent)
}

func hasHook(root map[string]any, agent string) bool {
	hooks, _ := root["hooks"].(map[string]any)
	list, _ := hooks[events[agent]].([]any)
	for _, entry := range list {
		e, _ := entry.(map[string]any)
		inner, _ := e["hooks"].([]any)
		for _, h := range inner {
			cmd, _ := h.(map[string]any)["command"].(string)
			if strings.Contains(cmd, "guard --hook "+agent) {
				return true
			}
		}
	}
	return false
}

// Readiness tells doctor whether the hook is installed.
func Readiness(files ports.Files, lay layout.Layout, agent string) ports.Readiness {
	rel := Settings[agent]
	if Installed(files, lay, agent) {
		return ports.Readiness{Ready: true, Detail: rel + " runs `specforge guard`"}
	}
	return ports.Readiness{Detail: "no guard hook in " + rel, Hint: "specforge setup installs it (guard.mode: off in specforge.yaml to go without)"}
}
