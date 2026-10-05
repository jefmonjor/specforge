// Package buildinfo exposes the version, commit and build date injected at
// link time via -ldflags "-X github.com/jefmonjor/specforge/v6/internal/buildinfo.Version=...".
// A binary built with go install has no such flags: it takes the module
// version and the commit Go recorded. Defaults identify a local build.
package buildinfo

import "runtime/debug"

var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

func init() {
	if info, ok := debug.ReadBuildInfo(); ok {
		fill(info)
	}
}

// fill completes what the link flags left at their defaults.
func fill(info *debug.BuildInfo) {
	if Version == "dev" && info.Main.Version != "" && info.Main.Version != "(devel)" {
		Version = info.Main.Version
	}
	for _, s := range info.Settings {
		switch {
		case s.Key == "vcs.revision" && Commit == "none" && len(s.Value) >= 7:
			Commit = s.Value[:7]
		case s.Key == "vcs.time" && Date == "unknown":
			Date = s.Value
		}
	}
}

// String returns a single-line human-readable description of the build:
// the version alone when nothing recorded the commit (go install of a
// module version).
func String() string {
	if Commit == "none" && Date == "unknown" {
		return Version
	}
	return Version + " (" + Commit + ", " + Date + ")"
}
