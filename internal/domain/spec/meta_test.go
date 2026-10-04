package spec

import (
	"strings"
	"testing"
)

const withFrontMatter = `---
id: "0001"
title: "Password reset"
status: draft
created: "2026-10-04"
approved_by: ""
approved_at: ""
---

# 0001 · Password reset
`

func TestReadMeta(t *testing.T) {
	m, ok, err := ReadMeta(withFrontMatter)
	if err != nil || !ok {
		t.Fatalf("ReadMeta: ok=%v err=%v", ok, err)
	}
	if m.ID != "0001" || m.Title != "Password reset" || m.Status != StatusDraft || m.Created != "2026-10-04" {
		t.Fatalf("unexpected meta %+v", m)
	}
}

func TestReadMetaWithoutFrontMatter(t *testing.T) {
	_, ok, err := ReadMeta("# Title\n\nbody\n")
	if ok || err != nil {
		t.Fatalf("want no front matter, got ok=%v err=%v", ok, err)
	}
}

func TestSetMetaReplacesAndKeepsTheRest(t *testing.T) {
	out, err := SetMeta(withFrontMatter, map[string]string{
		"status":      "approved",
		"approved_by": `Ana "QA" López`,
	})
	if err != nil {
		t.Fatal(err)
	}
	m, _, err := ReadMeta(out)
	if err != nil {
		t.Fatal(err)
	}
	if m.Status != StatusApproved || m.ApprovedBy != `Ana "QA" López` || m.ID != "0001" {
		t.Fatalf("unexpected meta %+v", m)
	}
	if !strings.HasSuffix(out, "# 0001 · Password reset\n") {
		t.Fatalf("body changed:\n%s", out)
	}
}

func TestSetMetaAddsMissingKeysInOrder(t *testing.T) {
	out, err := SetMeta("---\nid: \"7\"\n---\n# T\n", map[string]string{"b": "2", "a": "1"}, "b")
	if err != nil {
		t.Fatal(err)
	}
	want := "---\nid: \"7\"\nb: \"2\"\na: \"1\"\n---\n# T\n"
	if out != want {
		t.Fatalf("got\n%q\nwant\n%q", out, want)
	}
}

func TestSetMetaCreatesFrontMatter(t *testing.T) {
	out, err := SetMeta("# T\n", map[string]string{"status": "approved"})
	if err != nil {
		t.Fatal(err)
	}
	if out != "---\nstatus: \"approved\"\n---\n# T\n" {
		t.Fatalf("got %q", out)
	}
}
