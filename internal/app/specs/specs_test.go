package specs

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"specforge/internal/adapters/fsys"
	"specforge/internal/app/clarify"
	"specforge/internal/app/layout"
	"specforge/internal/domain/spec"
	"specforge/internal/ports"
)

func newService(t *testing.T, lang string) (Service, string) {
	t.Helper()
	root := t.TempDir()
	return Service{
		Layout:   layout.Layout{Root: root},
		FS:       os.DirFS(root),
		Files:    fsys.OS{},
		Now:      func() time.Time { return time.Date(2026, 10, 4, 9, 30, 0, 0, time.UTC) },
		Language: lang,
	}, root
}

func write(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSlug(t *testing.T) {
	cases := map[string]string{
		"Liquidación de nóminas":   "liquidacion-de-nominas",
		"  Password   reset!! ":    "password-reset",
		"Ñandú & Co. 2.0":          "nandu-co-2-0",
		"日本語":                      "",
		strings.Repeat("abc ", 30): "abc-abc-abc-abc-abc-abc-abc-abc-abc-abc-abc-abc-abc-abc-abc",
	}
	for in, want := range cases {
		if got := Slug(in); got != want {
			t.Errorf("Slug(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNewNumbersAfterTheHighest(t *testing.T) {
	s, root := newService(t, "en")
	write(t, root, "specs/0001-a.md", "# a\n")
	write(t, root, "specs/0007-b.md", "# b\n")
	write(t, root, "specs/notes.md", "# not a spec\n")

	e, err := s.New(`Password "reset"`)
	if err != nil {
		t.Fatal(err)
	}
	if e.Rel != "specs/0008-password-reset.md" || e.ID != "0008" {
		t.Fatalf("got %+v", e)
	}
	data, _ := os.ReadFile(e.Path)
	m, ok, err := spec.ReadMeta(string(data))
	if err != nil || !ok {
		t.Fatalf("front matter: ok=%v err=%v", ok, err)
	}
	if m.ID != "0008" || m.Title != `Password "reset"` || m.Status != spec.StatusDraft || m.Created != "2026-10-04" {
		t.Fatalf("meta %+v", m)
	}
	// A fresh template is a draft that cannot be approved yet.
	if b := spec.Blocking(spec.Lint(string(data), spec.ParseOptions{})); len(b) == 0 {
		t.Fatal("a fresh template must have blocking placeholders")
	}
}

func TestNewSpanishTemplateParses(t *testing.T) {
	s, _ := newService(t, "es")
	e, err := s.New("Restablecer contraseña")
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(e.Path)
	doc, err := spec.Parse(string(data), spec.ParseOptions{Languages: []string{"es"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Scenarios) != 2 || len(doc.Scenarios[0].StepsOf(spec.When)) != 1 {
		t.Fatalf("unexpected scenarios %+v", doc.Scenarios)
	}
	if qs := spec.OpenQuestions(string(data)); len(qs) != 0 {
		t.Fatalf("the template must not carry questions: %v", qs)
	}
}

func TestResolve(t *testing.T) {
	s, root := newService(t, "en")
	if _, err := s.Resolve(""); !errors.Is(err, ErrNoSpecs) {
		t.Fatalf("want ErrNoSpecs, got %v", err)
	}
	write(t, root, "specs/0001-password-reset.md", "# a\n")
	if e, err := s.Resolve(""); err != nil || e.ID != "0001" {
		t.Fatalf("single spec: %+v %v", e, err)
	}
	write(t, root, "specs/0002-payments.md", "# b\n")

	var amb *AmbiguousError
	if _, err := s.Resolve(""); !errors.As(err, &amb) || len(amb.Candidates) != 2 {
		t.Fatalf("want ambiguity, got %v", err)
	}
	for _, arg := range []string{"2", "0002", "0002-pay", "specs/0002-payments.md", "./specs/0002-payments.md", filepath.Join(root, "specs", "0002-payments.md")} {
		if e, err := s.Resolve(arg); err != nil || e.ID != "0002" {
			t.Errorf("Resolve(%q) = %+v, %v", arg, e, err)
		}
	}
	var nf *NotFoundError
	if _, err := s.Resolve("42"); !errors.As(err, &nf) {
		t.Fatalf("want not found, got %v", err)
	}
}

const ready = "---\nid: \"0001\"\ntitle: \"Reset\"\nstatus: draft\n---\n\n# Reset\n\n" +
	"```gherkin\nFeature: Reset\n  Scenario: Link\n    When it is requested\n    Then a link is sent\n```\n"

func TestApproveSealsAndRecordsTheApprover(t *testing.T) {
	s, root := newService(t, "en")
	write(t, root, "specs/0001-reset.md", ready)
	e, _ := s.Resolve("1")

	a, err := s.Approve(e, "Ana")
	if err != nil {
		t.Fatal(err)
	}
	if a.Already || a.Resealed || a.Hash == "" {
		t.Fatalf("approval %+v", a)
	}
	data, _ := os.ReadFile(e.Path)
	if err := spec.Verify(string(data)); err != nil {
		t.Fatalf("not sealed: %v", err)
	}
	m, _, _ := spec.ReadMeta(string(data))
	if m.Status != spec.StatusApproved || m.ApprovedBy != "Ana" || m.ApprovedAt != "2026-10-04T09:30:00Z" {
		t.Fatalf("meta %+v", m)
	}
	if e, _ := s.Resolve("1"); e.State != StateApproved {
		t.Fatalf("state %s", e.State)
	}

	again, err := s.Approve(e, "Ana")
	if err != nil || !again.Already || again.Hash != a.Hash {
		t.Fatalf("second approval %+v %v", again, err)
	}

	// An edit is reported as changed, and approving it again is a re-seal.
	write(t, root, "specs/0001-reset.md", strings.Replace(string(data), "a link is sent", "a link is emailed", 1))
	if e, _ := s.Resolve("1"); e.State != StateChanged {
		t.Fatalf("state %s", e.State)
	}
	re, err := s.Approve(e, "Luis")
	if err != nil || !re.Resealed || re.Hash == a.Hash {
		t.Fatalf("re-approval %+v %v", re, err)
	}
}

func TestApproveRefusesBlockingIssues(t *testing.T) {
	s, root := newService(t, "en")
	write(t, root, "specs/0001-reset.md", ready+"\n- [NEEDS CLARIFICATION]: which channel?\n")
	e, _ := s.Resolve("1")
	_, err := s.Approve(e, "Ana")
	var lint *LintError
	if !errors.As(err, &lint) || lint.Issues[0].Rule != spec.RuleOpenQuestion {
		t.Fatalf("want lint error, got %v", err)
	}
	if _, err := s.Approve(e, " "); err == nil {
		t.Fatal("an approval without a name must fail")
	}
}

type answers []string

func (a *answers) Ask(context.Context, ports.Question) (string, error) {
	if len(*a) == 0 {
		return "", ports.ErrNonInteractive
	}
	out := (*a)[0]
	*a = (*a)[1:]
	return out, nil
}

func TestClarifyWritesDecisionsInPlace(t *testing.T) {
	s, root := newService(t, "en")
	write(t, root, "specs/0001-reset.md", ready+"\n## 12. Open questions\n\n- [NEEDS CLARIFICATION]: Which channel?\n- [NEEDS CLARIFICATION]: How long is a link valid?\n")
	e, _ := s.Resolve("1")
	asker := &clarify.Asker{Prompter: &answers{"email"}, Files: fsys.OS{}, Lang: "en", Now: s.Now}

	n, err := s.Clarify(context.Background(), e, asker, "Ana")
	var pending *clarify.PendingQuestionError
	if n != 1 || !errors.As(err, &pending) {
		t.Fatalf("n=%d err=%v", n, err)
	}
	data, _ := os.ReadFile(e.Path)
	if !strings.Contains(string(data), "- **Decided:** Which channel? → email (2026-10-04, Ana)") {
		t.Fatalf("spec:\n%s", data)
	}
	if qs := spec.OpenQuestions(string(data)); len(qs) != 1 {
		t.Fatalf("open: %v", qs)
	}
	if q, _ := os.ReadFile(filepath.Join(root, "specs/0001-reset/questions.md")); !strings.Contains(string(q), "How long is a link valid?") {
		t.Fatalf("questions.md:\n%s", q)
	}

	// The second question is answered in questions.md.
	qpath := filepath.Join(root, "specs/0001-reset/questions.md")
	q, _ := os.ReadFile(qpath)
	os.WriteFile(qpath, []byte(strings.Replace(string(q), "_awaiting an answer_", "30 minutes", 1)), 0o644)
	if n, err := s.Clarify(context.Background(), e, asker, "Ana"); n != 1 || err != nil {
		t.Fatalf("second run n=%d err=%v", n, err)
	}
	if _, err := s.Approve(e, "Ana"); err != nil {
		t.Fatalf("a clarified spec approves: %v", err)
	}
}

func TestApprovalHistoryRecordsTheDeltaByMarker(t *testing.T) {
	s, root := newService(t, "en")
	write(t, root, "specs/0001-reset.md", ready)
	e, _ := s.Resolve("1")
	a, err := s.Approve(e, "Ana")
	if err != nil || len(a.Delta) != 1 || a.Delta[0].Change != spec.Added || a.Delta[0].Marker != "SDD_0001_001" {
		t.Fatalf("first approval %+v %v", a.Delta, err)
	}

	// A scenario inserted before Link, and Link's outcome changed: Link
	// keeps its marker, the new one gets the next.
	data, _ := os.ReadFile(e.Path)
	edited := strings.Replace(string(data), "  Scenario: Link", "  Scenario: Expired\n    When it is opened late\n    Then it fails\n\n  Scenario: Link", 1)
	edited = strings.Replace(edited, "Then a link is sent", "Then a link is emailed", 1)
	write(t, root, "specs/0001-reset.md", edited)
	a, err = s.Approve(e, "Luis")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]spec.ScenarioChange{}
	for _, c := range a.Delta {
		got[c.Title] = c
	}
	if got["Link"].Change != spec.Modified || got["Link"].Marker != "SDD_0001_001" || got["Link"].Index != 2 ||
		got["Expired"].Change != spec.Added || got["Expired"].Marker != "SDD_0001_002" || len(got) != 2 {
		t.Fatalf("delta %+v", a.Delta)
	}
	log, _ := os.ReadFile(filepath.Join(root, "specs/0001-reset/approvals.md"))
	if strings.Count(string(log), "### ") != 2 || !strings.Contains(string(log), "· Luis · sha256-v1:") || !strings.Contains(string(log), "- MODIFIED · SDD_0001_001 · 2 · Link") {
		t.Fatalf("approvals.md:\n%s", log)
	}
	l, err := ReadLedger(s.Files, s.Layout, e.Path)
	if err != nil || len(l.Active()) != 2 || l.Next != 3 {
		t.Fatalf("ledger %+v %v", l, err)
	}
}

// A specification approved before the ledger existed starts from its
// loop's markers: an unchanged scenario is not reported as modified.
func TestTheFirstLedgerStartsFromTheLoop(t *testing.T) {
	s, root := newService(t, "en")
	write(t, root, "specs/0001-reset.md", ready)
	e, _ := s.Resolve("1")
	data, _ := os.ReadFile(e.Path)
	doc, _ := spec.Parse(string(data), spec.ParseOptions{})
	state := fmt.Sprintf(`{"version":3,"scenarios":[{"index":1,"title":"Link","marker":"SDD_0001_001","fingerprint":%q,"done":true}]}`, doc.Scenarios[0].Fingerprint())
	write(t, root, ".specforge/state/0001-reset.json", state)
	edited := strings.Replace(string(data), "  Scenario: Link", "  Scenario: Expired\n    When it is opened late\n    Then it fails\n\n  Scenario: Link", 1)
	write(t, root, "specs/0001-reset.md", edited)
	got, err := s.Preview(e)
	if err != nil {
		t.Fatal(err)
	}
	byTitle := map[string]spec.ScenarioChange{}
	for _, c := range got {
		byTitle[c.Title] = c
	}
	if byTitle["Link"].Change != spec.Unchanged || byTitle["Link"].Marker != "SDD_0001_001" || byTitle["Expired"].Marker != "SDD_0001_002" {
		t.Fatalf("%+v", got)
	}
}
