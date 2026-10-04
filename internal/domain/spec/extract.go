package spec

import (
	"regexp"
	"strings"

	gherkin "github.com/cucumber/gherkin/go/v40"
)

var (
	fenceOpen    = regexp.MustCompile("^\\s*(```+|~~~+)\\s*([A-Za-z0-9_+-]*)")
	gherkinInfos = map[string]bool{"gherkin": true, "feature": true, "cucumber": true}
)

// fencedGherkin returns the bodies of the Markdown code blocks tagged as
// Gherkin, in document order.
func fencedGherkin(md string) []string {
	var blocks []string
	var cur strings.Builder
	fence, inBlock, keep := "", false, false

	for _, line := range strings.Split(normalizeNewlines(md), "\n") {
		if !inBlock {
			if m := fenceOpen.FindStringSubmatch(line); m != nil {
				fence, inBlock = m[1], true
				keep = gherkinInfos[strings.ToLower(m[2])]
				cur.Reset()
			}
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(line), fence) {
			if keep {
				blocks = append(blocks, cur.String())
			}
			inBlock = false
			continue
		}
		if keep {
			cur.WriteString(line + "\n")
		}
	}
	return blocks
}

// parseLoose reads Gherkin written straight into Markdown. For each
// candidate language it keeps only the lines that are Gherkin in that
// dialect and parses them; the reading with most scenarios wins.
func parseLoose(md string, langs []string) (parsed, error) {
	var best parsed
	var firstErr error
	found := false
	for _, lang := range langs {
		src, ok := looseSource(md, gherkin.DialectsBuiltin().GetDialect(lang))
		if !ok {
			continue
		}
		p, err := parseGherkin(src, lang)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if !found || len(p.scenarios) > len(best.scenarios) {
			best, found = p, true
		}
	}
	if found {
		return best, nil
	}
	if firstErr != nil {
		return parsed{}, firstErr
	}
	return parsed{}, ErrNoScenarios
}

var (
	headingPrefix = regexp.MustCompile(`^#{1,6}\s+`)
	listPrefix    = regexp.MustCompile(`^(?:[-*+]|\d+[.)])\s+`)
	quotePrefix   = regexp.MustCompile(`^>\s?`)
)

// looseSource builds a Gherkin document from the Gherkin lines of md. It
// reports false when md has no scenario line in dialect d.
func looseSource(md string, d *gherkin.Dialect) (string, bool) {
	if d == nil {
		return "", false
	}
	blockKeywords := withColon(d.FeatureKeywords(), d.RuleKeywords(), d.BackgroundKeywords(),
		d.ScenarioKeywords(), d.ScenarioOutlineKeywords(), d.ExamplesKeywords())
	scenarioKeywords := withColon(d.ScenarioKeywords(), d.ScenarioOutlineKeywords())
	featureKeywords := withColon(d.FeatureKeywords())

	var lines []string
	hasScenario, hasFeature, inDocString := false, false, false

	for _, raw := range strings.Split(normalizeNewlines(md), "\n") {
		line := stripMarkdown(raw)
		if inDocString {
			lines = append(lines, line)
			if strings.HasPrefix(line, `"""`) {
				inDocString = false
			}
			continue
		}
		switch {
		case hasPrefixAny(line, featureKeywords):
			hasFeature = true
			lines = append(lines, line)
		case hasPrefixAny(line, scenarioKeywords):
			hasScenario = true
			lines = append(lines, line)
		case hasPrefixAny(line, blockKeywords):
			lines = append(lines, line)
		case !hasScenario:
			// Steps, tables and tags only count inside a scenario.
		case hasPrefixAny(line, d.StepKeywords()), strings.HasPrefix(line, "|"), strings.HasPrefix(line, "@"):
			lines = append(lines, line)
		case strings.HasPrefix(line, `"""`):
			inDocString = true
			lines = append(lines, line)
		}
	}
	if !hasScenario {
		return "", false
	}
	if !hasFeature {
		lines = append([]string{d.FeatureKeywords()[0] + ": Specification"}, lines...)
	}
	return strings.Join(lines, "\n") + "\n", true
}

func stripMarkdown(line string) string {
	t := strings.TrimSpace(line)
	t = quotePrefix.ReplaceAllString(t, "")
	t = headingPrefix.ReplaceAllString(t, "")
	t = listPrefix.ReplaceAllString(t, "")
	// "**Scenario:** title" and "__Given__ x" → plain keywords.
	t = strings.ReplaceAll(t, "**", "")
	t = strings.ReplaceAll(t, "__", "")
	return strings.TrimSpace(t)
}

func withColon(groups ...[]string) []string {
	var out []string
	for _, g := range groups {
		for _, k := range g {
			out = append(out, strings.TrimSpace(k)+":")
		}
	}
	return out
}

func hasPrefixAny(s string, prefixes []string) bool {
	for _, p := range prefixes {
		if p != "" && strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

func normalizeNewlines(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}
