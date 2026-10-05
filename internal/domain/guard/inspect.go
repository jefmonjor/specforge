package guard

import (
	"encoding/json"
	"path"
	"regexp"
	"slices"
	"strings"
)

// Reader returns the text of a file a command runs (a script, a
// package.json, a Makefile), its path as the command wrote it; false when
// it cannot be read or is not text.
type Reader func(path string) (string, bool)

// maxDepth bounds how deep a script that runs another script is followed.
const maxDepth = 4

// Inspect is Recognize that also reads what the command runs: the scripts
// it executes (shell or any interpreter with a shebang), the file an
// interpreter is given, the package.json script or the Makefile target it
// starts. Code given inline to an interpreter (python -c, node -e) is
// inspected with or without a reader. What a program does that is not
// written in these files (what a script imports, a compiled binary) is
// out of reach: the loop's checkpoints are the way back from that.
func Inspect(line string, read Reader) []Match {
	in := inspector{read: read, seen: map[string]bool{}}
	return dedupe(in.line(line, 0))
}

type inspector struct {
	read Reader
	seen map[string]bool
}

func (in *inspector) line(line string, depth int) []Match {
	var out []Match
	for _, seg := range segments(line) {
		out = append(out, in.command(seg, words(seg), depth)...)
	}
	return append(out, sql(line)...)
}

// command recognises one simple command, then what it runs.
func (in *inspector) command(seg string, ws []string, depth int) []Match {
	ws = unwrap(ws)
	if len(ws) == 0 {
		return nil
	}
	if inner, ok := shellC(ws); ok {
		return in.line(inner, depth)
	}
	out := simple(seg, ws)
	name := path.Base(ws[0])
	if code, ok := inlineCode(name, ws[1:]); ok {
		return append(out, in.code(seg, code)...)
	}
	if depth >= maxDepth || in.read == nil {
		return out
	}
	for _, run := range runs(name, ws) {
		out = append(out, in.file(seg, run, depth+1)...)
	}
	return out
}

// target is something a command runs from a file.
type target struct {
	file string
	// kind: "script" (shell, or the interpreter its shebang names),
	// "code" (a file an interpreter is given), "npm" (a package.json
	// script) or "make" (Makefile targets).
	kind  string
	names []string
}

// runs lists what a command runs from files.
func runs(name string, ws []string) []target {
	args := ws[1:]
	operands := slices.DeleteFunc(slices.Clone(args), func(w string) bool { return strings.HasPrefix(w, "-") })
	switch {
	case slices.Contains(shells, name) || name == "source" || name == ".":
		if len(operands) > 0 {
			return []target{{file: operands[0], kind: "script"}}
		}
	case name == "go" || name == "deno" || name == "bun":
		// go run x.go, deno run x.ts, bun x.ts, bun run x.ts
		if len(operands) > 0 && operands[0] == "run" {
			operands = operands[1:]
		}
		if len(operands) > 0 && codeFile(operands[0]) {
			return []target{{file: operands[0], kind: "code"}}
		}
		if name == "bun" {
			return npmTarget(name, args)
		}
	case interpreters[name] != nil:
		if len(operands) > 0 && !slices.Contains(args, "-m") {
			return []target{{file: operands[0], kind: "code"}}
		}
	case name == "npm" || name == "pnpm" || name == "yarn":
		return npmTarget(name, args)
	case name == "make" || name == "gmake":
		return []target{{file: "Makefile", kind: "make", names: makeTargets(args)}}
	case strings.Contains(ws[0], "/"):
		return []target{{file: ws[0], kind: "script"}} // ./build.sh, scripts/x
	}
	return nil
}

func npmTarget(manager string, args []string) []target {
	if script := packageScript(manager, args); script != "" {
		return []target{{file: "package.json", kind: "npm", names: []string{"pre" + script, script, "post" + script}}}
	}
	return nil
}

// codeFile reports a path with a program's extension.
func codeFile(p string) bool {
	switch path.Ext(p) {
	case ".go", ".js", ".mjs", ".cjs", ".ts", ".mts", ".tsx", ".jsx", ".py", ".rb", ".pl", ".php":
		return true
	}
	return false
}

// file inspects what a command runs from a file.
func (in *inspector) file(seg string, t target, depth int) []Match {
	key := t.kind + ":" + t.file + ":" + strings.Join(t.names, ",")
	if in.seen[key] {
		return nil
	}
	in.seen[key] = true
	text, ok := in.read(t.file)
	if t.kind == "make" && !ok {
		for _, alt := range []string{"makefile", "GNUmakefile"} {
			if text, ok = in.read(alt); ok {
				break
			}
		}
	}
	if !ok {
		return nil
	}
	var out []Match
	switch t.kind {
	case "script":
		if lang := shebang(text); lang != "" && !slices.Contains(shells, lang) {
			out = in.code(seg, text)
		} else {
			out = in.line(text, depth)
		}
	case "code":
		out = in.code(seg, text)
	case "npm":
		for _, s := range npmScripts(text, t.names) {
			out = append(out, in.line(s, depth)...)
		}
	case "make":
		out = in.line(makeRecipes(text, t.names), depth)
	}
	for i := range out {
		out[i].Reason += " (run by `" + strings.TrimSpace(seg) + "`)"
		out[i].Segment = seg
	}
	return out
}

var shells = []string{"sh", "bash", "zsh", "dash", "ksh"}

// interpreters map each interpreter to its options that take inline code.
var interpreters = map[string][]string{
	"python": {"-c"}, "python3": {"-c"}, "python2": {"-c"}, "pypy": {"-c"}, "pypy3": {"-c"},
	"node": {"-e", "--eval", "-p", "--print"}, "nodejs": {"-e", "--eval", "-p", "--print"},
	"bun": {"-e", "--eval"}, "deno": {"eval"},
	"ruby": {"-e"}, "perl": {"-e", "-E"}, "php": {"-r"},
}

// inlineCode returns the code given on the command line to an interpreter.
func inlineCode(name string, args []string) (string, bool) {
	flags := interpreters[name]
	for i := 0; i < len(args)-1; i++ {
		if slices.Contains(flags, args[i]) {
			return args[i+1], true
		}
	}
	return "", false
}

// destructiveCalls are library calls that delete a directory tree.
var destructiveCalls = []struct {
	re     *regexp.Regexp
	reason string
}{
	{regexp.MustCompile(`\bshutil\.rmtree\s*\(`), "shutil.rmtree deletes a directory tree"},
	{regexp.MustCompile(`\bos\.removedirs\s*\(`), "os.removedirs deletes directories"},
	{regexp.MustCompile(`\b(rmSync|rm|rmdirSync|rmdir|promises\.rm)\s*\([^)]*recursive\s*:\s*true`), "a recursive fs.rm deletes a directory tree"},
	{regexp.MustCompile(`\b(rimraf|removeSync|emptyDirSync|emptyDir)\s*[.(]`), "rimraf / fs-extra remove deletes a directory tree"},
	{regexp.MustCompile(`\bDeno\.remove\s*\([^)]*recursive\s*:\s*true`), "Deno.remove with recursive deletes a directory tree"},
	{regexp.MustCompile(`\bFileUtils\.(rm_rf|rm_r|remove_dir|remove_entry|rmtree)\b`), "FileUtils deletes a directory tree"},
	{regexp.MustCompile(`\b(remove_tree|rmtree)\s*\(`), "File::Path deletes a directory tree"},
	{regexp.MustCompile(`\bos\.RemoveAll\s*\(`), "os.RemoveAll deletes a directory tree"},
}

var (
	// stringLiteral matches a quoted string on one line.
	stringLiteral = regexp.MustCompile(`"((?:[^"\\\n]|\\.)*)"|'((?:[^'\\\n]|\\.)*)'|` + "`([^`]*)`")
	// stringList matches a list of string literals: ["rm", "-rf", "x"].
	stringList = regexp.MustCompile(`\[\s*(?:"[^"\n]*"|'[^'\n]*')(?:\s*,\s*(?:"[^"\n]*"|'[^'\n]*'))*\s*,?\s*\]`)
)

// code inspects program text: destructive library calls, and shell
// commands or SQL written in its strings (os.system("..."),
// subprocess.run(["git", "reset", "--hard"]), execSync(`...`)).
func (in *inspector) code(seg, text string) []Match {
	var out []Match
	add := func(m Match) {
		m.Segment = seg
		out = append(out, m)
	}
	for _, c := range destructiveCalls {
		if c.re.MatchString(text) {
			add(Match{Kind: FS, Reason: c.reason})
		}
	}
	var shellish []string
	for _, l := range stringList.FindAllString(text, -1) {
		var parts []string
		for _, m := range stringLiteral.FindAllStringSubmatch(l, -1) {
			parts = append(parts, quoteIfNeeded(m[1]+m[2]+m[3]))
		}
		shellish = append(shellish, strings.Join(parts, " "))
	}
	for _, m := range stringLiteral.FindAllStringSubmatch(text, -1) {
		shellish = append(shellish, m[1]+m[2]+m[3])
	}
	for _, s := range shellish {
		for _, m := range Recognize(s) {
			add(m)
		}
	}
	return out
}

func quoteIfNeeded(s string) string {
	if strings.ContainsAny(s, " \t") {
		return "'" + s + "'"
	}
	return s
}

// shebang returns the interpreter a script's first line names.
func shebang(text string) string {
	first, _, _ := strings.Cut(text, "\n")
	ws := strings.Fields(strings.TrimPrefix(first, "#!"))
	if !strings.HasPrefix(first, "#!") || len(ws) == 0 {
		return ""
	}
	name := path.Base(ws[0])
	if name == "env" && len(ws) > 1 {
		name = path.Base(ws[len(ws)-1])
		if strings.HasPrefix(ws[1], "-") && len(ws) > 2 {
			name = path.Base(ws[2])
		}
	}
	return name
}

// packageScript names the package.json script a package manager runs.
func packageScript(manager string, args []string) string {
	a := slices.DeleteFunc(slices.Clone(args), func(w string) bool { return strings.HasPrefix(w, "-") })
	switch {
	case len(a) == 0:
		return ""
	case a[0] == "run" || a[0] == "run-script":
		if len(a) > 1 {
			return a[1]
		}
		return ""
	case slices.Contains([]string{"test", "start", "stop", "restart"}, a[0]):
		return a[0]
	case manager != "npm" && !slices.Contains([]string{"install", "add", "remove", "i", "x", "exec", "dlx", "create", "init", "why", "info", "upgrade", "update", "outdated", "link"}, a[0]):
		return a[0] // yarn build, pnpm build, bun build-script
	}
	return ""
}

// npmScripts returns the named scripts of a package.json.
func npmScripts(text string, names []string) []string {
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if json.Unmarshal([]byte(text), &pkg) != nil {
		return nil
	}
	var out []string
	for _, n := range names {
		if s, ok := pkg.Scripts[n]; ok {
			out = append(out, s)
		}
	}
	return out
}

// makeTargets are the targets a make command builds ("" for the default).
func makeTargets(args []string) []string {
	var out []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-f" || a == "-C" || a == "-j" || a == "-I" || a == "-o" || a == "-W":
			i++
		case strings.HasPrefix(a, "-") || strings.Contains(a, "="):
		default:
			out = append(out, a)
		}
	}
	if len(out) == 0 {
		return []string{""}
	}
	return out
}

var makeRule = regexp.MustCompile(`^([^\s:#=][^:=]*?)\s*::?(?:[^=]|$)(.*)$`)

// makeRecipes returns the recipe lines of the targets and of what they
// depend on, as one shell script; "" names the first target.
func makeRecipes(text string, targets []string) string {
	rules := map[string][]string{}
	deps := map[string][]string{}
	first := ""
	var current []string
	for _, line := range strings.Split(strings.ReplaceAll(text, "\\\n", " "), "\n") {
		if strings.HasPrefix(line, "\t") {
			recipe := strings.TrimLeft(strings.TrimSpace(line), "@-+")
			for _, t := range current {
				rules[t] = append(rules[t], recipe)
			}
			continue
		}
		m := makeRule.FindStringSubmatch(line)
		if m == nil {
			if strings.TrimSpace(line) != "" {
				current = nil
			}
			continue
		}
		current = strings.Fields(m[1])
		before, _, _ := strings.Cut(m[2], ";")
		for _, t := range current {
			deps[t] = append(deps[t], strings.Fields(before)...)
			if first == "" && !strings.HasPrefix(t, ".") {
				first = t
			}
		}
	}
	var lines []string
	seen := map[string]bool{}
	var visit func(string, int)
	visit = func(t string, depth int) {
		if seen[t] || depth > 8 {
			return
		}
		seen[t] = true
		for _, d := range deps[t] {
			visit(d, depth+1)
		}
		lines = append(lines, rules[t]...)
	}
	for _, t := range targets {
		if t == "" {
			t = first
		}
		visit(t, 0)
	}
	return strings.Join(lines, "\n")
}
