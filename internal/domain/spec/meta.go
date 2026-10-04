package spec

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Status is the lifecycle state recorded in a specification's front matter.
type Status string

const (
	StatusDraft    Status = "draft"
	StatusApproved Status = "approved"
)

// Meta is the YAML front matter at the top of a specification.
type Meta struct {
	ID         string `yaml:"id"`
	Title      string `yaml:"title"`
	Status     Status `yaml:"status"`
	Created    string `yaml:"created"`
	ApprovedBy string `yaml:"approved_by"`
	ApprovedAt string `yaml:"approved_at"`
}

const frontMatterFence = "---"

// ReadMeta parses the front matter. ok is false when there is none, which is
// valid: specifications written before SpecForge 4 have no front matter.
func ReadMeta(markdown string) (m Meta, ok bool, err error) {
	lines, end := frontMatter(markdown)
	if end < 0 {
		return m, false, nil
	}
	if err := yaml.Unmarshal([]byte(strings.Join(lines[1:end], "\n")), &m); err != nil {
		return m, true, fmt.Errorf("front matter: %w", err)
	}
	return m, true, nil
}

// SetMeta returns markdown with the given front matter keys set, keeping
// every other line as it is. Keys that are missing are added at the end of
// the front matter; a document without front matter gets one.
func SetMeta(markdown string, values map[string]string, order ...string) (string, error) {
	text := normalizeNewlines(markdown)
	lines, end := frontMatter(text)
	if end < 0 {
		lines = append([]string{frontMatterFence, frontMatterFence}, strings.Split(text, "\n")...)
		end = 1
	}
	pending := map[string]bool{}
	for k := range values {
		pending[k] = true
	}
	for i := 1; i < end; i++ {
		key, _, found := strings.Cut(lines[i], ":")
		key = strings.TrimSpace(key)
		if !found || !pending[key] {
			continue
		}
		v, err := scalar(values[key])
		if err != nil {
			return "", err
		}
		lines[i] = key + ": " + v
		delete(pending, key)
	}
	var added []string
	for _, key := range append(order, slices.Sorted(maps.Keys(pending))...) {
		if !pending[key] {
			continue
		}
		v, err := scalar(values[key])
		if err != nil {
			return "", err
		}
		added = append(added, key+": "+v)
		delete(pending, key)
	}
	out := append(append(append([]string{}, lines[:end]...), added...), lines[end:]...)
	return strings.Join(out, "\n"), nil
}

// frontMatter splits markdown into lines and returns the index of the
// closing fence, or -1 when the document does not start with front matter.
func frontMatter(markdown string) ([]string, int) {
	lines := strings.Split(normalizeNewlines(markdown), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != frontMatterFence {
		return lines, -1
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == frontMatterFence {
			return lines, i
		}
	}
	return lines, -1
}

// scalar renders a YAML string that round-trips exactly.
func scalar(v string) (string, error) {
	n := yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: v, Style: yaml.DoubleQuotedStyle}
	b, err := yaml.Marshal(&n)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}
