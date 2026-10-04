package jsontext

import (
	"reflect"
	"testing"
)

func TestCandidatesOrder(t *testing.T) {
	s := "intro {\"a\":1}\n```json\n{\"b\":2}\n```\ntext\n```\n[1,2]\n```\nend {\"c\":3}"
	got := Candidates(s)
	want := []string{"[1,2]", `{"b":2}`, `{"c":3}`, `{"a":1}`}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Candidates = %q, want %q", got, want)
	}
}

func TestDecodeWithPredicate(t *testing.T) {
	type v struct{ Status string }
	got, ok := Decode("```json\n{\"status\":\"x\"}\n```\n{\"status\":\"done\"}", func(x v) bool { return x.Status == "x" })
	if !ok || got.Status != "x" {
		t.Fatalf("Decode = %+v %v", got, ok)
	}
	if _, ok := Decode[v]("no json here", nil); ok {
		t.Fatal("no candidate expected")
	}
}

func FuzzCandidates(f *testing.F) {
	f.Add("```json\n{}\n```")
	f.Add("{[}]")
	f.Fuzz(func(t *testing.T, s string) {
		for _, c := range Candidates(s) {
			if c == "" {
				t.Fatal("empty candidate")
			}
		}
	})
}
