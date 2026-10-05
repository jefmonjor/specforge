package scratch

import "golang.org/x/sys/unix"

// cloneFile makes dst a copy-on-write clone of src (APFS); false when the
// file system cannot.
func cloneFile(src, dst string) bool {
	return unix.Clonefile(src, dst, unix.CLONE_NOFOLLOW) == nil
}
