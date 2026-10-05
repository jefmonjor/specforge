package deliver

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jefmonjor/specforge/v6/internal/adapters/fsys"
)

func TestTestNames(t *testing.T) {
	dir := t.TempDir()
	cases := map[string]string{
		"a_test.go":    "func TestSDD_0001_002_Expired(t *testing.T) {}\n// SDD_0001_002 again in a comment\nfunc TestSDD_0001_002_Expired(t *testing.T) {}\n",
		"a.test.ts":    "it(\"SDD_0001_002 refuses an expired link\", () => {})\n",
		"Test.java":    "@Test void SDD_0001_002_refusesExpiredLink() {}\n",
		"test_a.py":    "def test_SDD_0001_002_expired():\n",
		"none_test.go": "func TestOther(t *testing.T) {}\n",
	}
	want := map[string][]string{
		"a_test.go":    {"TestSDD_0001_002_Expired"},
		"a.test.ts":    {"SDD_0001_002 refuses an expired link"},
		"Test.java":    {"SDD_0001_002_refusesExpiredLink"},
		"test_a.py":    {"test_SDD_0001_002_expired"},
		"none_test.go": nil,
	}
	for name, content := range cases {
		p := filepath.Join(dir, name)
		os.WriteFile(p, []byte(content), 0o644)
		got := testNames(fsys.OS{}, p, "SDD_0001_002")
		if len(got) != len(want[name]) {
			t.Errorf("%s: got %q, want %q", name, got, want[name])
			continue
		}
		for i := range got {
			if got[i] != want[name][i] {
				t.Errorf("%s: got %q, want %q", name, got, want[name])
			}
		}
	}
}
