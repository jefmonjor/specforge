// Package specs holds the specification lifecycle use cases: create a
// numbered specification from the template, find one without guessing,
// lint it and approve it (the R0 review gate, which seals it).
package specs

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"text/template"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"specforge/assets"
	"specforge/internal/app/layout"
	"specforge/internal/domain/spec"
	"specforge/internal/ports"
)

// ErrNoSpecs reports a project without specifications.
var ErrNoSpecs = errors.New("there is no specification in specs/: create one with `specforge spec new \"<title>\"`")

// AmbiguousError reports that more than one specification matches and the
// caller did not say which one. SpecForge never picks one for the developer.
type AmbiguousError struct {
	Candidates []string // project-relative paths
}

func (e *AmbiguousError) Error() string {
	return fmt.Sprintf("%d specifications match; name one of: %s", len(e.Candidates), strings.Join(e.Candidates, ", "))
}

// NotFoundError reports a specification argument that matches nothing.
type NotFoundError struct{ Arg string }

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("no specification matches %q: list them with `specforge spec list`", e.Arg)
}

// LintError reports blocking lint issues.
type LintError struct {
	Path   string
	Issues []spec.Issue
}

func (e *LintError) Error() string {
	return fmt.Sprintf("%s has %d issue(s) to fix before approval", e.Path, len(e.Issues))
}

// Service runs the use cases for one project.
type Service struct {
	Layout layout.Layout
	// FS reads the project (os.DirFS(root) in production).
	FS    fs.FS
	Files ports.Files
	Now   func() time.Time
	// Language selects the template ("en" or "es").
	Language string
}

// Entry describes one specification.
type Entry struct {
	Path  string // absolute
	Rel   string // project-relative, slash-separated
	ID    string
	Title string
	State State
}

// State summarises where a specification is in its lifecycle.
type State string

const (
	StateDraft    State = "draft"
	StateApproved State = "approved" // sealed, and the seal matches
	StateChanged  State = "changed"  // sealed, but edited since
)

var specFile = regexp.MustCompile(`^(\d{4})-[^/]+\.md$`)

// List returns the specifications under specs/, ordered by number.
func (s Service) List() ([]Entry, error) {
	dirEntries, err := fs.ReadDir(s.FS, "specs")
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Entry
	for _, d := range dirEntries {
		m := specFile.FindStringSubmatch(d.Name())
		if d.IsDir() || m == nil {
			continue
		}
		rel := path.Join("specs", d.Name())
		data, err := fs.ReadFile(s.FS, rel)
		if err != nil {
			return nil, err
		}
		out = append(out, describe(s.Layout.Abs(rel), rel, m[1], string(data)))
	}
	slices.SortFunc(out, func(a, b Entry) int { return strings.Compare(a.Rel, b.Rel) })
	return out, nil
}

func describe(abs, rel, id, content string) Entry {
	e := Entry{Path: abs, Rel: rel, ID: id, Title: strings.TrimSuffix(strings.TrimPrefix(path.Base(rel), id+"-"), ".md"), State: StateDraft}
	if m, ok, err := spec.ReadMeta(content); ok && err == nil && m.Title != "" {
		e.Title = m.Title
	}
	switch err := spec.Verify(content); {
	case err == nil:
		e.State = StateApproved
	case errors.Is(err, spec.ErrNotSealed):
		e.State = StateDraft
	default:
		e.State = StateChanged
	}
	return e
}

// Resolve finds the specification arg names: a path, a number ("1",
// "0001") or a file name prefix ("0001-pass"). With an empty arg it returns
// the only specification, or an *AmbiguousError listing them all.
func (s Service) Resolve(arg string) (Entry, error) {
	all, err := s.List()
	if err != nil {
		return Entry{}, err
	}
	if len(all) == 0 {
		return Entry{}, ErrNoSpecs
	}
	arg = strings.TrimSpace(arg)
	if arg == "" {
		if len(all) == 1 {
			return all[0], nil
		}
		return Entry{}, ambiguous(all)
	}

	if n, err := strconv.Atoi(arg); err == nil && n > 0 && n <= 9999 {
		id := fmt.Sprintf("%04d", n)
		return pick(arg, filter(all, func(e Entry) bool { return e.ID == id }))
	}
	want := filepath.ToSlash(arg)
	if filepath.IsAbs(arg) {
		want = s.Layout.Rel(arg)
	}
	want = strings.TrimPrefix(path.Clean(want), "./")
	if exact := filter(all, func(e Entry) bool { return e.Rel == want }); len(exact) == 1 {
		return exact[0], nil
	}
	base := strings.TrimSuffix(path.Base(want), ".md")
	return pick(arg, filter(all, func(e Entry) bool { return strings.HasPrefix(path.Base(e.Rel), base) }))
}

func pick(arg string, matches []Entry) (Entry, error) {
	switch len(matches) {
	case 0:
		return Entry{}, &NotFoundError{Arg: arg}
	case 1:
		return matches[0], nil
	default:
		return Entry{}, ambiguous(matches)
	}
}

func ambiguous(es []Entry) error {
	var c []string
	for _, e := range es {
		c = append(c, e.Rel)
	}
	return &AmbiguousError{Candidates: c}
}

func filter(es []Entry, keep func(Entry) bool) []Entry {
	var out []Entry
	for _, e := range es {
		if keep(e) {
			out = append(out, e)
		}
	}
	return out
}

// New creates specs/NNNN-slug.md from the template, numbered after the
// highest existing specification.
func (s Service) New(title string) (Entry, error) {
	title = strings.Join(strings.Fields(title), " ")
	slug := Slug(title)
	if slug == "" {
		return Entry{}, fmt.Errorf("the title %q has no letters or digits to name the file with", title)
	}
	all, err := s.List()
	if err != nil {
		return Entry{}, err
	}
	next := 1
	for _, e := range all {
		if n, _ := strconv.Atoi(e.ID); n >= next {
			next = n + 1
		}
	}
	if next > 9999 {
		return Entry{}, errors.New("specs/ already holds specification 9999")
	}
	id := fmt.Sprintf("%04d", next)
	rel := path.Join("specs", id+"-"+slug+".md")

	body, err := s.render(id, title)
	if err != nil {
		return Entry{}, err
	}
	abs := s.Layout.Abs(rel)
	if s.Files.Exists(abs) {
		return Entry{}, fmt.Errorf("%s already exists", rel)
	}
	if err := s.Files.WriteFile(abs, []byte(body)); err != nil {
		return Entry{}, err
	}
	return Entry{Path: abs, Rel: rel, ID: id, Title: title, State: StateDraft}, nil
}

func (s Service) render(id, title string) (string, error) {
	lang := s.Language
	if lang != "es" {
		lang = "en"
	}
	t, err := template.ParseFS(assets.FS, "templates/"+lang+"/spec.md")
	if err != nil {
		return "", err
	}
	var b bytes.Buffer
	data := struct{ ID, Title, Date string }{ID: id, Title: title, Date: s.Now().Format(time.DateOnly)}
	if err := t.Execute(&b, data); err != nil {
		return "", err
	}
	// The front matter is set through SetMeta so any title is valid YAML.
	return spec.SetMeta(b.String(), map[string]string{"id": id, "title": title})
}

// Slug turns a title into a file name: accents removed, lower case, words
// joined by hyphens ("Liquidación de nóminas" → "liquidacion-de-nominas").
func Slug(title string) string {
	var b strings.Builder
	hyphen := false
	for _, r := range norm.NFD.String(strings.ToLower(title)) {
		switch {
		case unicode.Is(unicode.Mn, r):
			// a combining accent: drop it, keep the base letter
		case r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)):
			b.WriteRune(r)
			hyphen = false
		case b.Len() > 0 && !hyphen:
			b.WriteByte('-')
			hyphen = true
		}
	}
	slug := strings.TrimRight(b.String(), "-")
	if len(slug) > 60 {
		slug = strings.TrimRight(slug[:60], "-")
	}
	return slug
}

// Lint reads and lints a specification.
func (s Service) Lint(e Entry) ([]spec.Issue, error) {
	data, err := s.Files.ReadFile(e.Path)
	if err != nil {
		return nil, err
	}
	return spec.Lint(string(data), spec.ParseOptions{Languages: []string{s.Language}}), nil
}

// Approval is the outcome of Approve.
type Approval struct {
	Hash string
	// Already is true when the specification was approved and unchanged.
	Already bool
	// Resealed is true when an approved specification had been edited and
	// this approval accepts the new content.
	Resealed bool
	// Advice are the non-blocking lint issues.
	Advice []spec.Issue
}

// Approve is the R0 review gate: the specification must lint clean, then
// it records who approved it and when, and seals it. Approving an edited
// specification again is how a developer accepts a change on purpose.
func (s Service) Approve(e Entry, by string) (Approval, error) {
	return s.approve(e.Path, e.Rel, by, func(content string) []spec.Issue {
		return spec.Lint(content, spec.ParseOptions{Languages: []string{s.Language}})
	})
}

// ErrNoPlan reports a specification without plan.md.
var ErrNoPlan = errors.New("this specification has no plan yet: create it with `specforge plan <spec>`")

// ErrPlanNeedsApprovedSpec: a plan is only drafted or approved against an
// approved, unchanged specification.
var ErrPlanNeedsApprovedSpec = errors.New("the plan follows the specification: approve the specification first")

// ApprovePlan is the R1 review gate: the plan must place a test for every
// scenario of the approved specification and leave no placeholder; then
// it is recorded and sealed like the specification.
func (s Service) ApprovePlan(e Entry, by string) (Approval, error) {
	doc, err := s.Approved(e)
	if err != nil {
		return Approval{}, err
	}
	path := s.Layout.Plan(e.Path)
	if !s.Files.Exists(path) {
		return Approval{}, ErrNoPlan
	}
	return s.approve(path, s.Layout.Rel(path), by, func(content string) []spec.Issue {
		return spec.LintPlan(content, Markers(e.ID, doc))
	})
}

// Approved parses a specification that must be approved and unchanged.
func (s Service) Approved(e Entry) (*spec.Document, error) {
	data, err := s.Files.ReadFile(e.Path)
	if err != nil {
		return nil, err
	}
	if err := spec.Verify(string(data)); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrPlanNeedsApprovedSpec, err)
	}
	return spec.Parse(string(data), spec.ParseOptions{Languages: []string{s.Language}})
}

// Markers lists the scenario markers of a parsed specification.
func Markers(specID string, doc *spec.Document) []string {
	out := make([]string, len(doc.Scenarios))
	for i, sc := range doc.Scenarios {
		out[i] = spec.Marker(specID, sc.Index)
	}
	return out
}

func (s Service) approve(path, rel, by string, lint func(string) []spec.Issue) (Approval, error) {
	by = strings.TrimSpace(by)
	if by == "" {
		return Approval{}, errors.New("an approval needs the approver's name")
	}
	data, err := s.Files.ReadFile(path)
	if err != nil {
		return Approval{}, err
	}
	content := string(data)
	verify := spec.Verify(content)
	if verify == nil {
		info, _ := spec.ReadSeal(content)
		return Approval{Hash: info.Hash, Already: true}, nil
	}

	issues := lint(content)
	if blocking := spec.Blocking(issues); len(blocking) > 0 {
		return Approval{}, &LintError{Path: rel, Issues: blocking}
	}

	content, err = spec.SetMeta(content, map[string]string{
		"status":      string(spec.StatusApproved),
		"approved_by": by,
		"approved_at": s.Now().UTC().Format(time.RFC3339),
	}, "status", "approved_by", "approved_at")
	if err != nil {
		return Approval{}, err
	}
	sealed, hash := spec.Seal(content)
	if err := s.Files.WriteFile(path, []byte(sealed)); err != nil {
		return Approval{}, err
	}
	var tampered *spec.TamperedError
	return Approval{Hash: hash, Resealed: errors.As(verify, &tampered), Advice: issues}, nil
}
