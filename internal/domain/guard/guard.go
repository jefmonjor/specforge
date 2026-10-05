package guard

import (
	"path"
	"regexp"
	"slices"
	"strings"
)

// Kind of destructive command.
type Kind string

const (
	// FS deletes or overwrites files.
	FS Kind = "fs"
	// Git discards work or rewrites history.
	Git Kind = "git"
	// DB drops or empties data.
	DB Kind = "db"
	// Sensitive reads or writes secrets.
	Sensitive Kind = "sensitive"
)

// Match is one destructive command found in a command line.
type Match struct {
	Kind Kind `json:"kind"`
	// HardDeny marks what is never allowed, whatever the configuration:
	// deleting the root, the home directory or the whole project.
	HardDeny bool   `json:"hard_deny,omitempty"`
	Reason   string `json:"reason"`
	// Segment is the simple command it was found in.
	Segment string `json:"segment"`
}

// Recognize returns every destructive command in line, the code given
// inline to an interpreter included. Inspect also reads the files the
// line runs.
func Recognize(line string) []Match {
	return Inspect(line, nil)
}

// Allowed reports whether a match is allowed by one of the patterns
// (shell globs over the whole simple command, * matching anything). A
// hard-deny match never is.
func Allowed(m Match, patterns []string) bool {
	if m.HardDeny {
		return false
	}
	for _, p := range patterns {
		if glob(strings.TrimSpace(p), strings.Join(strings.Fields(m.Segment), " ")) {
			return true
		}
	}
	return false
}

// simple recognises one simple command, its wrappers already unwrapped.
func simple(seg string, ws []string) []Match {
	var out []Match
	add := func(k Kind, hard bool, reason string) {
		out = append(out, Match{Kind: k, HardDeny: hard, Reason: reason, Segment: seg})
	}
	switch name := path.Base(ws[0]); {
	case name == "rm":
		rm(ws[1:], add)
	case name == "find":
		if slices.Contains(ws, "-delete") || execRm(ws) {
			add(FS, false, "find deletes the files it matches")
		}
	case name == "git":
		if reason := gitDestructive(ws[1:]); reason != "" {
			add(Git, false, reason)
		}
	case name == "rimraf" || name == "del-cli" || name == "trash":
		add(FS, false, name+" deletes directory trees")
	case strings.HasPrefix(name, "mkfs"), name == "shred", name == "wipefs":
		add(FS, false, name+" destroys data")
	case name == "dd" && slices.ContainsFunc(ws, func(w string) bool { return strings.HasPrefix(w, "of=/dev/") }):
		add(FS, true, "dd writes over a device")
	}
	for _, w := range ws[1:] {
		if secret(w) {
			add(Sensitive, false, "touches a secret: "+w)
			break
		}
	}
	return out
}

// wrappers run the command that follows them; their own options are
// skipped. The value is how many option arguments take a value.
var wrappers = map[string]map[string]bool{
	"sudo":    {"-u": true, "-g": true, "-C": true, "-p": true, "-U": true, "-h": false},
	"env":     {"-u": true, "-C": true},
	"xargs":   {"-I": true, "-n": true, "-L": true, "-P": true, "-d": true, "-E": true, "-s": true, "-a": true},
	"timeout": {"-s": true, "-k": true},
	"nice":    {"-n": true},
	"nohup":   {},
	"time":    {},
	"command": {},
	"exec":    {},
	"builtin": {},
	"stdbuf":  {},
	"npx":     {"-p": true, "--package": true, "-c": false},
	"pnpx":    {},
	"bunx":    {},
}

var assignment = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)

// unwrap drops leading assignments and wrapper commands with their options.
func unwrap(ws []string) []string {
	for len(ws) > 0 {
		if assignment.MatchString(ws[0]) {
			ws = ws[1:]
			continue
		}
		opts, ok := wrappers[path.Base(ws[0])]
		if !ok {
			return ws
		}
		name := path.Base(ws[0])
		ws = ws[1:]
		for len(ws) > 0 && (strings.HasPrefix(ws[0], "-") || (name == "env" && assignment.MatchString(ws[0]))) {
			takesValue := opts[ws[0]]
			ws = ws[1:]
			if takesValue && len(ws) > 0 {
				ws = ws[1:]
			}
		}
		if name == "timeout" && len(ws) > 0 {
			ws = ws[1:] // the duration
		}
	}
	return ws
}

// shellC returns the script of `sh -c "script"` and its relatives.
func shellC(ws []string) (string, bool) {
	switch path.Base(ws[0]) {
	case "sh", "bash", "zsh", "dash", "ksh":
	default:
		return "", false
	}
	for i := 1; i < len(ws)-1; i++ {
		if strings.HasPrefix(ws[i], "-") && strings.Contains(ws[i], "c") && !strings.HasPrefix(ws[i], "--") {
			return ws[i+1], true
		}
	}
	return "", false
}

// hardTargets are what no command may ever delete.
var hardTargets = []string{"/", "/*", "~", "~/", "~/*", ".", "./", "./*", "..", "../", "*", "$HOME", "${HOME}", "$HOME/", "$HOME/*", "/home", "/root", "/usr", "/etc", "/var"}

func rm(args []string, add func(Kind, bool, string)) {
	recursive, targets, options := false, []string{}, true
	for _, a := range args {
		switch {
		case options && a == "--":
			options = false
		case options && (a == "--recursive" || a == "-r" || a == "-R"):
			recursive = true
		case options && strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "--"):
			recursive = recursive || strings.ContainsAny(a, "rR")
		case options && strings.HasPrefix(a, "--"):
		default:
			targets = append(targets, a)
		}
	}
	for _, t := range targets {
		if slices.Contains(hardTargets, t) || path.Clean(t) == "/" {
			add(FS, true, "rm of "+t+" is never allowed")
			return
		}
	}
	if recursive {
		add(FS, false, "recursive delete: rm -r "+strings.Join(targets, " "))
	}
}

func execRm(ws []string) bool {
	for i, w := range ws {
		if (w == "-exec" || w == "-execdir") && i+1 < len(ws) && path.Base(ws[i+1]) == "rm" {
			return true
		}
	}
	return false
}

// gitArgs are the arguments after a git subcommand.
type gitArgs []string

// has reports any of the long or exact flags.
func (a gitArgs) has(flags ...string) bool {
	return slices.ContainsFunc(a, func(w string) bool { return slices.Contains(flags, w) })
}

// short reports a short flag cluster containing letter (-fdx has f).
func (a gitArgs) short(letter string) bool {
	return slices.ContainsFunc(a, func(w string) bool {
		return strings.HasPrefix(w, "-") && !strings.HasPrefix(w, "--") && strings.Contains(w, letter)
	})
}

func (a gitArgs) first(values ...string) bool { return len(a) > 0 && slices.Contains(values, a[0]) }

// gitChecks name what each git subcommand destroys, given its arguments.
var gitChecks = map[string]func(gitArgs) string{
	"reset": func(a gitArgs) string {
		return when(a.has("--hard", "--merge", "--keep"), "git reset --hard discards uncommitted work")
	},
	"clean": func(a gitArgs) string {
		return when(a.has("--force") || a.short("f"), "git clean -f deletes untracked files")
	},
	"push": func(a gitArgs) string {
		refspec := slices.ContainsFunc(a, func(w string) bool { return strings.HasPrefix(w, "+") || strings.HasPrefix(w, ":") })
		forced := a.has("--force", "--force-with-lease", "--force-if-includes", "--delete", "--mirror", "--prune") || a.short("f") || a.short("d")
		return when(forced || refspec, "git push rewrites or deletes remote history")
	},
	"branch": func(a gitArgs) string {
		return when(a.has("-D") || (a.has("--delete", "-d") && a.has("--force", "-f")), "git branch -D deletes a branch with unmerged work")
	},
	"stash": func(a gitArgs) string {
		return when(a.first("drop", "clear"), "git stash drop/clear deletes stashed work")
	},
	"checkout": func(a gitArgs) string {
		return when(a.has("--force", "-f", ".") || (a.has("--") && len(a) > 1), "git checkout discards uncommitted changes")
	},
	"restore": func(a gitArgs) string {
		return when(!a.has("--staged", "-S") || a.has("--worktree", "-W"), "git restore discards uncommitted changes")
	},
	"filter-branch": func(gitArgs) string { return "git filter-branch rewrites history" },
	"filter-repo":   func(gitArgs) string { return "git filter-repo rewrites history" },
	"reflog": func(a gitArgs) string {
		return when(a.first("expire", "delete"), "git reflog expire/delete removes the way back")
	},
	"update-ref": func(a gitArgs) string { return when(a.has("-d"), "git update-ref -d deletes a ref") },
}

func when(cond bool, reason string) string {
	if cond {
		return reason
	}
	return ""
}

// gitDestructive names what a git command destroys, or "".
func gitDestructive(args []string) string {
	// Global options before the subcommand.
	for len(args) > 0 && strings.HasPrefix(args[0], "-") {
		if args[0] == "-C" || args[0] == "-c" {
			args = args[1:]
		}
		args = args[1:]
	}
	if len(args) == 0 {
		return ""
	}
	if check, ok := gitChecks[args[0]]; ok {
		return check(gitArgs(args[1:]))
	}
	return ""
}

var (
	// A SQL line comment is "-- " (with the space), so command-line options
	// such as --eval are not taken for one.
	sqlComment  = regexp.MustCompile(`(?s)/\*.*?\*/|(^|\s)--(\s[^\n]*|$)`)
	sqlDrop     = regexp.MustCompile(`(?i)\bDROP\s+(TABLE|DATABASE|SCHEMA|INDEX|VIEW|COLLECTION|USER|ROLE|KEYSPACE)\b`)
	sqlTruncate = regexp.MustCompile(`(?i)\bTRUNCATE\s+(TABLE\s+)?["\x60\w.]+`)
	sqlDelete   = regexp.MustCompile(`(?i)\bDELETE\s+FROM\b`)
	sqlUpdate   = regexp.MustCompile(`(?i)\bUPDATE\s+["\x60\w.]+\s+SET\b`)
	sqlWhere    = regexp.MustCompile(`(?i)\bWHERE\b`)
	nosqlDrop   = regexp.MustCompile(`(?i)\.(drop|dropDatabase)\(\s*\)|\bdeleteMany\(\s*\{\s*\}\s*\)|\bFLUSH(ALL|DB)\b`)
)

// sql finds statements that drop or empty data: DROP, TRUNCATE, DELETE or
// UPDATE without WHERE, and their NoSQL relatives.
func sql(line string) []Match {
	text := sqlComment.ReplaceAllString(line, " ")
	var out []Match
	add := func(reason, stmt string) {
		out = append(out, Match{Kind: DB, Reason: reason, Segment: strings.TrimSpace(stmt)})
	}
	for _, stmt := range strings.Split(text, ";") {
		switch {
		case sqlDrop.MatchString(stmt):
			add("DROP removes a whole object", stmt)
		case sqlTruncate.MatchString(stmt):
			add("TRUNCATE empties a table", stmt)
		case sqlDelete.MatchString(stmt) && !sqlWhere.MatchString(stmt):
			add("DELETE without WHERE empties a table", stmt)
		case sqlUpdate.MatchString(stmt) && !sqlWhere.MatchString(stmt):
			add("UPDATE without WHERE rewrites every row", stmt)
		case nosqlDrop.MatchString(stmt):
			add("drops or empties a whole collection or database", stmt)
		}
	}
	return out
}

// secret reports an argument that names a secret file.
func secret(w string) bool {
	w = strings.Trim(w, `"'`)
	if strings.HasPrefix(w, "-") && strings.Contains(w, "=") {
		_, w, _ = strings.Cut(w, "=")
	}
	base := path.Base(w)
	switch {
	case base == ".env" || strings.HasPrefix(base, ".env.") && !strings.HasSuffix(base, ".example") && !strings.HasSuffix(base, ".sample"):
		return true
	case strings.Contains(w, ".ssh/") || base == ".ssh":
		return true
	case strings.HasSuffix(base, ".pem") || strings.HasSuffix(base, ".p12") || strings.HasSuffix(base, ".pfx"):
		return true
	case base == "id_rsa" || base == "id_ed25519" || base == "id_ecdsa" || base == "id_dsa":
		return true
	case strings.Contains(strings.ToLower(base), "credentials"), base == ".netrc", base == ".pgpass", base == ".npmrc", base == ".pypirc":
		return true
	}
	return false
}

func dedupe(ms []Match) []Match {
	var out []Match
	for _, m := range ms {
		if !slices.Contains(out, m) {
			out = append(out, m)
		}
	}
	return out
}

// glob matches s against a pattern where * matches any text.
func glob(pattern, s string) bool {
	re := "^" + strings.ReplaceAll(regexp.QuoteMeta(pattern), `\*`, ".*") + "$"
	ok, _ := regexp.MatchString(re, s)
	return ok
}
