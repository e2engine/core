package fs

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/ygrebnov/errorc"
	sharedfs "github.com/ygrebnov/fs"

	coreerrors "github.com/e2engine/core/pkg/errors"
	"github.com/e2engine/core/pkg/keys"
)

// MaxFileSize limits files loaded into memory through this package.
const MaxFileSize int64 = 16 << 20 // 16 MiB

func fileError(path string, err error) error {
	fields := []errorc.Field{errorc.String(keys.FilePath, path)}
	switch {
	case errors.Is(err, sharedfs.ErrNotRegularFile):
		fields = append(fields, errorc.String(keys.Validation, "not a regular file"))
	case errors.Is(err, sharedfs.ErrExceedsFileSizeLimit):
		fields = append(fields, errorc.String(keys.Validation, "file size exceeds maximum allowed"))
	default:
		fields = append(fields, errorc.Error(keys.Cause, err))
	}
	return errorc.With(coreerrors.ErrInaccessiblePath, fields...)
}

// ValidateFilePath checks that path names an accessible, regular file
// of bounded size.
//
// It does not restrict the file to a particular directory.
func ValidateFilePath(path string) error {
	file, err := sharedfs.OpenRegularRead(path)
	if err != nil {
		return fileError(path, err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return fileError(path, err)
	}
	if info.Size() > MaxFileSize {
		return fileError(path, sharedfs.ErrExceedsFileSizeLimit)
	}
	return nil
}

// ReadFile reads an accessible, regular file up to MaxFileSize.
//
// It is not a directory confinement boundary.
// Use ReadFileWithinRoot when directory confinement is required.
func ReadFile(path string) ([]byte, error) {
	data, err := sharedfs.ReadFile(path, MaxFileSize)
	if err != nil {
		return nil, fileError(path, err)
	}
	return data, nil
}

// ReadFileWithinRoot reads a file confined to root.
//
// The path must be relative to root. os.Root prevents traversal
// outside root through '..' or symlinks.
func ReadFileWithinRoot(root, path string) ([]byte, error) {
	if filepath.IsAbs(path) || path == "" {
		return nil, errorc.With(
			coreerrors.ErrInaccessiblePath,
			errorc.String(keys.FilePath, path),
			errorc.String(keys.Validation, "path must be non-empty and relative to root"),
		)
	}

	dir, err := os.OpenRoot(root)
	if err != nil {
		return nil, fileError(root, err)
	}
	defer dir.Close()

	data, err := sharedfs.ReadFileWithinRoot(dir, path, MaxFileSize)
	if err != nil {
		return nil, fileError(path, err)
	}
	return data, nil
}
