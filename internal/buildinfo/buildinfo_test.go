package buildinfo

import (
	"runtime/debug"
	"testing"
)

func TestGoInstallFillsWhatTheLinkFlagsLeft(t *testing.T) {
	Version, Commit, Date = "dev", "none", "unknown"
	t.Cleanup(func() { Version, Commit, Date = "dev", "none", "unknown" })
	fill(&debug.BuildInfo{
		Main:     debug.Module{Version: "v6.2.0"},
		Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "4e70e7f306580ad8"}, {Key: "vcs.time", Value: "2026-10-05T15:49:00Z"}},
	})
	if String() != "v6.2.0 (4e70e7f, 2026-10-05T15:49:00Z)" {
		t.Fatal(String())
	}
}

func TestLinkFlagsWin(t *testing.T) {
	Version, Commit, Date = "6.1.0", "abc1234", "today"
	t.Cleanup(func() { Version, Commit, Date = "dev", "none", "unknown" })
	fill(&debug.BuildInfo{Main: debug.Module{Version: "v6.2.0"}, Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "4e70e7f306580ad8"}}})
	if String() != "6.1.0 (abc1234, today)" {
		t.Fatal(String())
	}
}

func TestALocalBuildStaysDev(t *testing.T) {
	Version = "dev"
	fill(&debug.BuildInfo{Main: debug.Module{Version: "(devel)"}})
	if Version != "dev" {
		t.Fatal(Version)
	}
}
