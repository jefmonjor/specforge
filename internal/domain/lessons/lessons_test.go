package lessons

import (
	"fmt"
	"strings"
	"testing"
)

func TestAddDedupesCapsAndFilters(t *testing.T) {
	md, added := Add("", Lesson{"go", "Inject a Clock instead of calling time.Now in the domain."})
	if !added || !strings.HasPrefix(md, "# Lessons") {
		t.Fatalf("first add:\n%s", md)
	}
	if _, added := Add(md, Lesson{"go", "inject a clock   instead of calling time.Now in the domain"}); added {
		t.Fatal("an equivalent lesson must not be added twice")
	}
	md, _ = Add(md, Lesson{"node", "Await every promise in tests."})
	if got := For(md, "go"); got != "- Inject a Clock instead of calling time.Now in the domain." {
		t.Fatalf("For(go) = %q", got)
	}
	for i := range Max + 5 {
		md, _ = Add(md, Lesson{"go", fmt.Sprintf("rule %d", i)})
	}
	all := Parse(md)
	if len(all) != Max || all[len(all)-1].Text != fmt.Sprintf("rule %d", Max+4) {
		t.Fatalf("kept %d, last %+v", len(all), all[len(all)-1])
	}
	if _, added := Add(md, Lesson{"go", "  "}); added {
		t.Fatal("an empty lesson must be ignored")
	}
}
