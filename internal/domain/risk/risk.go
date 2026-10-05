// Package risk decides how much scrutiny a change deserves, from what
// version control says it touched: which paths and how many lines. The
// verdict is a pure function of the change; the agent may raise it with a
// reason, never lower it. Review lenses, the verifier and the developer's
// review are all proportional to it.
package risk

import (
	"fmt"
	"regexp"
	"strings"

	"specforge/internal/domain/change"
)

// Tier is the level of scrutiny.
type Tier string

const (
	// Passive: documentation only. No review lens, no verifier.
	Passive Tier = "passive"
	// Medium: ordinary code. One review lens.
	Medium Tier = "medium"
	// High: sensitive paths or a large change. Every lens and the verifier.
	High Tier = "high"
)

// Tiers in increasing order of scrutiny.
var Tiers = []Tier{Passive, Medium, High}

func (t Tier) rank() int {
	for i, x := range Tiers {
		if x == t {
			return i
		}
	}
	return -1
}

// AtLeast reports whether t is o or stricter.
func (t Tier) AtLeast(o Tier) bool { return t.rank() >= o.rank() }

// Max returns the stricter of t and o.
func (t Tier) Max(o Tier) Tier {
	if o.rank() > t.rank() {
		return o
	}
	return t
}

// ParseTier reads a tier name; "" is Passive, the floor.
func ParseTier(s string) (Tier, error) {
	t := Tier(strings.ToLower(strings.TrimSpace(s)))
	if t == "" {
		return Passive, nil
	}
	if t.rank() < 0 {
		return "", fmt.Errorf("unknown risk tier %q (use passive, medium or high)", s)
	}
	return t, nil
}

// DefaultMaxLines is the size above which any change is high risk.
const DefaultMaxLines = 400

// DefaultHigh are the paths whose change is high risk: security and
// money, process execution, data migrations, infrastructure, CI and the
// build files that pull dependencies. Matched case-insensitively against
// the slash-separated path.
var DefaultHigh = []string{
	`(^|/)(auth|authn|authz|authentication|authorization|security|secrets?|credentials?|tokens?|passwords?|payments?|billing|permissions?|acl|shell|process|exec|migrations?|infra|deploy|ci)(/|\.[^/]*$|$)`,
	`(^|/)(go\.mod|package\.json|pom\.xml|build\.gradle(\.kts)?|settings\.gradle(\.kts)?|pyproject\.toml|setup\.py|requirements[^/]*\.txt|Dockerfile[^/]*|docker-compose[^/]*\.ya?ml|Makefile)$`,
	`(^|/)\.github/workflows/`,
}

// DefaultPassive are documentation paths.
var DefaultPassive = []string{`\.(md|markdown|txt|rst|adoc)$`, `(^|/)docs/`}

// Rules tune the classification.
type Rules struct {
	MaxLines int
	High     []*regexp.Regexp
	Passive  []*regexp.Regexp
	// Floor is the lowest tier any change gets (passive by default).
	Floor Tier
}

// NewRules compiles the patterns; nil lists take the defaults and
// maxLines <= 0 takes DefaultMaxLines.
func NewRules(maxLines int, high, passive []string, floor Tier) (Rules, error) {
	if maxLines <= 0 {
		maxLines = DefaultMaxLines
	}
	if high == nil {
		high = DefaultHigh
	}
	if passive == nil {
		passive = DefaultPassive
	}
	r := Rules{MaxLines: maxLines, Floor: floor}
	var err error
	if r.High, err = compile(high); err != nil {
		return Rules{}, fmt.Errorf("risk.high_paths: %w", err)
	}
	if r.Passive, err = compile(passive); err != nil {
		return Rules{}, fmt.Errorf("risk.passive_paths: %w", err)
	}
	if r.Floor == "" {
		r.Floor = Passive
	}
	return r, nil
}

// DefaultRules are NewRules with every default.
func DefaultRules() Rules {
	r, _ := NewRules(0, nil, nil, Passive)
	return r
}

func compile(patterns []string) ([]*regexp.Regexp, error) {
	out := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		re, err := regexp.Compile("(?i)" + p)
		if err != nil {
			return nil, fmt.Errorf("invalid pattern %q: %w", p, err)
		}
		out = append(out, re)
	}
	return out, nil
}

// Assessment is a tier and why.
type Assessment struct {
	Tier    Tier     `json:"tier"`
	Lines   int      `json:"lines"`
	Files   int      `json:"files"`
	Reasons []string `json:"reasons"`
	// Raised is the agent's reason when it raised the tier.
	Raised string `json:"raised,omitempty"`
}

// Classify assesses a change. Generated files (lock files, vendored code)
// do not count for size, but a dependency file such as go.mod does count
// as sensitive.
func Classify(files []change.File, r Rules) Assessment {
	authored := change.Authored(files)
	a := Assessment{Lines: change.Total(authored), Files: len(files)}
	var high []string
	passive := len(files) > 0
	for _, f := range files {
		for _, re := range r.High {
			if re.MatchString(f.Path) {
				high = append(high, fmt.Sprintf("`%s` is a sensitive path", f.Path))
				break
			}
		}
		if f.Binary {
			a.Reasons = append(a.Reasons, fmt.Sprintf("`%s` is binary", f.Path))
		}
		if !matchesAny(f.Path, r.Passive) {
			passive = false
		}
	}
	if a.Lines > r.MaxLines {
		high = append(high, fmt.Sprintf("%d changed lines > %d", a.Lines, r.MaxLines))
	}
	switch {
	case len(high) > 0:
		a.Tier, a.Reasons = High, append(high, a.Reasons...)
	case len(files) == 0:
		a.Tier, a.Reasons = Passive, []string{"no file changed"}
	case passive:
		a.Tier, a.Reasons = Passive, []string{"documentation only"}
	default:
		a.Tier = Medium
		a.Reasons = append([]string{fmt.Sprintf("code changed: %d line(s) in %d file(s)", a.Lines, len(files))}, a.Reasons...)
	}
	if !a.Tier.AtLeast(r.Floor) {
		a.Tier = r.Floor
		a.Reasons = append(a.Reasons, fmt.Sprintf("the configured floor is %s", r.Floor))
	}
	return a
}

// Escalate raises the tier to to, keeping why. A lower or equal tier, or an
// unknown one, changes nothing: the agent can ask for more scrutiny, never
// for less.
func (a Assessment) Escalate(to Tier, why string) Assessment {
	if to.rank() <= a.Tier.rank() {
		return a
	}
	a.Tier = to
	a.Raised = strings.TrimSpace(why)
	a.Reasons = append(a.Reasons, "raised by the agent: "+a.Raised)
	return a
}

func matchesAny(p string, res []*regexp.Regexp) bool {
	for _, re := range res {
		if re.MatchString(p) {
			return true
		}
	}
	return false
}
