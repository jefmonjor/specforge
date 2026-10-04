package protocol

import (
	"errors"
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	cases := []struct {
		name   string
		answer string
		want   Response
	}{
		{
			"fenced block at the end",
			"I wrote the test.\n\n```json\n{\"status\": \"done\", \"files_written\": [\"./pkg\\\\reset_test.go\"], \"summary\": \"x\"}\n```\n",
			Response{Status: Done, FilesWritten: []string{"pkg/reset_test.go"}, Summary: "x"},
		},
		{
			"bare object after prose",
			"Done. {\"status\":\"DONE\"}",
			Response{Status: Done},
		},
		{
			"the last block wins over a quoted example",
			"The contract looks like ```json\n{\"status\":\"blocked\",\"reason\":\"example\"}\n```\nmy answer:\n```json\n{\"status\":\"needs_clarification\",\"question\":\"From request or delivery time?\",\"options\":[\"request\",\"delivery\"]}\n```",
			Response{Status: NeedsClarification, Question: "From request or delivery time?", Options: []string{"request", "delivery"}},
		},
		{
			"blocked",
			"```json\n{\"status\":\"blocked\",\"reason\":\"pytest missing\",\"suggested_action\":\"pip install pytest\"}\n```",
			Response{Status: Blocked, Reason: "pytest missing", SuggestedAction: "pip install pytest"},
		},
		{
			"nested braces in prose before the object",
			"func f() { if x { y() } }\n{\"status\":\"done\",\"summary\":\"uses {braces}\"}",
			Response{Status: Done, Summary: "uses {braces}"},
		},
	}
	for _, c := range cases {
		got, err := Parse(c.answer)
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s:\n got %+v\nwant %+v", c.name, got, c.want)
		}
	}
}

func TestParseRejectsMissingOrInvalidContracts(t *testing.T) {
	for _, answer := range []string{
		"I implemented everything, all tests pass.",
		"```json\n{\"status\":\"finished\"}\n```",
		"```json\n{\"status\":\"needs_clarification\"}\n```",
		"```json\n{\"status\":\"blocked\"}\n```",
		"",
	} {
		if _, err := Parse(answer); !errors.Is(err, ErrNoContract) {
			t.Errorf("Parse(%q) = %v, want ErrNoContract", answer, err)
		}
	}
}

func FuzzParse(f *testing.F) {
	f.Add("```json\n{\"status\":\"done\"}\n```")
	f.Add("{\"status\":\"blocked\",\"reason\":\"x\"}")
	f.Fuzz(func(t *testing.T, s string) {
		r, err := Parse(s)
		if err != nil {
			return
		}
		switch r.Status {
		case Done, NeedsClarification, Blocked:
		default:
			t.Fatalf("invalid status accepted: %q", r.Status)
		}
	})
}
