//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package util

import (
	"os"
	"syscall"
)

// A FIFO or device must not block a command before its file type is checked.
func openReadFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
}

func openRootReadFile(root *os.Root, path string) (*os.File, error) {
	return root.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
}
