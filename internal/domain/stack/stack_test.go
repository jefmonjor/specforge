package stack

import (
	"testing"
	"testing/fstest"
)

func file(s string) *fstest.MapFile { return &fstest.MapFile{Data: []byte(s)} }

func TestDetectSingleStacks(t *testing.T) {
	cases := []struct {
		name  string
		files fstest.MapFS
		want  Profile
	}{
		{"go", fstest.MapFS{"go.mod": file("module x")}, Profile{Kind: Go, Runner: RunnerGo, Standard: "go"}},
		{"maven", fstest.MapFS{"pom.xml": file("<project/>")}, Profile{Kind: Maven, Runner: RunnerMaven, Standard: "java"}},
		{"gradle kts", fstest.MapFS{"build.gradle.kts": file("")}, Profile{Kind: Gradle, Runner: RunnerGradle, Standard: "java"}},
		{"python", fstest.MapFS{"pyproject.toml": file("")}, Profile{Kind: Python, Runner: RunnerPytest, Standard: "python"}},
		{"vitest + react", fstest.MapFS{"package.json": file(`{"dependencies":{"react":"19"},"devDependencies":{"vitest":"3"}}`)},
			Profile{Kind: Node, Runner: RunnerVitest, Framework: "react", Standard: "node"}},
		{"jest via script", fstest.MapFS{"package.json": file(`{"scripts":{"test":"jest --ci"}}`)},
			Profile{Kind: Node, Runner: RunnerJest, Standard: "node"}},
		{"angular", fstest.MapFS{"package.json": file(`{"dependencies":{"@angular/core":"18","react":"x"}}`)},
			Profile{Kind: Node, Runner: RunnerNPM, Framework: "angular", Standard: "node"}},
		{"broken package.json", fstest.MapFS{"package.json": file(`{`)}, Profile{Kind: Node, Runner: RunnerNPM, Standard: "node"}},
	}
	for _, c := range cases {
		got := Detect(c.files)
		if len(got) != 1 || got[0] != c.want {
			t.Errorf("%s: Detect = %+v, want [%+v]", c.name, got, c.want)
		}
	}
}

func TestDetectAmbiguousProjectReturnsEveryCandidate(t *testing.T) {
	// Regression: go.mod + package.json used to be "Go" without asking.
	got := Detect(fstest.MapFS{"go.mod": file(""), "package.json": file(`{}`)})
	if len(got) != 2 || got[0].Kind != Go || got[1].Kind != Node {
		t.Fatalf("Detect = %+v", got)
	}
	if p, ok := ByName(got, " Node "); !ok || p.Kind != Node {
		t.Fatalf("ByName = %+v %v", p, ok)
	}
	if _, ok := ByName(got, "python"); ok {
		t.Fatal("python is not a candidate")
	}
}

func TestDetectEmptyProject(t *testing.T) {
	if got := Detect(fstest.MapFS{"README.md": file("")}); len(got) != 0 {
		t.Fatalf("Detect = %+v", got)
	}
}

func TestIsTestFile(t *testing.T) {
	cases := []struct {
		kind Kind
		path string
		want bool
	}{
		{Go, "internal/auth/reset_test.go", true},
		{Go, "internal/auth/reset.go", false},
		{Maven, "src/test/java/com/x/ResetTest.java", true},
		{Maven, "src/main/java/com/x/Reset.java", false},
		{Gradle, "module/src/test/kotlin/ResetTest.kt", true},
		{Node, "src/reset.test.tsx", true},
		{Node, "src/__tests__/reset.ts", true},
		{Node, "src/reset.spec.mjs", true},
		{Node, "src/reset.ts", false},
		{Python, "tests/test_reset.py", true},
		{Python, "app/reset_test.py", true},
		{Python, "conftest.py", true},
		{Python, "app/reset.py", false},
		{Go, `internal\auth\reset_test.go`, true},
	}
	for _, c := range cases {
		if got := (Profile{Kind: c.kind}).IsTestFile(c.path); got != c.want {
			t.Errorf("%s IsTestFile(%q) = %v, want %v", c.kind, c.path, got, c.want)
		}
	}
}

func TestName(t *testing.T) {
	if got := (Profile{Kind: Node, Runner: RunnerVitest, Framework: "react"}).Name(); got != "node (vitest, react)" {
		t.Fatalf("Name = %q", got)
	}
}

func TestTestCommandAndMarkerExample(t *testing.T) {
	cases := []struct {
		p      Profile
		filter string
		cmd    string
	}{
		{Profile{Kind: Go, Runner: RunnerGo}, "SDD_0001_001", "go test -run SDD_0001_001 ./..."},
		{Profile{Kind: Go, Runner: RunnerGo}, "", "go test ./..."},
		{Profile{Kind: Maven, Runner: RunnerMaven}, "SDD_0001_001", "mvn test -Dtest='*SDD_0001_001*'"},
		{Profile{Kind: Gradle, Runner: RunnerGradle}, "SDD_0001_001", "gradle test --tests '*SDD_0001_001*'"},
		{Profile{Kind: Node, Runner: RunnerVitest}, "SDD_0001_001", "npx vitest run -t SDD_0001_001"},
		{Profile{Kind: Node, Runner: RunnerJest}, "", "npx jest"},
		{Profile{Kind: Node, Runner: RunnerNPM}, "SDD_0001_001", "npm test"},
		{Profile{Kind: Python, Runner: RunnerPytest}, "SDD_0001_001", "pytest -k SDD_0001_001"},
	}
	for _, c := range cases {
		if got := c.p.TestCommand(c.filter); got != c.cmd {
			t.Errorf("%s TestCommand(%q) = %q, want %q", c.p.Name(), c.filter, got, c.cmd)
		}
		if c.p.MarkerExample("SDD_0001_001") == "" {
			t.Errorf("%s has no marker example", c.p.Name())
		}
	}
}
