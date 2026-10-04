// Package legacy reads an existing codebase to plan its migration: what it
// is built with, which Java release it targets, which frameworks it uses
// (from its imports, not guesses) and what each one means for a move to a
// modern stack. It is pure: it walks an fs.FS.
package legacy

import (
	"bufio"
	"io/fs"
	"path"
	"regexp"
	"slices"
	"strings"

	"specforge/internal/domain/stack"
)

// maxScannedFiles bounds a scan; the inventory says when it was reached.
const maxScannedFiles = 50_000

// Framework is a technology found through its imports or files.
type Framework struct {
	ID    string
	Name  string
	Files int
	// Example is one file that uses it.
	Example string
}

// Package is a top-level source package and its size.
type Package struct {
	Name  string
	Files int
}

// Inventory is what a codebase is made of.
type Inventory struct {
	Build       string // maven, gradle, ant or none
	JavaRelease string // "1.6", "8", "11"… as declared by the build; "" when unknown
	Files       map[string]int
	SourceFiles int
	TestFiles   int
	Lines       int
	Frameworks  []Framework
	Packages    []Package
	Truncated   bool
}

// marker recognises a technology by an import prefix or a file pattern.
type marker struct {
	id, name string
	imports  []string
	files    *regexp.Regexp
}

var markers = []marker{
	{id: "servlet", name: "Servlet API (javax.servlet)", imports: []string{"javax.servlet."}},
	{id: "jsp", name: "JSP views", files: regexp.MustCompile(`\.jspx?$`)},
	{id: "struts1", name: "Struts 1", imports: []string{"org.apache.struts."}},
	{id: "struts2", name: "Struts 2", imports: []string{"org.apache.struts2."}},
	{id: "ejb", name: "EJB (javax.ejb)", imports: []string{"javax.ejb."}},
	{id: "jpa", name: "JPA (javax.persistence)", imports: []string{"javax.persistence."}},
	{id: "hibernate", name: "Hibernate", imports: []string{"org.hibernate."}},
	{id: "spring", name: "Spring Framework", imports: []string{"org.springframework."}},
	{id: "jdbc", name: "Plain JDBC", imports: []string{"java.sql.", "javax.sql."}},
	{id: "jaxrpc", name: "SOAP with JAX-RPC / Axis 1", imports: []string{"javax.xml.rpc.", "org.apache.axis."}},
	{id: "jaxws", name: "SOAP with JAX-WS (javax)", imports: []string{"javax.jws.", "javax.xml.ws."}},
	{id: "jaxb", name: "JAXB (javax.xml.bind)", imports: []string{"javax.xml.bind."}},
	{id: "validation", name: "Bean Validation (javax)", imports: []string{"javax.validation."}},
	{id: "jms", name: "JMS (javax.jms)", imports: []string{"javax.jms."}},
	{id: "log4j1", name: "Log4j 1.x", imports: []string{"org.apache.log4j."}},
	{id: "commonslogging", name: "Commons Logging", imports: []string{"org.apache.commons.logging."}},
	{id: "junit3", name: "JUnit 3", imports: []string{"junit.framework."}},
	{id: "junit4", name: "JUnit 4", imports: []string{"org.junit.Test", "org.junit.Assert", "org.junit.Before"}},
	{id: "legacycollections", name: "Vector / Hashtable / Enumeration", imports: []string{"java.util.Vector", "java.util.Hashtable", "java.util.Enumeration"}},
	{id: "legacydates", name: "java.util.Date / Calendar / SimpleDateFormat", imports: []string{"java.util.Date", "java.util.Calendar", "java.text.SimpleDateFormat"}},
}

var (
	importLine  = regexp.MustCompile(`^\s*import\s+(?:static\s+)?([\w.]+(?:\.\*)?)\s*;`)
	packageLine = regexp.MustCompile(`^\s*package\s+([\w.]+)\s*;`)
	sourceExt   = map[string]bool{".java": true, ".kt": true, ".groovy": true, ".scala": true, ".js": true, ".ts": true, ".tsx": true, ".jsx": true, ".py": true, ".go": true, ".cs": true, ".php": true, ".rb": true, ".cbl": true, ".cob": true}
)

// Scan builds the inventory of the codebase in fsys.
func Scan(fsys fs.FS) (Inventory, error) {
	inv := Inventory{Files: map[string]int{}, Build: build(fsys)}
	inv.JavaRelease = javaRelease(fsys)
	found := map[string]*Framework{}
	packages := map[string]int{}
	scanned := 0

	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // unreadable entries are skipped, as a reader would
		}
		if d.IsDir() {
			if p != "." && (stack.IgnoredDirs[d.Name()] || strings.HasPrefix(d.Name(), ".")) {
				return fs.SkipDir
			}
			return nil
		}
		if scanned++; scanned > maxScannedFiles {
			inv.Truncated = true
			return fs.SkipAll
		}
		ext := strings.ToLower(path.Ext(p))
		inv.Files[strings.TrimPrefix(ext, ".")]++
		for _, m := range markers {
			if m.files != nil && m.files.MatchString(p) {
				note(found, m, p)
			}
		}
		if !sourceExt[ext] {
			return nil
		}
		if isTest(p) {
			inv.TestFiles++
		} else {
			inv.SourceFiles++
		}
		if ext != ".java" {
			lines, _ := countLines(fsys, p)
			inv.Lines += lines
			return nil
		}
		pkg, imports, lines := readJava(fsys, p)
		inv.Lines += lines
		if pkg != "" && !isTest(p) {
			packages[topPackage(pkg)]++
		}
		seen := map[string]bool{}
		for _, imp := range imports {
			for _, m := range markers {
				if seen[m.id] {
					continue
				}
				for _, prefix := range m.imports {
					if strings.HasPrefix(imp, prefix) {
						note(found, m, p)
						seen[m.id] = true
						break
					}
				}
			}
		}
		return nil
	})
	for _, m := range markers {
		if f, ok := found[m.id]; ok {
			inv.Frameworks = append(inv.Frameworks, *f)
		}
	}
	for name, n := range packages {
		inv.Packages = append(inv.Packages, Package{Name: name, Files: n})
	}
	slices.SortFunc(inv.Packages, func(a, b Package) int {
		if a.Files != b.Files {
			return b.Files - a.Files
		}
		return strings.Compare(a.Name, b.Name)
	})
	return inv, err
}

// Has reports whether a framework was found.
func (inv Inventory) Has(id string) bool {
	return slices.ContainsFunc(inv.Frameworks, func(f Framework) bool { return f.ID == id })
}

func note(found map[string]*Framework, m marker, p string) {
	f, ok := found[m.id]
	if !ok {
		f = &Framework{ID: m.id, Name: m.name, Example: p}
		found[m.id] = f
	}
	f.Files++
}

func isTest(p string) bool {
	lower := strings.ToLower(p)
	base := path.Base(lower)
	return strings.Contains(lower, "src/test/") || strings.Contains(lower, "/test/") || strings.HasPrefix(lower, "test/") ||
		strings.HasSuffix(base, "test.java") || strings.HasSuffix(base, "tests.java") || strings.HasPrefix(base, "test_") || strings.Contains(base, ".test.")
}

// topPackage keeps the first three segments: com.acme.billing.
func topPackage(pkg string) string {
	parts := strings.Split(pkg, ".")
	if len(parts) > 3 {
		parts = parts[:3]
	}
	return strings.Join(parts, ".")
}

func readJava(fsys fs.FS, p string) (pkg string, imports []string, lines int) {
	f, err := fsys.Open(p)
	if err != nil {
		return "", nil, 0
	}
	defer func() { _ = f.Close() }() // read only
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		lines++
		line := sc.Text()
		if m := packageLine.FindStringSubmatch(line); m != nil && pkg == "" {
			pkg = m[1]
		} else if m := importLine.FindStringSubmatch(line); m != nil {
			imports = append(imports, m[1])
		}
	}
	return pkg, imports, lines
}

func countLines(fsys fs.FS, p string) (int, error) {
	data, err := fs.ReadFile(fsys, p)
	if err != nil {
		return 0, err
	}
	return strings.Count(string(data), "\n"), nil
}

func build(fsys fs.FS) string {
	for _, b := range []struct{ file, name string }{
		{"pom.xml", "maven"}, {"build.gradle", "gradle"}, {"build.gradle.kts", "gradle"}, {"build.xml", "ant"},
	} {
		if _, err := fs.Stat(fsys, b.file); err == nil {
			return b.name
		}
	}
	return "none"
}

var releasePatterns = []*regexp.Regexp{
	regexp.MustCompile(`<maven\.compiler\.(?:release|source|target)>\s*([\d.]+)\s*<`),
	regexp.MustCompile(`<java\.version>\s*([\d.]+)\s*<`),
	regexp.MustCompile(`<(?:release|source|target)>\s*([\d.]+)\s*</(?:release|source|target)>`),
	regexp.MustCompile(`(?:sourceCompatibility|targetCompatibility)\s*=\s*['"]?(?:JavaVersion\.VERSION_)?([\d._]+)`),
	regexp.MustCompile(`<javac[^>]*\b(?:source|target)\s*=\s*"([\d.]+)"`),
	regexp.MustCompile(`name="(?:javac\.source|ant\.build\.javac\.source|java\.version)"\s+value="([\d.]+)"`),
}

// javaRelease reads the release the build declares; the lowest wins, since
// that is what the code must still compile for.
func javaRelease(fsys fs.FS) string {
	var found []string
	for _, f := range []string{"pom.xml", "build.gradle", "build.gradle.kts", "build.xml"} {
		data, err := fs.ReadFile(fsys, f)
		if err != nil {
			continue
		}
		for _, re := range releasePatterns {
			for _, m := range re.FindAllStringSubmatch(string(data), -1) {
				found = append(found, strings.ReplaceAll(m[1], "_", "."))
			}
		}
	}
	if len(found) == 0 {
		return ""
	}
	slices.SortFunc(found, func(a, b string) int { return ReleaseNumber(a) - ReleaseNumber(b) })
	return found[0]
}

// ReleaseNumber turns "1.6" into 6 and "21" into 21; 0 when unreadable.
func ReleaseNumber(v string) int {
	v = strings.TrimPrefix(strings.TrimSpace(v), "1.")
	n := 0
	for _, r := range v {
		if r < '0' || r > '9' {
			break
		}
		n = n*10 + int(r-'0')
	}
	return n
}
