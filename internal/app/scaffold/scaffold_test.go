package scaffold

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"specforge/internal/adapters/fsys"
)

func TestNewData(t *testing.T) {
	d, err := NewData("Payroll Service")
	if err != nil || d.Name != "payroll-service" || d.Package != "com.example.payrollservice" || d.Module != "payroll_service" || d.Title != "Payroll service" {
		t.Fatalf("%+v %v", d, err)
	}
	if d, _ := NewData("2fa"); d.Name != "app-2fa" {
		t.Fatalf("%+v", d)
	}
	if _, err := NewData("!!!"); err == nil {
		t.Fatal("a name without letters")
	}
}

func TestEveryScaffoldRenders(t *testing.T) {
	d, _ := NewData("payroll")
	want := map[string][]string{
		"java":   {"pom.xml", "src/test/java/com/example/payroll/ArchitectureTest.java", "src/main/java/com/example/payroll/domain/package-info.java"},
		"react":  {"package.json", "src/App.test.tsx", "stryker.config.json", "knip.json", "eslint.config.js"},
		"python": {"pyproject.toml", "src/payroll/__init__.py", "tests/test_package.py"},
		"go":     {"go.mod", ".golangci.yml", "main.go"},
	}
	for _, kind := range Kinds {
		root := t.TempDir()
		changes, err := Create(fsys.OS{}, root, kind, d)
		if err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
		for _, rel := range want[kind] {
			data, err := os.ReadFile(filepath.Join(root, rel))
			if err != nil {
				t.Fatalf("%s: %s missing (%v)", kind, rel, changes)
			}
			if strings.Contains(string(data), "[[") || strings.Contains(string(data), "__PKG__") {
				t.Errorf("%s/%s not rendered", kind, rel)
			}
		}
		if _, err := Create(fsys.OS{}, root, kind, d); !errors.Is(err, ErrExistingProject) {
			t.Errorf("%s twice: %v", kind, err)
		}
	}
}

func TestCreateKeepsExistingFilesAndRefusesUnknownKinds(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, ".gitignore"), []byte("mine\n"), 0o644)
	d, _ := NewData("x")
	changes, err := Create(fsys.OS{}, root, "go", d)
	if err != nil {
		t.Fatal(err)
	}
	kept := false
	for _, c := range changes {
		kept = kept || (c.Path == ".gitignore" && c.Action == Kept)
	}
	if data, _ := os.ReadFile(filepath.Join(root, ".gitignore")); !kept || string(data) != "mine\n" {
		t.Fatalf("changes %v", changes)
	}
	if _, err := Create(fsys.OS{}, t.TempDir(), "cobol", d); err == nil {
		t.Fatal("unknown kind")
	}
}
