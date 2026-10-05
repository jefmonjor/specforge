package guardhook

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jefmonjor/specforge/v6/internal/adapters/fsys"
	"github.com/jefmonjor/specforge/v6/internal/app/layout"
)

func TestDecide(t *testing.T) {
	cases := []struct {
		cmd, mode string
		allow     []string
		canAsk    bool
		want      Verdict
	}{
		{"go test ./...", Block, nil, false, Allow},
		{"git reset --hard", Block, nil, true, Deny},
		{"git reset --hard", Confirm, nil, true, Ask},
		{"git reset --hard", Confirm, nil, false, Deny}, // inside the loop nobody can answer
		{"git reset --hard", Off, nil, false, Allow},
		{"rm -rf /", Off, nil, true, Deny}, // never allowed
		{"git push --force-with-lease origin claude/x", Block, []string{"git push --force-with-lease origin claude/*"}, false, Allow},
	}
	for _, c := range cases {
		if got := Decide(c.cmd, nil, c.mode, c.allow, c.canAsk); got.Verdict != c.want {
			t.Errorf("Decide(%q, %s, ask=%v) = %s, want %s", c.cmd, c.mode, c.canAsk, got.Verdict, c.want)
		}
	}
}

func TestHookProtocol(t *testing.T) {
	req, err := Command("claude", strings.NewReader(`{"tool_name":"Bash","cwd":"/repo","tool_input":{"command":"git clean -fdx"}}`))
	if err != nil || req.Command != "git clean -fdx" || req.Dir != filepath.FromSlash("/repo") {
		t.Fatalf("Command = %+v %v", req, err)
	}
	if req, _ := Command("claude", strings.NewReader(`{"tool_name":"Write","tool_input":{"file_path":"a"}}`)); req.Command != "" {
		t.Fatal("other tools are not shell commands")
	}
	req, _ = Command("gemini", strings.NewReader(`{"tool_name":"run_shell_command","cwd":"/repo","tool_input":{"command":"rm -rf x","dir_path":"web"}}`))
	if req.Command != "rm -rf x" || req.Dir != filepath.Join("/repo", "web") {
		t.Fatalf("gemini's shell tool: %+v", req)
	}
	if req, _ := Command("claude", strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"ls"}}`)); req.Dir != "" {
		t.Fatal("no directory when the agent sends none")
	}
	if _, err := Command("claude", strings.NewReader("not json")); err == nil {
		t.Fatal("unreadable input is an error")
	}

	var out, errOut bytes.Buffer
	deny := Decide("git reset --hard", nil, Block, nil, false)
	if code := Respond("claude", deny, &out, &errOut); code != 2 || !strings.Contains(errOut.String(), "git reset --hard discards uncommitted work") {
		t.Fatalf("deny: exit %d, stderr %q", code, errOut.String())
	}
	out.Reset()
	ask := Decide("git reset --hard", nil, Confirm, nil, true)
	if code := Respond("claude", ask, &out, &errOut); code != 0 || !strings.Contains(out.String(), `"permissionDecision":"ask"`) {
		t.Fatalf("ask: exit %d, stdout %q", code, out.String())
	}
	if code := Respond("gemini", ask, &out, &errOut); code != 2 {
		t.Fatal("gemini cannot ask: it blocks")
	}
	if code := Respond("claude", Decision{Verdict: Allow}, &out, &errOut); code != 0 {
		t.Fatal("allow exits 0")
	}
}

func TestSelfTest(t *testing.T) {
	if err := SelfTest(Block, nil); err != nil {
		t.Fatal(err)
	}
	if err := SelfTest(Block, []string{"git reset --hard*"}); err == nil {
		t.Fatal("an allow list that lets the probe through fails the self test")
	}
}

func TestInstallMergesAndIsIdempotent(t *testing.T) {
	root := t.TempDir()
	lay := layout.Layout{Root: root}
	settings := filepath.Join(root, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settings), 0o755); err != nil {
		t.Fatal(err)
	}
	existing := `{"permissions":{"allow":["Bash(go test:*)"]},"hooks":{"PreToolUse":[{"matcher":"Write","hooks":[{"type":"command","command":"fmt.sh"}]}]}}`
	if err := os.WriteFile(settings, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}
	files := fsys.OS{}
	if Installed(files, lay, "claude") {
		t.Fatal("not installed yet")
	}
	changed, err := Install(files, lay, "claude", "specforge")
	if err != nil || !changed {
		t.Fatalf("Install = %v %v", changed, err)
	}
	var got map[string]any
	data, _ := os.ReadFile(settings)
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"Bash(go test:*)"`) || !strings.Contains(string(data), `"fmt.sh"`) || !strings.Contains(string(data), `"specforge guard --hook claude"`) {
		t.Fatalf("the rest of the file is kept:\n%s", data)
	}
	if changed, err := Install(files, lay, "claude", "specforge"); changed || err != nil {
		t.Fatalf("a second install changes nothing: %v %v", changed, err)
	}
	if !Installed(files, lay, "claude") || !Readiness(files, lay, "claude").Ready {
		t.Fatal("installed")
	}
	if changed, err := Install(files, lay, "gemini", "/usr/local/bin/specforge"); !changed || err != nil || !Installed(files, lay, "gemini") {
		t.Fatalf("gemini: %v %v", changed, err)
	}
	if err := os.WriteFile(settings, []byte("{broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(files, lay, "claude", "specforge"); err == nil || !strings.Contains(err.Error(), "not valid JSON") {
		t.Fatalf("a broken settings file is never overwritten: %v", err)
	}
}

func TestTheHookReadsTheScriptsACommandRuns(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write("clean.sh", "#!/bin/sh\nrm -rf build\n")
	write("blob.bin", "\x00\x00rm -rf build")
	write("big.sh", strings.Repeat("echo hi\n", MaxScriptBytes/8+1)+"rm -rf build\n")
	read := Files(dir, func(p string) (io.ReadCloser, error) { return os.Open(p) })
	if d := Decide("./clean.sh", read, Block, nil, false); d.Verdict != Deny || !strings.Contains(d.Reason(), "run by `./clean.sh`") {
		t.Fatalf("a script that deletes is blocked: %+v", d)
	}
	if _, ok := read("blob.bin"); ok {
		t.Fatal("binary files are not read")
	}
	if _, ok := read("big.sh"); ok {
		t.Fatal("files over the limit are not read")
	}
	if d := Decide("./clean.sh", read, Block, []string{"./clean.sh"}, false); d.Verdict != Allow {
		t.Fatal("guard.allow lets a known script through")
	}
}
