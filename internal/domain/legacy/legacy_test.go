package legacy

import (
	"strings"
	"testing"
	"testing/fstest"
)

func file(s string) *fstest.MapFile { return &fstest.MapFile{Data: []byte(s)} }

var legacyRepo = fstest.MapFS{
	"pom.xml": file(`<project><build><plugins><plugin><artifactId>maven-compiler-plugin</artifactId>
<configuration><source>1.6</source><target>1.6</target></configuration></plugin></plugins></build></project>`),
	"src/main/java/com/acme/payroll/PayrollServlet.java":  file("package com.acme.payroll;\nimport javax.servlet.http.HttpServlet;\nimport java.util.Vector;\nimport org.apache.log4j.Logger;\npublic class PayrollServlet extends HttpServlet {}\n"),
	"src/main/java/com/acme/payroll/dao/EmployeeDao.java": file("package com.acme.payroll.dao;\nimport java.sql.Connection;\nimport java.util.Date;\npublic class EmployeeDao {}\n"),
	"src/main/java/com/acme/billing/Invoice.java":         file("package com.acme.billing;\npublic class Invoice {}\n"),
	"src/test/java/com/acme/payroll/PayrollTest.java":     file("package com.acme.payroll;\nimport junit.framework.TestCase;\npublic class PayrollTest extends TestCase {}\n"),
	"src/main/webapp/payroll.jsp":                         file("<html></html>\n"),
	"target/classes/Ignored.java":                         file("import org.hibernate.Session;\n"),
}

func TestScan(t *testing.T) {
	inv, err := Scan(legacyRepo)
	if err != nil {
		t.Fatal(err)
	}
	if inv.Build != "maven" || inv.JavaRelease != "1.6" || inv.SourceFiles != 3 || inv.TestFiles != 1 {
		t.Fatalf("inventory %+v", inv)
	}
	for _, id := range []string{"servlet", "jsp", "jdbc", "log4j1", "junit3", "legacycollections", "legacydates"} {
		if !inv.Has(id) {
			t.Errorf("missing %s in %+v", id, inv.Frameworks)
		}
	}
	if inv.Has("hibernate") {
		t.Error("build output must be ignored")
	}
	if len(inv.Packages) != 2 || inv.Packages[0].Name != "com.acme.payroll" || inv.Packages[0].Files != 2 {
		t.Fatalf("packages %+v", inv.Packages)
	}
}

func TestReleaseNumberAndGradleAnt(t *testing.T) {
	for in, want := range map[string]int{"1.6": 6, "1.8": 8, "11": 11, "21": 21, "17.0.2": 17, "x": 0} {
		if got := ReleaseNumber(in); got != want {
			t.Errorf("ReleaseNumber(%q) = %d, want %d", in, got, want)
		}
	}
	g, _ := Scan(fstest.MapFS{"build.gradle": file("sourceCompatibility = JavaVersion.VERSION_1_7\n")})
	a, _ := Scan(fstest.MapFS{"build.xml": file(`<javac srcdir="src" source="1.5" target="1.5"/>`)})
	if g.Build != "gradle" || g.JavaRelease != "1.7" || a.Build != "ant" || a.JavaRelease != "1.5" {
		t.Fatalf("gradle %q %q · ant %q %q", g.Build, g.JavaRelease, a.Build, a.JavaRelease)
	}
}

func TestMarkdown(t *testing.T) {
	inv, _ := Scan(legacyRepo)
	md := inv.Markdown("es", "../payroll")
	for _, want := range []string{"# Inventario legacy · `../payroll`", "**Release de Java declarada**: 1.6", "**Servlet API (javax.servlet)**: 1", "Java 1.6 → 21", "jakarta.servlet", "`com.acme.payroll`: 2"} {
		if !strings.Contains(md, want) {
			t.Errorf("missing %q in:\n%s", want, md)
		}
	}
	if en := inv.Markdown("en", "x"); !strings.Contains(en, "Java 11 removed Java EE") {
		t.Errorf("english notes:\n%s", en)
	}
}
