package util

import (
	"io"
	"os"
	"path/filepath"

	"github.com/ygrebnov/errorc"

	"github.com/e2engine/core/pkg/errors"
	"github.com/e2engine/core/pkg/keys"
)

// MaxFileSize limits files loaded into memory through this package.
const MaxFileSize int64 = 16 << 20 // 16 MiB

// validateRegularFile checks the opened file rather than a separate path lookup.
func validateRegularFile(file *os.File, path string) error {
	info, err := file.Stat()
	if err != nil {
		return errorc.With(
			errors.ErrInaccessiblePath,
			errorc.String(keys.FilePath, path),
			errorc.Error(keys.Cause, err),
		)
	}

	if !info.Mode().IsRegular() {
		return errorc.With(
			errors.ErrInaccessiblePath,
			errorc.String(keys.FilePath, path),
			errorc.String(keys.Validation, "not a regular file"),
		)
	}

	if info.Size() > MaxFileSize {
		return errorc.With(
			errors.ErrInaccessiblePath,
			errorc.String(keys.FilePath, path),
			errorc.String(keys.Validation, "file size exceeds maximum allowed"),
		)
	}

	return nil
}

func readOpenedFile(file *os.File, path string) ([]byte, error) {
	if err := validateRegularFile(file, path); err != nil {
		return nil, err
	}

	data, err := io.ReadAll(
		io.LimitReader(file, MaxFileSize+1),
	)
	if err != nil {
		return nil, errorc.With(
			errors.ErrInaccessiblePath,
			errorc.String(keys.FilePath, path),
			errorc.Error(keys.Cause, err),
		)
	}

	if int64(len(data)) > MaxFileSize {
		return nil, errorc.With(
			errors.ErrInaccessiblePath,
			errorc.String(keys.FilePath, path),
			errorc.String(keys.Validation, "file size exceeds maximum allowed"),
		)
	}

	return data, nil
}

// ValidateFilePath checks that path names an accessible, regular file
// of bounded size.
//
// It does not restrict the file to a particular directory.
func ValidateFilePath(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return errorc.With(
			errors.ErrInaccessiblePath,
			errorc.String(keys.FilePath, path),
			errorc.Error(keys.Cause, err),
		)
	}
	defer file.Close()

	return validateRegularFile(file, path)
}

// ReadFile reads an accessible, regular file up to MaxFileSize.
//
// It is not a directory confinement boundary.
// Use ReadFileWithinRoot when directory confinement is required.
func ReadFile(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, errorc.With(
			errors.ErrInaccessiblePath,
			errorc.String(keys.FilePath, path),
			errorc.Error(keys.Cause, err),
		)
	}
	defer file.Close()

	return readOpenedFile(file, path)
}

// ReadFileWithinRoot reads a file confined to root.
//
// The path must be relative to root. os.Root prevents traversal
// outside root through '..' or symlinks.
func ReadFileWithinRoot(root, path string) ([]byte, error) {
	if filepath.IsAbs(path) || path == "" {
		return nil, errorc.With(
			errors.ErrInaccessiblePath,
			errorc.String(keys.FilePath, path),
			errorc.String(keys.Validation, "path must be non-empty and relative to root"),
		)
	}

	dir, err := os.OpenRoot(root)
	if err != nil {
		return nil, errorc.With(
			errors.ErrInaccessiblePath,
			errorc.String(keys.FilePath, root),
			errorc.Error(keys.Cause, err),
		)
	}
	defer dir.Close()

	file, err := dir.Open(path)
	if err != nil {
		return nil, errorc.With(
			errors.ErrInaccessiblePath,
			errorc.String(keys.FilePath, path),
			errorc.Error(keys.Cause, err),
		)
	}
	defer file.Close()

	return readOpenedFile(file, path)
}
