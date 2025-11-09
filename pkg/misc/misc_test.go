package misc

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHash(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name        string
		content     string
		wantErr     bool
		description string
	}{
		{
			name:        "empty file",
			content:     "",
			wantErr:     false,
			description: "Empty file should hash successfully",
		},
		{
			name:        "small file",
			content:     "Hello, World!",
			wantErr:     false,
			description: "Small text file should hash successfully",
		},
		{
			name:        "binary content",
			content:     "\x00\x01\x02\x03\xff\xfe\xfd",
			wantErr:     false,
			description: "Binary content should hash successfully",
		},
		{
			name:        "large file",
			content:     string(make([]byte, 1024*1024)),
			wantErr:     false,
			description: "Large file (1MB) should hash successfully",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath := filepath.Join(tmpDir, tt.name+".txt")
			err := os.WriteFile(filePath, []byte(tt.content), 0644)
			require.NoError(t, err)

			hash, err := Hash(filePath)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, hash, "Hash should not be empty for valid file")
			}
		})
	}
}

func TestHash_Consistency(t *testing.T) {
	tmpDir := t.TempDir()
	content := "This is test content for consistency check"

	filePath := filepath.Join(tmpDir, "consistent.txt")
	err := os.WriteFile(filePath, []byte(content), 0644)
	require.NoError(t, err)

	hash1, err := Hash(filePath)
	require.NoError(t, err)

	hash2, err := Hash(filePath)
	require.NoError(t, err)

	assert.Equal(t, hash1, hash2, "Hashes should be consistent")
}

func TestHash_DifferentContent(t *testing.T) {
	tmpDir := t.TempDir()

	file1Path := filepath.Join(tmpDir, "file1.txt")
	file2Path := filepath.Join(tmpDir, "file2.txt")

	err := os.WriteFile(file1Path, []byte("content 1"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(file2Path, []byte("content 2"), 0644)
	require.NoError(t, err)

	hash1, err := Hash(file1Path)
	require.NoError(t, err)

	hash2, err := Hash(file2Path)
	require.NoError(t, err)

	assert.NotEqual(t, hash1, hash2, "Different content should produce different hashes")
}

func TestHash_SameContent(t *testing.T) {
	tmpDir := t.TempDir()
	content := "identical content"

	file1Path := filepath.Join(tmpDir, "same1.txt")
	file2Path := filepath.Join(tmpDir, "same2.txt")

	err := os.WriteFile(file1Path, []byte(content), 0644)
	require.NoError(t, err)
	err = os.WriteFile(file2Path, []byte(content), 0644)
	require.NoError(t, err)

	hash1, err := Hash(file1Path)
	require.NoError(t, err)

	hash2, err := Hash(file2Path)
	require.NoError(t, err)

	assert.Equal(t, hash1, hash2, "Same content should produce same hash")
}

func TestHash_NonExistentFile(t *testing.T) {
	_, err := Hash("/path/that/does/not/exist/file.txt")
	assert.Error(t, err, "Expected error for non-existent file")
}

func TestHash_Directory(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := Hash(tmpDir)
	assert.Error(t, err, "Expected error when trying to hash a directory")
}

func TestClose(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "closable.txt")

	err := os.WriteFile(filePath, []byte("test"), 0644)
	require.NoError(t, err)

	file, err := os.Open(filePath)
	require.NoError(t, err)

	Close(filePath, file)

	_, err = file.Read(make([]byte, 1))
	assert.Error(t, err, "File should be closed and not readable")
}

func TestClose_AlreadyClosed(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "already_closed.txt")

	err := os.WriteFile(filePath, []byte("test"), 0644)
	require.NoError(t, err)

	file, err := os.Open(filePath)
	require.NoError(t, err)

	err = file.Close()
	require.NoError(t, err)

	Close(filePath, file)
}
