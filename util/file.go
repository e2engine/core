package util

import (
	"io"
	"os"

	"github.com/ygrebnov/errorc"

	"github.com/e2engine/core/pkg/errors"
	"github.com/e2engine/core/pkg/keys"
)

// ValidateFilePath performs basic path validation.
// It is not a security boundary.
func ValidateFilePath(path string) error {
	if path == "" {
		return errorc.With(
			errors.ErrInaccessiblePath,
			errorc.String(keys.FilePath, path),
		)
	}
	info, err := os.Stat(path)
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
		)
	}

	return nil
}

// ReadFile opens, validates, and reads the same filesystem object.
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

	info, err := file.Stat()
	if err != nil {
		return nil, errorc.With(
			errors.ErrInaccessiblePath,
			errorc.String(keys.FilePath, path),
			errorc.Error(keys.Cause, err),
		)
	}

	if !info.Mode().IsRegular() {
		return nil, errorc.With(
			errors.ErrInaccessiblePath,
			errorc.String(keys.FilePath, path),
		)
	}

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, errorc.With(
			errors.ErrInaccessiblePath,
			errorc.String(keys.FilePath, path),
			errorc.Error(keys.Cause, err),
		)
	}

	return data, nil
}

// TODO: add FilePolicy with MaxSize, AllowSymlinks etc.
