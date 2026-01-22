//go:build unix

package wrtagflag

import (
	"io/fs"
	"syscall"
)

func init() {
	umask := syscall.Umask(0)
	syscall.Umask(umask)
	defaultFileMode = fs.FileMode(0o666 &^ umask) //nolint:gosec
}
