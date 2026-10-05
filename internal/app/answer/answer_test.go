package answer

import "testing"

func TestDecodeValidatesAgainstTheSchema(t *testing.T) {
	s, err := Load("review/validate-schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var v struct {
		Results []struct{ ID, Status string } `json:"results"`
	}
	good := "Checked.\n```json\n{\"results\":[{\"id\":\"REL-001\",\"status\":\"resolved\",\"reason\":\"net is clamped\"}]}\n```"
	if !s.Decode(good, &v) || v.Results[0].Status != "resolved" {
		t.Fatalf("valid answer: %+v", v)
	}
	for _, bad := range []string{
		"all good, trust me",
		"```json\n{\"results\":[{\"id\":\"REL-001\",\"status\":\"fine\",\"reason\":\"net is clamped\"}]}\n```",
		"```json\n{\"results\":[], \"extra\": 1}\n```",
	} {
		if s.Decode(bad, &v) {
			t.Errorf("accepted %q", bad)
		}
	}
	if again, _ := Load("review/validate-schema.json"); again != s {
		t.Error("schemas are compiled once")
	}
	if _, err := Load("review/missing.json"); err == nil {
		t.Error("a missing schema is an error")
	}
}
