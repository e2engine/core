//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package fs

import (
	"errors"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	coreerrors "github.com/e2engine/core/pkg/errors"
)

func TestFileReadersRejectFIFOWithoutBlocking(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "spec.yaml")
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Fatal(err)
	}

	for name, read := range map[string]func() error{
		"validate": func() error { return ValidateFilePath(path) },
		"read":     func() error { _, err := ReadFile(path); return err },
		"read within root": func() error {
			_, err := ReadFileWithinRoot(root, "spec.yaml")
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			result := make(chan error, 1)
			go func() { result <- read() }()
			select {
			case err := <-result:
				if !errors.Is(err, coreerrors.ErrInaccessiblePath) {
					t.Fatalf("error = %v, want ErrInaccessiblePath", err)
				}
			case <-time.After(time.Second):
				t.Fatal("FIFO read blocked")
			}
		})
	}
}
