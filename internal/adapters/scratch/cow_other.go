//go:build !linux && !darwin

package scratch

// cloneFile reports that copy-on-write clones are not available here.
func cloneFile(string, string) bool { return false }
