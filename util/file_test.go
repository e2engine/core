package util

import (
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
