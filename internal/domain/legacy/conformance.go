package legacy

import (
	"bufio"
	"fmt"
	"io/fs"
	"path"
	"strings"

	"specforge/internal/domain/stack"
)

// Target is what the rewritten code must comply with.
type Target struct {
	// JavaRelease is the release the build must declare; 0 skips the check.
	JavaRelease int
	// ForbiddenImports are package or class prefixes the code may not
	// import ("javax.servlet", "org.apache.log4j", "java.util.Vector").
	ForbiddenImports []string
}

// Violation is one place where the code does not comply.
type Violation struct {
	Path string // "" for the build as a whole
	Line int
	Rule string
}

func (v Violation) String() string {
	if v.Path == "" {
		return v.Rule
	}
	return fmt.Sprintf("%s:%d: %s", v.Path, v.Line, v.Rule)
}

// Conformance checks the code in fsys against the target: the build must
// declare at least the target release, and no Java source (production or
// test) may import a forbidden package. Dependency and build directories
// are skipped, as in a scan.
func Conformance(fsys fs.FS, t Target) ([]Violation, error) {
	var out []Violation
	if t.JavaRelease > 0 {
		declared := javaRelease(fsys)
		switch n := ReleaseNumber(declared); {
		case declared == "":
			out = append(out, Violation{Rule: fmt.Sprintf("the build declares no Java release: set it to %d (maven.compiler.release, or the Gradle toolchain)", t.JavaRelease)})
		case n < t.JavaRelease:
			out = append(out, Violation{Rule: fmt.Sprintf("the build declares Java %s, the target is %d", declared, t.JavaRelease)})
		}
	}
	if len(t.ForbiddenImports) == 0 {
		return out, nil
	}
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != "." && (stack.IgnoredDirs[d.Name()] || strings.HasPrefix(d.Name(), ".")) {
				return fs.SkipDir
			}
			return nil
		}
		if path.Ext(p) != ".java" {
			return nil
		}
		v, err := forbiddenImports(fsys, p, t.ForbiddenImports)
		out = append(out, v...)
		return err
	})
	return out, err
}

func forbiddenImports(fsys fs.FS, p string, forbidden []string) ([]Violation, error) {
	f, err := fsys.Open(p)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }() // read only
	var out []Violation
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for n := 1; sc.Scan(); n++ {
		m := importLine.FindStringSubmatch(sc.Text())
		if m == nil {
			continue
		}
		for _, prefix := range forbidden {
			if imports(m[1], prefix) {
				out = append(out, Violation{Path: p, Line: n, Rule: fmt.Sprintf("imports %s (forbidden: %s)", m[1], prefix)})
				break
			}
		}
	}
	return out, sc.Err()
}

// imports reports whether an import names prefix or something inside it:
// "javax.servlet" forbids javax.servlet.http.HttpServlet but not
// javax.servletx.Foo.
func imports(imported, prefix string) bool {
	prefix = strings.TrimSuffix(strings.TrimSuffix(prefix, ".*"), ".")
	return imported == prefix || strings.HasPrefix(imported, prefix+".")
}
