package change

import "testing"

func TestGeneratedFilesAreNotAuthored(t *testing.T) {
	cases := map[string]bool{
		"go.sum": true, "web/package-lock.json": true, "pnpm-lock.yaml": true, "app/Gemfile.lock": true,
		"vendor/x/y.go": true, "pkg/testdata/golden/out.txt": true, "node_modules/a/index.js": true,
		"go.mod": false, "src/lock.go": false, "internal/pay/net.go": false, "docs/vendor.md": false,
	}
	for p, want := range cases {
		if got := Generated(p); got != want {
			t.Errorf("Generated(%q) = %v, want %v", p, got, want)
		}
	}
}

func TestTotalCountsAuthoredLines(t *testing.T) {
	files := []File{{Path: "a.go", Added: 10, Deleted: 2}, {Path: "go.sum", Added: 300}, {Path: "logo.png", Binary: true}}
	if got := Total(files); got != 312 {
		t.Fatalf("Total = %d", got)
	}
	if got := Total(Authored(files)); got != 12 {
		t.Fatalf("authored total = %d", got)
	}
}

func TestFromContent(t *testing.T) {
	if f := FromContent("a", []byte("x\ny")); f.Added != 2 {
		t.Fatalf("a last line without newline counts: %+v", f)
	}
	if f := FromContent("a", nil); f.Added != 0 || f.Binary {
		t.Fatalf("empty file: %+v", f)
	}
	if f := FromContent("a.png", []byte{1, 0, 2}); !f.Binary || f.Added != 0 {
		t.Fatalf("binary: %+v", f)
	}
}
