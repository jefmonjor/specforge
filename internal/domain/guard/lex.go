// Package guard recognises destructive shell commands before an agent runs
// them: recursive deletes, history-rewriting git commands, SQL that drops
// or empties data, and commands that touch secrets. It is a lexical
// recogniser, not an interpreter and not a sandbox: it reads the command
// line the agent wrote (unwrapping sudo, env, xargs, timeout and sh -c),
// and cannot see inside scripts, programs or variable expansions. For real
// containment, run the agent in a container.
package guard

import "strings"

// segments splits a command line into simple commands at ; && || | & and
// newlines, outside quotes, and adds the commands inside $( ) and
// backticks.
func segments(line string) []string {
	var out []string
	var cur strings.Builder
	var quote rune
	escaped := false
	flush := func() {
		if s := strings.TrimSpace(cur.String()); s != "" {
			out = append(out, s)
		}
		cur.Reset()
	}
	runes := []rune(line)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		switch {
		case escaped:
			escaped = false
			cur.WriteRune(r)
			continue
		case r == '\\' && quote != '\'':
			escaped = true
			cur.WriteRune(r)
			continue
		case quote != 0:
			if r == quote {
				quote = 0
			}
			cur.WriteRune(r)
			continue
		case r == '\'' || r == '"':
			quote = r
			cur.WriteRune(r)
			continue
		case r == ';' || r == '\n' || r == '|' || r == '&':
			flush()
			continue
		}
		cur.WriteRune(r)
	}
	flush()
	for _, inner := range substitutions(line) {
		out = append(out, segments(inner)...)
	}
	return out
}

// substitutions returns the commands inside $( ) and backticks.
func substitutions(line string) []string {
	var out []string
	for i := 0; i < len(line); i++ {
		switch {
		case strings.HasPrefix(line[i:], "$("):
			depth := 0
			for j := i + 1; j < len(line); j++ {
				switch line[j] {
				case '(':
					depth++
				case ')':
					depth--
					if depth == 0 {
						out = append(out, line[i+2:j])
						i = j
						j = len(line)
					}
				}
			}
		case line[i] == '`':
			if j := strings.IndexByte(line[i+1:], '`'); j >= 0 {
				out = append(out, line[i+1:i+1+j])
				i += j + 1
			}
		}
	}
	return out
}

// words splits a simple command into words the way a shell would, with
// quotes removed and backslash escapes applied. Expansions are left as
// they are written.
func words(cmd string) []string {
	var out []string
	var cur strings.Builder
	var quote rune
	inWord, escaped := false, false
	for _, r := range cmd {
		switch {
		case escaped:
			cur.WriteRune(r)
			escaped, inWord = false, true
		case r == '\\' && quote != '\'':
			escaped = true
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote, inWord = r, true
		case r == ' ' || r == '\t':
			if inWord {
				out = append(out, cur.String())
				cur.Reset()
				inWord = false
			}
		default:
			cur.WriteRune(r)
			inWord = true
		}
	}
	if inWord {
		out = append(out, cur.String())
	}
	return out
}
