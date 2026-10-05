package spec

import (
	"path"
	"regexp"
	"slices"
	"strings"
)

// Section titles of a technical plan, in English and Spanish.
var (
	ComponentsTitle    = regexp.MustCompile(`(?i)^(components|componentes)\b`)
	PlanTestsTitle     = regexp.MustCompile(`(?i)^tests? (per|por) (scenario|escenario)`)
	backticked         = regexp.MustCompile("`([^`\\s]+)`")
	pathLike           = regexp.MustCompile(`^[A-Za-z0-9_.@/\-\[\]]+$`)
	hasExtension       = regexp.MustCompile(`\.[A-Za-z0-9]{1,10}$`)
	newWord            = regexp.MustCompile(`(?i)\b(new|nuevo|nueva|create|crear)\b`)
	componentLinePrefx = regexp.MustCompile(`^\s*([-*+]|\d+[.)]|\|)\s*`)
)

// Surfaces are the files an approved plan allows the agent to edit: its
// components, its planned test files, and anything under the directory of
// a component it creates.
type Surfaces struct {
	Files []string `json:"files"`
	// Dirs are directories whose whole content is allowed: the directory
	// of each new component, and components that are directories.
	Dirs []string `json:"dirs,omitempty"`
}

// Empty reports a plan that names no file: nothing can be checked.
func (s Surfaces) Empty() bool { return len(s.Files) == 0 && len(s.Dirs) == 0 }

// Allows reports whether the plan covers the slash-separated path rel.
func (s Surfaces) Allows(rel string) bool {
	rel = cleanRel(rel)
	if slices.Contains(s.Files, rel) {
		return true
	}
	for _, d := range s.Dirs {
		if strings.HasPrefix(rel, d+"/") {
			return true
		}
	}
	return false
}

// List is every file and directory, for prompts.
func (s Surfaces) List() []string {
	out := slices.Clone(s.Files)
	for _, d := range s.Dirs {
		out = append(out, d+"/")
	}
	slices.Sort(out)
	return out
}

// PlanSurfaces reads the surfaces of a plan from its components section
// and its tests-per-scenario table.
func PlanSurfaces(plan string) Surfaces {
	var s Surfaces
	files := map[string]bool{}
	dirs := map[string]bool{}
	for _, line := range strings.Split(Section(plan, ComponentsTitle), "\n") {
		paths := linePaths(line)
		for _, p := range paths {
			if strings.HasSuffix(p, "/") {
				dirs[strings.TrimSuffix(p, "/")] = true
				continue
			}
			files[p] = true
			if newWord.MatchString(line) && path.Dir(p) != "." {
				dirs[path.Dir(p)] = true
			}
		}
	}
	for _, line := range strings.Split(Section(plan, PlanTestsTitle), "\n") {
		for _, p := range linePaths(line) {
			if !strings.HasSuffix(p, "/") {
				files[p] = true
			}
		}
	}
	for f := range files {
		s.Files = append(s.Files, f)
	}
	for d := range dirs {
		s.Dirs = append(s.Dirs, d)
	}
	slices.Sort(s.Files)
	slices.Sort(s.Dirs)
	return s
}

// unparsedComponents returns the component lines that name no path.
func unparsedComponents(plan string) []string {
	var out []string
	for _, line := range strings.Split(Section(plan, ComponentsTitle), "\n") {
		text := strings.TrimSpace(line)
		if text == "" || strings.HasPrefix(text, "|---") || strings.HasPrefix(text, "| :") || strings.HasPrefix(text, "|--") {
			continue
		}
		if len(linePaths(line)) == 0 {
			out = append(out, text)
		}
	}
	return out
}

// linePaths returns the paths a line names: every backticked path, or else
// its first word when that is a path.
func linePaths(line string) []string {
	var out []string
	for _, m := range backticked.FindAllStringSubmatch(line, -1) {
		if p, ok := asPath(m[1]); ok {
			out = append(out, p)
		}
	}
	if len(out) > 0 {
		return out
	}
	if trimmed := strings.TrimSpace(line); strings.HasPrefix(trimmed, "|") {
		for _, cell := range strings.Split(trimmed, "|") {
			if p, ok := asPath(strings.TrimSpace(cell)); ok {
				out = append(out, p)
			}
		}
		return out
	}
	rest := componentLinePrefx.ReplaceAllString(line, "")
	if fields := strings.FieldsFunc(rest, func(r rune) bool { return r == ' ' || r == '|' || r == '\t' }); len(fields) > 0 {
		if p, ok := asPath(strings.Trim(fields[0], "*:,;")); ok {
			out = append(out, p)
		}
	}
	return out
}

// asPath accepts a token that names a file or directory of the project.
func asPath(tok string) (string, bool) {
	if !pathLike.MatchString(tok) || strings.Contains(tok, "://") || strings.HasPrefix(tok, "-") || strings.HasPrefix(tok, "/") || strings.Contains(tok, "..") {
		return "", false
	}
	if !strings.Contains(tok, "/") && !hasExtension.MatchString(tok) {
		return "", false
	}
	if strings.HasSuffix(tok, "/") {
		return cleanRel(tok) + "/", true
	}
	return cleanRel(tok), true
}

func cleanRel(p string) string {
	return strings.TrimPrefix(path.Clean(strings.ReplaceAll(p, "\\", "/")), "./")
}

var scenarioMarker = regexp.MustCompile(`\bSDD_\d{4}_\d{3}\b`)

// ScenarioSurfaces reads the surfaces of each scenario from a plan: a
// component line that names markers belongs to those scenarios, one that
// names none is shared by all of them, and each row of the tests table
// belongs to its marker.
func ScenarioSurfaces(plan string, markers []string) map[string]Surfaces {
	files := map[string]map[string]bool{}
	dirs := map[string]map[string]bool{}
	for _, m := range markers {
		files[m], dirs[m] = map[string]bool{}, map[string]bool{}
	}
	addTo := func(owners []string, p string, newDir bool) {
		for _, m := range owners {
			if files[m] == nil {
				continue
			}
			switch {
			case strings.HasSuffix(p, "/"):
				dirs[m][strings.TrimSuffix(p, "/")] = true
			default:
				files[m][p] = true
				if newDir && path.Dir(p) != "." {
					dirs[m][path.Dir(p)] = true
				}
			}
		}
	}
	for _, line := range strings.Split(Section(plan, ComponentsTitle), "\n") {
		owners := scenarioMarker.FindAllString(line, -1)
		if len(owners) == 0 {
			owners = markers
		}
		for _, p := range linePaths(line) {
			addTo(owners, p, newWord.MatchString(line))
		}
	}
	for _, line := range strings.Split(Section(plan, PlanTestsTitle), "\n") {
		owners := scenarioMarker.FindAllString(line, -1)
		for _, p := range linePaths(line) {
			addTo(owners, p, false)
		}
	}
	out := map[string]Surfaces{}
	for _, m := range markers {
		var s Surfaces
		for f := range files[m] {
			s.Files = append(s.Files, f)
		}
		for d := range dirs[m] {
			s.Dirs = append(s.Dirs, d)
		}
		slices.Sort(s.Files)
		slices.Sort(s.Dirs)
		out[m] = s
	}
	return out
}

// Overlaps reports whether two surfaces could touch the same file: a file
// in both, or a file or directory of one inside a directory of the other.
// Empty surfaces overlap everything: nothing is known about them.
func (s Surfaces) Overlaps(o Surfaces) bool {
	if s.Empty() || o.Empty() {
		return true
	}
	for _, f := range s.Files {
		if o.Allows(f) {
			return true
		}
	}
	for _, f := range o.Files {
		if s.Allows(f) {
			return true
		}
	}
	for _, a := range s.Dirs {
		for _, b := range o.Dirs {
			if a == b || strings.HasPrefix(a, b+"/") || strings.HasPrefix(b, a+"/") {
				return true
			}
		}
	}
	return false
}
