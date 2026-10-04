package legacy

import (
	"strings"
	"testing"
	"testing/fstest"
)

func TestConformance(t *testing.T) {
	target := Target{JavaRelease: 21, ForbiddenImports: []string{"javax.servlet", "org.apache.log4j.*", "java.util.Vector"}}
	repo := fstest.MapFS{
		"pom.xml":                      file("<project><properties><maven.compiler.release>17</maven.compiler.release></properties></project>"),
		"src/main/java/a/Web.java":     file("package a;\nimport jakarta.servlet.http.HttpServlet;\nimport javax.servlet.http.HttpServlet;\nimport javax.servletx.Other;\nimport static org.apache.log4j.Level.INFO;\n"),
		"src/test/java/a/WebTest.java": file("package a;\nimport java.util.Vector;\nimport java.util.VectorUtils;\n"),
		"target/generated/Old.java":    file("import javax.servlet.Foo;\n"),
	}
	v, err := Conformance(repo, target)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, x := range v {
		got = append(got, x.String())
	}
	want := []string{
		"the build declares Java 17, the target is 21",
		"src/main/java/a/Web.java:3: imports javax.servlet.http.HttpServlet (forbidden: javax.servlet)",
		"src/main/java/a/Web.java:5: imports org.apache.log4j.Level.INFO (forbidden: org.apache.log4j.*)",
		"src/test/java/a/WebTest.java:2: imports java.util.Vector (forbidden: java.util.Vector)",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("got\n%s", strings.Join(got, "\n"))
	}

	repo["pom.xml"] = file("<project/>")
	v, _ = Conformance(repo, Target{JavaRelease: 21})
	if len(v) != 1 || !strings.Contains(v[0].Rule, "declares no Java release") {
		t.Fatalf("got %v", v)
	}
	delete(repo, "pom.xml")
	repo["build.gradle"] = file("java { toolchain { languageVersion = JavaLanguageVersion.of(21) } }\n")
	if v, _ := Conformance(repo, Target{JavaRelease: 21}); len(v) != 0 {
		t.Fatalf("toolchain 21: %v", v)
	}
}
