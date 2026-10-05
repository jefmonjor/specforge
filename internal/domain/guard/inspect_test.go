package guard

import (
	"strings"
	"testing"
)

// files is a fake file system for the reader.
type files map[string]string

func (f files) read(p string) (string, bool) {
	s, ok := f[strings.TrimPrefix(p, "./")]
	return s, ok
}

func TestInspectReadsWhatTheCommandRuns(t *testing.T) {
	fs := files{
		"clean.sh":       "#!/bin/sh\necho cleaning\nrm -rf build\n",
		"safe.sh":        "#!/bin/sh\ngo test ./...\n",
		"nested.sh":      "sh ./clean.sh\n",
		"loop.sh":        "sh ./loop.sh\n",
		"wipe.py":        "#!/usr/bin/env python3\nimport shutil\nshutil.rmtree('data')\n",
		"tool":           "#!/usr/bin/env python3\nimport os\nos.system('git reset --hard')\n",
		"build.py":       "print('building')\n",
		"run.js":         "const { execSync } = require('child_process')\nexecSync(`git push --force origin main`)\n",
		"del.js":         "require('fs').rmSync('dist', { recursive: true, force: true })\n",
		"sub.py":         "import subprocess\nsubprocess.run(['git', 'clean', '-fdx'])\n",
		"main.go":        "package main\nimport \"os\"\nfunc main() { os.RemoveAll(\"out\") }\n",
		"db.rb":          "DB.run('DELETE FROM users')\n",
		"package.json":   `{"scripts": {"build": "tsc", "clean": "rimraf dist", "prereset": "git reset --hard", "reset": "echo"}}`,
		"Makefile":       "all: build\n\nbuild:\n\tgo build ./...\n\nclean: tidy\n\t@rm -rf bin\n\ntidy:\n\tgo mod tidy\n",
		"scripts/nuke":   "git clean -fdx\n",
		"bin/binary.exe": "",
	}
	cases := []struct {
		cmd  string
		want Kind // "" when nothing is destructive
	}{
		// Scripts, run however.
		{"sh clean.sh", FS},
		{"bash -x ./clean.sh", FS},
		{"./clean.sh", FS},
		{"source clean.sh", FS},
		{". ./clean.sh", FS},
		{"sh nested.sh", FS},
		{"sh loop.sh", ""}, // a script that runs itself ends
		{"sh safe.sh", ""},
		{"scripts/nuke", Git},
		{"sh missing.sh", ""},
		// A shebang names the language.
		{"./wipe.py", FS},
		{"./tool", Git},
		// Files given to an interpreter.
		{"python3 wipe.py", FS},
		{"python build.py", ""},
		{"python -m pytest", ""},
		{"node run.js", Git},
		{"node del.js", FS},
		{"python sub.py", Git},
		{"go run main.go", FS},
		{"ruby db.rb", DB},
		{"bun run.js", Git},
		// Code given inline.
		{`python -c "import shutil; shutil.rmtree('/tmp/x')"`, FS},
		{`python3 -c 'import os; os.system("rm -rf build")'`, FS},
		{`node -e "require('fs').rmSync('x', {recursive: true})"`, FS},
		{`node -e "console.log(1)"`, ""},
		{`ruby -e "FileUtils.rm_rf('x')"`, FS},
		{`perl -e 'use File::Path; remove_tree("x")'`, FS},
		{`python -c "print('hello')"`, ""},
		// package.json scripts and Makefile recipes.
		{"npm run clean", FS},
		{"npm run build", ""},
		{"pnpm clean", FS},
		{"yarn clean", FS},
		{"bun run clean", FS},
		{"npm run reset", Git}, // its pre-script
		{"npm install", ""},
		{"make clean", FS},
		{"make", ""},
		{"make -j4 build", ""},
	}
	for _, c := range cases {
		got := Inspect(c.cmd, fs.read)
		switch {
		case c.want == "" && len(got) > 0:
			t.Errorf("%q: want nothing, got %+v", c.cmd, got)
		case c.want != "" && !hasKind(got, c.want):
			t.Errorf("%q: want %s, got %+v", c.cmd, c.want, got)
		}
	}
}

func hasKind(ms []Match, k Kind) bool {
	for _, m := range ms {
		if m.Kind == k {
			return true
		}
	}
	return false
}

func TestInspectSaysWhatRanIt(t *testing.T) {
	got := Inspect("sh clean.sh", files{"clean.sh": "rm -rf build\n"}.read)
	if len(got) != 1 || !strings.Contains(got[0].Reason, "run by `sh clean.sh`") || got[0].Segment != "sh clean.sh" {
		t.Fatalf("%+v", got)
	}
	// The allow list matches what the agent typed.
	if !Allowed(got[0], []string{"sh clean.sh"}) {
		t.Fatal("an allowed script stays allowed")
	}
}

func TestInlineCodeNeedsNoReader(t *testing.T) {
	if got := Recognize(`python -c "import shutil; shutil.rmtree('x')"`); len(got) == 0 {
		t.Fatal("inline code is read without files")
	}
	if got := Recognize("sh clean.sh"); len(got) != 0 {
		t.Fatal("without a reader, files are not read")
	}
}
