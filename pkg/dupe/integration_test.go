package dupe

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/lixmal/finddupes/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestE2E_CompleteWorkflow tests the complete workflow from indexing to deletion
func TestE2E_CompleteWorkflow(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	photosDir := filepath.Join(tmpDir, "photos")
	backupDir := filepath.Join(tmpDir, "backup")
	require.NoError(t, os.MkdirAll(photosDir, 0755))
	require.NoError(t, os.MkdirAll(backupDir, 0755))

	photo1 := filepath.Join(photosDir, "vacation.jpg")
	photo2 := filepath.Join(backupDir, "vacation.jpg")
	photo3 := filepath.Join(photosDir, "family.jpg")
	photo4 := filepath.Join(backupDir, "family.jpg")

	content1 := "fake jpeg content for vacation"
	content2 := "fake jpeg content for family"

	require.NoError(t, os.WriteFile(photo1, []byte(content1), 0644))
	require.NoError(t, os.WriteFile(photo2, []byte(content1), 0644))
	require.NoError(t, os.WriteFile(photo3, []byte(content2), 0644))
	require.NoError(t, os.WriteFile(photo4, []byte(content2), 0644))

	conf := config.Config{
		StoreOnly: true,
		Path:      dbPath,
		Workers:   2,
		Verbose:   false,
	}

	dupe := New(conf)
	err := dupe.ProcessFiles([]string{tmpDir})
	require.NoError(t, err)

	assert.FileExists(t, dbPath, "Database file should be created")
	assert.FileExists(t, photo1)
	assert.FileExists(t, photo2)

	delRegex := regexp.MustCompile(`/backup/`)
	conf2 := config.Config{
		Path:     dbPath,
		Delete:   true,
		DelMatch: delRegex,
		Workers:  2,
	}

	dupe2 := New(conf2)
	err = dupe2.ProcessFiles(nil)
	require.NoError(t, err)

	assert.FileExists(t, photo1, "Original photo1 should be kept")
	assert.NoFileExists(t, photo2, "Backup photo2 should be deleted")
	assert.FileExists(t, photo3, "Original photo3 should be kept")
	assert.NoFileExists(t, photo4, "Backup photo4 should be deleted")
}

// TestE2E_OnTheFlyMode tests the workflow without database persistence
func TestE2E_OnTheFlyMode(t *testing.T) {
	tmpDir := t.TempDir()

	file1 := filepath.Join(tmpDir, "aaa_file.txt")
	file2 := filepath.Join(tmpDir, "bbb_file.txt")
	file3 := filepath.Join(tmpDir, "ccc_file.txt")
	content := "duplicate content"

	require.NoError(t, os.WriteFile(file1, []byte(content), 0644))
	require.NoError(t, os.WriteFile(file2, []byte(content), 0644))
	require.NoError(t, os.WriteFile(file3, []byte(content), 0644))

	conf := config.Config{
		Delete:    true,
		KeepFirst: true,
		Workers:   2,
	}

	dupe := New(conf)
	err := dupe.ProcessFiles([]string{tmpDir})
	require.NoError(t, err)

	assert.FileExists(t, file1, "First file should be kept")
	assert.NoFileExists(t, file2, "Second file should be deleted")
	assert.NoFileExists(t, file3, "Third file should be deleted")
}

// TestE2E_MultipleDirectories tests handling multiple input directories
func TestE2E_MultipleDirectories(t *testing.T) {
	tmpDir := t.TempDir()

	dir1 := filepath.Join(tmpDir, "dir1")
	dir2 := filepath.Join(tmpDir, "dir2")
	dir3 := filepath.Join(tmpDir, "dir3")

	require.NoError(t, os.MkdirAll(dir1, 0755))
	require.NoError(t, os.MkdirAll(dir2, 0755))
	require.NoError(t, os.MkdirAll(dir3, 0755))

	file1 := filepath.Join(dir1, "file.txt")
	file2 := filepath.Join(dir2, "file.txt")
	file3 := filepath.Join(dir3, "unique.txt")

	content := "shared content"
	require.NoError(t, os.WriteFile(file1, []byte(content), 0644))
	require.NoError(t, os.WriteFile(file2, []byte(content), 0644))
	require.NoError(t, os.WriteFile(file3, []byte("unique"), 0644))

	conf := config.Config{
		Delete:    true,
		KeepFirst: true,
		Workers:   2,
	}

	dupe := New(conf)
	err := dupe.ProcessFiles([]string{dir1, dir2, dir3})
	require.NoError(t, err)

	remainingCount := 0
	if _, err := os.Stat(file1); err == nil {
		remainingCount++
	}
	if _, err := os.Stat(file2); err == nil {
		remainingCount++
	}

	assert.Equal(t, 1, remainingCount, "Exactly 1 duplicate should remain")
	assert.FileExists(t, file3, "Unique file should not be deleted")
}

// TestE2E_DatabasePersistence tests that database correctly persists between runs
func TestE2E_DatabasePersistence(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "persist.db")
	filesDir := filepath.Join(tmpDir, "files")

	require.NoError(t, os.MkdirAll(filesDir, 0755))

	file1 := filepath.Join(filesDir, "file1.txt")
	file2 := filepath.Join(filesDir, "file2.txt")
	content := "test content"

	require.NoError(t, os.WriteFile(file1, []byte(content), 0644))
	require.NoError(t, os.WriteFile(file2, []byte(content), 0644))

	conf1 := config.Config{
		StoreOnly: true,
		Path:      dbPath,
		Workers:   2,
	}

	dupe1 := New(conf1)
	require.NoError(t, dupe1.ProcessFiles([]string{filesDir}))

	conf2 := config.Config{
		Path:      dbPath,
		Delete:    true,
		KeepFirst: true,
		Workers:   2,
	}

	dupe2 := New(conf2)
	require.NoError(t, dupe2.ProcessFiles(nil))

	assert.NotEmpty(t, dupe2.database.Hashes, "Database was not loaded from disk")

	remainingCount := 0
	if _, err := os.Stat(file1); err == nil {
		remainingCount++
	}
	if _, err := os.Stat(file2); err == nil {
		remainingCount++
	}

	assert.Equal(t, 1, remainingCount, "Exactly 1 file should remain")
}

// TestE2E_VerifyDatabaseUpdates tests that database removes stale entries
func TestE2E_VerifyDatabaseUpdates(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "verify.db")
	filesDir := filepath.Join(tmpDir, "files")

	require.NoError(t, os.MkdirAll(filesDir, 0755))

	file1 := filepath.Join(filesDir, "file1.txt")
	file2 := filepath.Join(filesDir, "file2.txt")
	content := "test content"

	require.NoError(t, os.WriteFile(file1, []byte(content), 0644))
	require.NoError(t, os.WriteFile(file2, []byte(content), 0644))

	conf := config.Config{
		StoreOnly: true,
		Path:      dbPath,
		Workers:   2,
	}

	dupe1 := New(conf)
	require.NoError(t, dupe1.ProcessFiles([]string{filesDir}))

	require.NoError(t, os.Remove(file2))

	dupe2 := New(conf)
	require.NoError(t, dupe2.ProcessFiles([]string{filesDir}))

	totalFiles := 0
	for _, fileMap := range dupe2.database.Files {
		totalFiles += len(fileMap)
	}

	assert.Equal(t, 1, totalFiles, "Database should have 1 file after cleanup")
}

// TestE2E_ModifiedFileDetection tests that modified files are detected and handled correctly
func TestE2E_ModifiedFileDetection(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "rehash.db")
	filesDir := filepath.Join(tmpDir, "files")

	require.NoError(t, os.MkdirAll(filesDir, 0755))

	file1 := filepath.Join(filesDir, "file1.txt")
	file2 := filepath.Join(filesDir, "file2.txt")
	content := "duplicate content"

	require.NoError(t, os.WriteFile(file1, []byte(content), 0644))
	require.NoError(t, os.WriteFile(file2, []byte(content), 0644))

	conf := config.Config{
		StoreOnly: true,
		Path:      dbPath,
		Workers:   2,
	}

	dupe1 := New(conf)
	require.NoError(t, dupe1.ProcessFiles([]string{filesDir}))

	hashCountBefore := len(dupe1.database.Hashes)
	assert.NotZero(t, hashCountBefore, "No hashes found after first run")

	time.Sleep(100 * time.Millisecond)

	require.NoError(t, os.WriteFile(file1, []byte("completely different content now"), 0644))

	dupe2 := New(conf)
	require.NoError(t, dupe2.ProcessFiles([]string{filesDir}))

	totalFiles := 0
	for _, fileMap := range dupe2.database.Files {
		totalFiles += len(fileMap)
	}

	assert.Equal(t, 2, totalFiles, "Database should still have 2 files")
}

// TestE2E_ComplexRulesCombination tests multiple deletion rules working together
func TestE2E_ComplexRulesCombination(t *testing.T) {
	tmpDir := t.TempDir()

	importantDir := filepath.Join(tmpDir, "important")
	tempDir := filepath.Join(tmpDir, "temp")

	require.NoError(t, os.MkdirAll(importantDir, 0755))
	require.NoError(t, os.MkdirAll(tempDir, 0755))

	file1 := filepath.Join(importantDir, "data.jpg")
	file2 := filepath.Join(tempDir, "data.jpg")
	file3 := filepath.Join(tempDir, "data.txt")
	content := "duplicate content"

	require.NoError(t, os.WriteFile(file1, []byte(content), 0644))
	require.NoError(t, os.WriteFile(file2, []byte(content), 0644))
	require.NoError(t, os.WriteFile(file3, []byte(content), 0644))

	delRegex := regexp.MustCompile(`/temp/.*\.jpg$`)
	conf := config.Config{
		Delete:   true,
		DelMatch: delRegex,
		Workers:  2,
	}

	dupe := New(conf)
	err := dupe.ProcessFiles([]string{tmpDir})
	require.NoError(t, err)

	assert.FileExists(t, file1, "Important JPG should remain")
	assert.NoFileExists(t, file2, "Temp JPG should be deleted")
	assert.FileExists(t, file3, "TXT file should remain")
}

// TestE2E_EmptyDirectories tests handling of empty directories gracefully
func TestE2E_EmptyDirectories(t *testing.T) {
	tmpDir := t.TempDir()

	emptyDir := filepath.Join(tmpDir, "empty")
	require.NoError(t, os.MkdirAll(emptyDir, 0755))

	conf := config.Config{
		Delete:    true,
		KeepFirst: true,
		Workers:   2,
	}

	dupe := New(conf)
	err := dupe.ProcessFiles([]string{emptyDir})

	require.NoError(t, err, "Should handle empty directories gracefully")
}

// TestE2E_LargeNumberOfDuplicates tests performance with many duplicates
func TestE2E_LargeNumberOfDuplicates(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long test in short mode")
	}

	tmpDir := t.TempDir()
	content := "shared content across all files"

	fileCount := 50
	for i := 0; i < fileCount; i++ {
		filePath := filepath.Join(tmpDir, filepath.Base(tmpDir)+string(rune('a'+i%26))+".txt")
		require.NoError(t, os.WriteFile(filePath, []byte(content), 0644))
	}

	conf := config.Config{
		Delete:    true,
		KeepFirst: true,
		Workers:   4,
	}

	dupe := New(conf)
	err := dupe.ProcessFiles([]string{tmpDir})
	require.NoError(t, err)

	remainingCount := 0
	entries, _ := os.ReadDir(tmpDir)
	for _, entry := range entries {
		if !entry.IsDir() {
			remainingCount++
		}
	}

	assert.Equal(t, 1, remainingCount, "Expected exactly 1 file out of %d duplicates", fileCount)
}
