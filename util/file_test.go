package util

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	coreerrors "github.com/e2engine/core/pkg/errors"
)

func TestValidateFilePath(t *testing.T) {
	tempDir := t.TempDir()

	filePath := filepath.Join(tempDir, "file.txt")
	if err := os.WriteFile(
		filePath,
		[]byte("test"),
		0o600,
	); err != nil {
		t.Fatalf("create test file: %v", err)
	}

	missingPath := filepath.Join(
		tempDir,
		"missing.txt",
	)

	exactSize := filepath.Join(tempDir, "exact.txt")
	writeSizedTestFile(t, exactSize, MaxFileSize)

	oversized := filepath.Join(tempDir, "oversized.txt")
	writeSizedTestFile(t, oversized, MaxFileSize+1)

	tests := []struct {
		name string
		path string

		expectedError error
	}{
		{
			name:          "empty path",
			path:          "",
			expectedError: coreerrors.ErrInaccessiblePath,
		},
		{
			name:          "missing path",
			path:          missingPath,
			expectedError: coreerrors.ErrInaccessiblePath,
		},
		{
			name:          "directory",
			path:          tempDir,
			expectedError: coreerrors.ErrInaccessiblePath,
		},
		{
			name: "regular file",
			path: filePath,
		},
		{
			name: "exact size limit",
			path: exactSize,
		},
		{
			name:          "oversized file",
			path:          oversized,
			expectedError: coreerrors.ErrInaccessiblePath,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFilePath(tt.path)

			if tt.expectedError != nil {
				if !errors.Is(err, tt.expectedError) {
					t.Fatalf(
						"expected error %v, got %v",
						tt.expectedError,
						err,
					)
				}

				return
			}

			if err != nil {
				t.Fatalf(
					"expected no error, got %v",
					err,
				)
			}
		})
	}
}

func TestReadFile(t *testing.T) {
	dir := t.TempDir()

	regular := filepath.Join(dir, "file.txt")
	writeTestFile(t, regular, []byte("hello"))

	empty := filepath.Join(dir, "empty.txt")
	writeTestFile(t, empty, nil)

	exact := filepath.Join(dir, "exact.txt")
	writeSizedTestFile(t, exact, MaxFileSize)

	oversized := filepath.Join(dir, "oversized.txt")
	writeSizedTestFile(t, oversized, MaxFileSize+1)

	tests := []struct {
		name          string
		path          string
		expected      []byte
		expectedSize  int
		expectedError error
	}{
		{
			name:     "regular file",
			path:     regular,
			expected: []byte("hello"),
		},
		{
			name: "empty file",
			path: empty,
		},
		{
			name:         "exact size limit",
			path:         exact,
			expectedSize: int(MaxFileSize),
		},
		{
			name:          "oversized file",
			path:          oversized,
			expectedError: coreerrors.ErrInaccessiblePath,
		},
		{
			name:          "directory",
			path:          dir,
			expectedError: coreerrors.ErrInaccessiblePath,
		},
		{
			name:          "missing file",
			path:          filepath.Join(dir, "missing.txt"),
			expectedError: coreerrors.ErrInaccessiblePath,
		},
		{
			name:          "empty path",
			path:          "",
			expectedError: coreerrors.ErrInaccessiblePath,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := ReadFile(tt.path)

			if !errors.Is(err, tt.expectedError) {
				t.Fatalf(
					"expected error %v, got %v",
					tt.expectedError,
					err,
				)
			}

			if tt.expectedError != nil {
				if actual != nil {
					t.Fatalf(
						"expected nil data on error, got %d bytes",
						len(actual),
					)
				}
				return
			}

			if tt.expectedSize != 0 {
				if len(actual) != tt.expectedSize {
					t.Fatalf(
						"expected %d bytes, got %d",
						tt.expectedSize,
						len(actual),
					)
				}
			} else if !bytes.Equal(actual, tt.expected) {
				t.Fatalf(
					"expected %q, got %q",
					tt.expected,
					actual,
				)
			}
		})
	}
}

func TestReadFileWithinRoot(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "allowed")

	if err := os.MkdirAll(
		filepath.Join(root, "nested"),
		0o700,
	); err != nil {
		t.Fatal(err)
	}

	writeTestFile(
		t,
		filepath.Join(root, "nested", "file.txt"),
		[]byte("inside"),
	)

	writeTestFile(
		t,
		filepath.Join(parent, "outside.txt"),
		[]byte("outside"),
	)

	writeSizedTestFile(
		t,
		filepath.Join(root, "oversized.txt"),
		MaxFileSize+1,
	)

	if err := os.Symlink(
		"nested/file.txt",
		filepath.Join(root, "inside-link"),
	); err != nil {
		t.Fatal(err)
	}

	if err := os.Symlink(
		"../outside.txt",
		filepath.Join(root, "outside-link"),
	); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name          string
		root          string
		path          string
		expected      []byte
		expectedError error
	}{
		{
			name:     "nested file",
			root:     root,
			path:     "nested/file.txt",
			expected: []byte("inside"),
		},
		{
			name:     "internal symlink",
			root:     root,
			path:     "inside-link",
			expected: []byte("inside"),
		},
		{
			name:          "parent traversal",
			root:          root,
			path:          "../outside.txt",
			expectedError: coreerrors.ErrInaccessiblePath,
		},
		{
			name:          "escaping symlink",
			root:          root,
			path:          "outside-link",
			expectedError: coreerrors.ErrInaccessiblePath,
		},
		{
			name:          "absolute path",
			root:          root,
			path:          filepath.Join(root, "nested", "file.txt"),
			expectedError: coreerrors.ErrInaccessiblePath,
		},
		{
			name:          "empty path",
			root:          root,
			path:          "",
			expectedError: coreerrors.ErrInaccessiblePath,
		},
		{
			name:          "directory",
			root:          root,
			path:          "nested",
			expectedError: coreerrors.ErrInaccessiblePath,
		},
		{
			name:          "missing file",
			root:          root,
			path:          "missing.txt",
			expectedError: coreerrors.ErrInaccessiblePath,
		},
		{
			name:          "oversized file",
			root:          root,
			path:          "oversized.txt",
			expectedError: coreerrors.ErrInaccessiblePath,
		},
		{
			name:          "missing root",
			root:          filepath.Join(parent, "missing-root"),
			path:          "file.txt",
			expectedError: coreerrors.ErrInaccessiblePath,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := ReadFileWithinRoot(
				tt.root,
				tt.path,
			)

			if !errors.Is(err, tt.expectedError) {
				t.Fatalf(
					"expected error %v, got %v",
					tt.expectedError,
					err,
				)
			}

			if tt.expectedError != nil {
				if actual != nil {
					t.Fatalf(
						"expected nil data on error, got %d bytes",
						len(actual),
					)
				}
				return
			}

			if !bytes.Equal(actual, tt.expected) {
				t.Fatalf(
					"expected %q, got %q",
					tt.expected,
					actual,
				)
			}
		})
	}
}

func writeTestFile(t *testing.T, path string, data []byte) {
	t.Helper()

	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func writeSizedTestFile(t *testing.T, path string, size int64) {
	t.Helper()

	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := file.Truncate(size); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}

	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}
