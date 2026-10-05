package deliver

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"specforge/internal/adapters/fsys"
	"specforge/internal/domain/change"
	"specforge/internal/domain/spec"
	"specforge/internal/domain/tdd"
)

const specMD = "# Discounts\n\n## 5. Acceptance criteria\n\n```gherkin\nFeature: Discounts\n" +
	"  Scenario: A valid code\n    Given a code\n    When applied\n    Then 10% off\n\n" +
	"  Scenario: An expired code\n    Given an old code\n    When applied\n    Then it is refused\n\n" +
	"  Scenario: Stacking\n    Given two codes\n    When applied\n    Then only one counts\n```\n"

type measurer map[string][]change.File

func (m measurer) Changes(context.Context, string, []string) ([]change.File, error) { return nil, nil }
func (m measurer) CommitChanges(_ context.Context, _, sha string) ([]change.File, error) {
	files, ok := m[sha]
	if !ok {
		return nil, errors.New("unknown revision")
	}
	return files, nil
}

// project writes an approved specification whose three scenarios are done
// with the given commits.
func project(t *testing.T, commits ...string) Options {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, "specs", "0001-discounts.md")
	sealed, _ := spec.Seal(specMD)
	if err := fsys.WriteAtomic(path, []byte(sealed), 0o644); err != nil {
		t.Fatal(err)
	}
	doc, err := spec.Parse(sealed, spec.ParseOptions{})
	if err != nil {
		t.Fatal(err)
	}
	st := tdd.State{Version: tdd.StateVersion, SpecPath: "specs/0001-discounts.md"}
	for i, sc := range doc.Scenarios {
		st.Scenarios = append(st.Scenarios, tdd.ScenarioRef{Index: sc.Index, Title: sc.Title, Fingerprint: sc.Fingerprint(), Done: true, Commit: commits[i]})
	}
	data, _ := json.Marshal(st)
	if err := fsys.WriteAtomic(filepath.Join(root, ".specforge", "state", "0001-discounts.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	return Options{Root: root, SpecPath: path, Language: "en", Now: time.Now(), Budget: 400, SliceBodies: true}
}

func TestDeliverySizesScenariosAndProposesSlices(t *testing.T) {
	o := project(t, "aaaaaaa1", "bbbbbbb2", "ccccccc3")
	m := measurer{
		"aaaaaaa1": {{Path: "discount.go", Added: 200, Deleted: 20}, {Path: "go.sum", Added: 900}},
		"bbbbbbb2": {{Path: "discount.go", Added: 150}},
		"ccccccc3": {{Path: "stack.go", Added: 120}},
	}
	tr, err := Build(context.Background(), fsys.OS{}, m, o)
	if err != nil {
		t.Fatal(err)
	}
	if got := []int{tr.Scenarios[0].Lines, tr.Scenarios[1].Lines, tr.Scenarios[2].Lines}; got[0] != 220 || got[1] != 150 || got[2] != 120 {
		t.Fatalf("lines = %v (go.sum excluded)", got)
	}
	if len(tr.Slices) != 2 || len(tr.Slices[0].Scenarios) != 2 || len(tr.Slices[1].Scenarios) != 1 {
		t.Fatalf("slices = %+v", tr.Slices)
	}
	paths, err := Write(fsys.OS{}, o, tr)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 5 || !strings.HasSuffix(paths[4], "PR_BODY-2.md") {
		t.Fatalf("paths = %v", paths)
	}
	body, _ := os.ReadFile(paths[4])
	if strings.Contains(string(body), "A valid code") || !strings.Contains(string(body), "Stacking") {
		t.Fatalf("a slice body has only its scenarios:\n%s", body)
	}
}

func TestAMissingCommitIsAnError(t *testing.T) {
	o := project(t, "aaaaaaa1", "bbbbbbb2", "gone0000")
	_, err := Build(context.Background(), fsys.OS{}, measurer{"aaaaaaa1": nil, "bbbbbbb2": nil}, o)
	if err == nil || !strings.Contains(err.Error(), "scenario 3 (gone0000)") {
		t.Fatalf("a commit that cannot be measured is said, not hidden: %v", err)
	}
}
