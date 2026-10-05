package review

import (
	"reflect"
	"testing"

	"specforge/internal/domain/risk"
)

func TestSelect(t *testing.T) {
	rules, err := risk.NewRules(0, []string{`auth`}, []string{`\.(md|yaml)$`}, risk.Passive)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		tier  risk.Tier
		files []string
		want  []Lens
	}{
		{risk.Passive, []string{"README.md"}, nil},
		{risk.High, []string{"a.go"}, AllLenses},
		{risk.Medium, []string{"auth/x.go"}, []Lens{LensRisk}},
		{risk.Medium, []string{"pay/net.go"}, []Lens{LensReliability}},
		{risk.Medium, []string{"config.yaml"}, []Lens{LensReadability}},
	}
	for _, c := range cases {
		if got := Select(c.tier, c.files, rules); !reflect.DeepEqual(got, c.want) {
			t.Errorf("Select(%s, %v) = %v, want %v", c.tier, c.files, got, c.want)
		}
	}
}

func TestParseLenses(t *testing.T) {
	if l, auto, err := ParseLenses(nil); l != nil || !auto || err != nil {
		t.Fatal("empty is auto")
	}
	if l, auto, err := ParseLenses([]string{"off"}); len(l) != 0 || l == nil || auto || err != nil {
		t.Fatal("off is no lens")
	}
	if l, _, err := ParseLenses([]string{"Risk", "resilience", "risk"}); err != nil || !reflect.DeepEqual(l, []Lens{LensRisk, LensResilience}) {
		t.Fatalf("list = %v %v", l, err)
	}
	if _, _, err := ParseLenses([]string{"style"}); err == nil {
		t.Fatal("unknown lens")
	}
}
