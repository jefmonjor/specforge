package risk

import (
	"strings"
	"testing"

	"github.com/jefmonjor/specforge/v6/internal/domain/change"
)

func files(spec ...string) []change.File {
	var out []change.File
	for _, s := range spec {
		out = append(out, change.File{Path: s, Added: 10, Deleted: 2})
	}
	return out
}

func TestClassify(t *testing.T) {
	big := []change.File{{Path: "internal/pay/net.go", Added: 380, Deleted: 30}}
	cases := []struct {
		name  string
		files []change.File
		want  Tier
		why   string
	}{
		{"nothing changed", nil, Passive, "no file changed"},
		{"readme", files("README.md"), Passive, "documentation only"},
		{"docs tree", files("docs/guide/setup.md", "docs/media/flow.svg"), Passive, "documentation only"},
		{"plain code", files("internal/pay/net.go", "internal/pay/net_test.go"), Medium, "code changed: 24 line(s) in 2 file(s)"},
		{"docs and code", files("README.md", "src/app.ts"), Medium, "code changed"},
		{"auth directory", files("internal/auth/login.go"), High, "`internal/auth/login.go` is a sensitive path"},
		{"token file", files("src/token.ts"), High, "sensitive path"},
		{"password package", files("app/passwords/reset.py"), High, "sensitive path"},
		{"migration", files("db/migrations/0003_add_index.sql"), High, "sensitive path"},
		{"process execution", files("internal/process/runner.go"), High, "sensitive path"},
		{"go.mod", files("go.mod"), High, "`go.mod` is a sensitive path"},
		{"package.json", files("web/package.json"), High, "sensitive path"},
		{"pom.xml", files("pom.xml"), High, "sensitive path"},
		{"gradle kts", files("build.gradle.kts"), High, "sensitive path"},
		{"pyproject", files("pyproject.toml"), High, "sensitive path"},
		{"dockerfile", files("deploy.Dockerfile", "Dockerfile.dev"), High, "sensitive path"},
		{"workflow", files(".github/workflows/release.yml"), High, "sensitive path"},
		{"case insensitive", files("src/Auth/Login.java"), High, "sensitive path"},
		{"too big", big, High, "410 changed lines > 400"},
		{"author is not auth", files("src/author/name.go"), Medium, "code changed"},
		{"lock file does not count for size", []change.File{{Path: "package-lock.json", Added: 5000}, {Path: "src/a.ts", Added: 3}}, Medium, "code changed: 3 line(s)"},
		{"binary", []change.File{{Path: "assets/logo.png", Binary: true}}, Medium, "`assets/logo.png` is binary"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := Classify(c.files, DefaultRules())
			if a.Tier != c.want || !strings.Contains(strings.Join(a.Reasons, "; "), c.why) {
				t.Fatalf("Classify = %s %q, want %s with %q", a.Tier, a.Reasons, c.want, c.why)
			}
		})
	}
}

func TestEscalateOnlyRaises(t *testing.T) {
	a := Classify(files("README.md"), DefaultRules())
	up := a.Escalate(High, "the doc describes the token format clients rely on")
	if up.Tier != High || !strings.Contains(strings.Join(up.Reasons, ";"), "raised by the agent: ") || !strings.Contains(strings.Join(up.Reasons, ";"), "raised by the agent") {
		t.Fatalf("Escalate = %+v", up)
	}
	if down := up.Escalate(Passive, "nothing to see"); down.Tier != High {
		t.Fatal("the agent can never lower the tier")
	}
	if same := a.Escalate("critical", "x"); same.Tier != Passive {
		t.Fatal("an unknown tier changes nothing")
	}
}

func TestRulesAreConfigurable(t *testing.T) {
	r, err := NewRules(50, []string{`(^|/)ledger/`}, []string{`\.adr$`}, Medium)
	if err != nil {
		t.Fatal(err)
	}
	if a := Classify(files("ledger/post.go"), r); a.Tier != High {
		t.Fatalf("custom high path: %s", a.Tier)
	}
	if a := Classify(files("internal/auth/x.go"), r); a.Tier != Medium {
		t.Fatalf("custom lists replace the defaults: %s", a.Tier)
	}
	if a := Classify(files("decisions/001.adr"), r); a.Tier != Medium || !strings.Contains(strings.Join(a.Reasons, ";"), "floor is medium") {
		t.Fatalf("the floor lifts passive changes: %+v", a)
	}
	if _, err := NewRules(0, []string{"("}, nil, Passive); err == nil || !strings.Contains(err.Error(), "risk.high_paths") {
		t.Fatalf("an invalid pattern names its key: %v", err)
	}
}

func TestTiers(t *testing.T) {
	if !High.AtLeast(Medium) || Passive.AtLeast(Medium) {
		t.Fatal("tier order")
	}
	if tier, err := ParseTier(" HIGH "); err != nil || tier != High {
		t.Fatalf("ParseTier = %v %v", tier, err)
	}
	if tier, err := ParseTier(""); err != nil || tier != Passive {
		t.Fatal("empty is the floor")
	}
	if _, err := ParseTier("extreme"); err == nil {
		t.Fatal("unknown tier")
	}
}
