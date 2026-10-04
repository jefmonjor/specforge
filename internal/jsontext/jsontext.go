// Package jsontext finds JSON values embedded in model output, which mixes
// prose, Markdown fences and JSON.
package jsontext

import (
	"encoding/json"
	"regexp"
	"strings"
)

var fenced = regexp.MustCompile("(?s)```(?:json)?[ \\t]*\\n(.*?)\\n?[ \\t]*```")

// Candidates returns the JSON objects and arrays found in s, most likely
// answer first: fenced blocks from last to first, then bare values from
// the end of the text backwards. Each candidate is syntactically valid JSON.
func Candidates(s string) []string {
	var out []string
	seen := map[string]bool{}
	add := func(c string) {
		c = strings.TrimSpace(c)
		if c != "" && !seen[c] && json.Valid([]byte(c)) {
			seen[c] = true
			out = append(out, c)
		}
	}
	blocks := fenced.FindAllStringSubmatch(s, -1)
	for i := len(blocks) - 1; i >= 0; i-- {
		add(blocks[i][1])
	}
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] != '{' && s[i] != '[' {
			continue
		}
		var raw json.RawMessage
		if json.NewDecoder(strings.NewReader(s[i:])).Decode(&raw) == nil {
			add(string(raw))
		}
	}
	return out
}

// Decode stores into v the first candidate that unmarshals and passes ok.
// It reports whether one did.
func Decode[T any](s string, ok func(T) bool) (T, bool) {
	for _, c := range Candidates(s) {
		var v T
		if json.Unmarshal([]byte(c), &v) == nil && (ok == nil || ok(v)) {
			return v, true
		}
	}
	var zero T
	return zero, false
}
