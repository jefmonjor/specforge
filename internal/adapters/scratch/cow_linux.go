package scratch

import (
	"os"

	"golang.org/x/sys/unix"
)

// cloneFile makes dst a copy-on-write clone of src (Btrfs, XFS and other
// file systems with reflinks); false when the file system cannot.
func cloneFile(src, dst string) bool {
	in, err := os.Open(src)
	if err != nil {
		return false
	}
	defer func() { _ = in.Close() }()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return false
	}
	err = unix.IoctlFileClone(int(out.Fd()), int(in.Fd()))
	if out.Close() != nil || err != nil {
		// Leave no file behind: the plain copy creates it with its mode.
		_ = os.Remove(dst)
		return false
	}
	return true
}
