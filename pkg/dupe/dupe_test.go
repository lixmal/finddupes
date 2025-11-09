package dupe

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/lixmal/finddupes/pkg/config"
	"github.com/lixmal/finddupes/pkg/file"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	conf := config.Config{
		Workers: 4,
		Path:    "/tmp/test.db",
	}

	dupe := New(conf)

	require.NotNil(t, dupe)
	assert.NotNil(t, dupe.ctx)
	assert.NotNil(t, dupe.cancel)
	assert.NotNil(t, dupe.done)
	assert.NotNil(t, dupe.database)
	assert.Equal(t, 4, dupe.config.Workers)
}

func TestDupe_Done(t *testing.T) {
	dupe := New(config.Config{})
	done := dupe.Done()

	select {
	case <-done:
		assert.Fail(t, "Done channel should not be closed initially")
	default:
	}

	go func() {
		close(dupe.done)
	}()

	<-done
}

func TestDupe_Stop(t *testing.T) {
	dupe := New(config.Config{})

	select {
	case <-dupe.ctx.Done():
		assert.Fail(t, "Context should not be cancelled initially")
	default:
	}

	dupe.Stop()

	<-dupe.ctx.Done()
}

func TestDupe_matchRules_KeepRecent(t *testing.T) {
	now := time.Now()
	older := now.Add(-1 * time.Hour)
	oldest := now.Add(-2 * time.Hour)

	files := file.Slice{
		&file.File{Path: "/old", MTime: oldest},
		&file.File{Path: "/recent", MTime: now},
		&file.File{Path: "/middle", MTime: older},
	}

	dupe := New(config.Config{KeepRecent: true})

	tests := []struct {
		name     string
		index    int
		file     *file.File
		expected bool
	}{
		{
			name:     "most recent file should not match",
			index:    1,
			file:     files[1],
			expected: false,
		},
		{
			name:     "older file should match",
			index:    0,
			file:     files[0],
			expected: true,
		},
		{
			name:     "middle file should match",
			index:    2,
			file:     files[2],
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matched := dupe.matchRules(files, tt.index, tt.file)
			assert.Equal(t, tt.expected, matched)
		})
	}
}

func TestDupe_matchRules_KeepOldest(t *testing.T) {
	now := time.Now()
	older := now.Add(-1 * time.Hour)
	oldest := now.Add(-2 * time.Hour)

	files := file.Slice{
		&file.File{Path: "/recent", MTime: now},
		&file.File{Path: "/old", MTime: oldest},
		&file.File{Path: "/middle", MTime: older},
	}

	dupe := New(config.Config{KeepOldest: true})

	tests := []struct {
		name     string
		index    int
		file     *file.File
		expected bool
	}{
		{
			name:     "oldest file should not match",
			index:    1,
			file:     files[1],
			expected: false,
		},
		{
			name:     "recent file should match",
			index:    0,
			file:     files[0],
			expected: true,
		},
		{
			name:     "middle file should match",
			index:    2,
			file:     files[2],
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matched := dupe.matchRules(files, tt.index, tt.file)
			assert.Equal(t, tt.expected, matched)
		})
	}
}

func TestDupe_matchRules_KeepFirst(t *testing.T) {
	files := file.Slice{
		&file.File{Path: "/aaa"},
		&file.File{Path: "/bbb"},
		&file.File{Path: "/ccc"},
	}

	dupe := New(config.Config{KeepFirst: true})

	tests := []struct {
		name     string
		index    int
		expected bool
	}{
		{
			name:     "first file should not match",
			index:    0,
			expected: false,
		},
		{
			name:     "second file should match",
			index:    1,
			expected: true,
		},
		{
			name:     "third file should match",
			index:    2,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matched := dupe.matchRules(files, tt.index, files[tt.index])
			assert.Equal(t, tt.expected, matched)
		})
	}
}

func TestDupe_matchRules_KeepLast(t *testing.T) {
	files := file.Slice{
		&file.File{Path: "/aaa"},
		&file.File{Path: "/bbb"},
		&file.File{Path: "/ccc"},
	}

	dupe := New(config.Config{KeepLast: true})

	tests := []struct {
		name     string
		index    int
		expected bool
	}{
		{
			name:     "first file should match",
			index:    0,
			expected: true,
		},
		{
			name:     "second file should match",
			index:    1,
			expected: true,
		},
		{
			name:     "last file should not match",
			index:    2,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matched := dupe.matchRules(files, tt.index, files[tt.index])
			assert.Equal(t, tt.expected, matched)
		})
	}
}

func TestDupe_matchRules_DelMatch(t *testing.T) {
	files := file.Slice{
		&file.File{Path: "/path/file.jpg"},
		&file.File{Path: "/path/file.png"},
		&file.File{Path: "/path/document.txt"},
	}

	delRegex := regexp.MustCompile(`\.(jpg|png)$`)
	dupe := New(config.Config{DelMatch: delRegex})

	tests := []struct {
		name     string
		index    int
		expected bool
	}{
		{
			name:     "jpg file should match",
			index:    0,
			expected: true,
		},
		{
			name:     "png file should match",
			index:    1,
			expected: true,
		},
		{
			name:     "txt file should not match",
			index:    2,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matched := dupe.matchRules(files, tt.index, files[tt.index])
			assert.Equal(t, tt.expected, matched)
		})
	}
}

func TestDupe_matchRules_KeepMatch(t *testing.T) {
	files := file.Slice{
		&file.File{Path: "/important/file.txt"},
		&file.File{Path: "/temp/file.txt"},
		&file.File{Path: "/archive/file.txt"},
	}

	keepRegex := regexp.MustCompile(`/important/`)
	dupe := New(config.Config{KeepMatch: keepRegex})

	tests := []struct {
		name     string
		index    int
		expected bool
	}{
		{
			name:     "important file should not match for deletion",
			index:    0,
			expected: false,
		},
		{
			name:     "temp file should match for deletion",
			index:    1,
			expected: true,
		},
		{
			name:     "archive file should match for deletion",
			index:    2,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matched := dupe.matchRules(files, tt.index, files[tt.index])
			assert.Equal(t, tt.expected, matched)
		})
	}
}

func TestDupe_matchRules_NoRules(t *testing.T) {
	files := file.Slice{
		&file.File{Path: "/file1"},
		&file.File{Path: "/file2"},
	}

	dupe := New(config.Config{})

	for i, f := range files {
		matched := dupe.matchRules(files, i, f)
		assert.False(t, matched, "No rules configured, but file %s matched", f.Path)
	}
}

func TestDupe_IndexFiles(t *testing.T) {
	tmpDir := t.TempDir()

	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")
	subDir := filepath.Join(tmpDir, "subdir")
	file3 := filepath.Join(subDir, "file3.txt")

	require.NoError(t, os.WriteFile(file1, []byte("content1"), 0644))
	require.NoError(t, os.WriteFile(file2, []byte("content2"), 0644))
	require.NoError(t, os.Mkdir(subDir, 0755))
	require.NoError(t, os.WriteFile(file3, []byte("content3"), 0644))

	dupe := New(config.Config{})
	err := dupe.IndexFiles([]string{tmpDir})

	require.NoError(t, err)

	totalFiles := 0
	for _, fileMap := range dupe.database.Files {
		totalFiles += len(fileMap)
	}

	assert.Equal(t, 3, totalFiles)
}

func TestDupe_IndexFiles_IgnoresEmpty(t *testing.T) {
	tmpDir := t.TempDir()

	emptyFile := filepath.Join(tmpDir, "empty.txt")
	require.NoError(t, os.WriteFile(emptyFile, []byte(""), 0644))

	dupe := New(config.Config{})
	err := dupe.IndexFiles([]string{tmpDir})

	require.NoError(t, err)

	totalFiles := 0
	for _, fileMap := range dupe.database.Files {
		totalFiles += len(fileMap)
	}

	assert.Equal(t, 0, totalFiles, "Empty files should be ignored")
}

func TestDupe_IndexFiles_SameSize(t *testing.T) {
	tmpDir := t.TempDir()

	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")

	content := "same size"
	require.NoError(t, os.WriteFile(file1, []byte(content), 0644))
	require.NoError(t, os.WriteFile(file2, []byte(content), 0644))

	dupe := New(config.Config{})
	err := dupe.IndexFiles([]string{tmpDir})

	require.NoError(t, err)

	size := int64(len(content))
	filesMap, exists := dupe.database.Files[size]
	require.True(t, exists, "Files of size %d should exist", size)
	assert.Len(t, filesMap, 2)
}

func TestDupe_ReadWriteDatabase(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	dupe := New(config.Config{Path: dbPath})

	f1 := &file.File{Path: "/test1", Hash: "hash1", Size: 1024}
	dupe.database.Files[1024] = file.Map{"/test1": f1}
	dupe.database.Hashes["hash1"] = file.Map{"/test1": f1}

	err := dupe.WriteDatabase()
	require.NoError(t, err)

	dupe2 := New(config.Config{Path: dbPath})
	err = dupe2.ReadDatabase()
	require.NoError(t, err)

	assert.Len(t, dupe2.database.Files[1024], 1)
	assert.Len(t, dupe2.database.Hashes["hash1"], 1)
}

func TestDupe_CalculateHashes(t *testing.T) {
	tmpDir := t.TempDir()

	file1 := filepath.Join(tmpDir, "dup1.txt")
	file2 := filepath.Join(tmpDir, "dup2.txt")
	content := "duplicate content"

	require.NoError(t, os.WriteFile(file1, []byte(content), 0644))
	require.NoError(t, os.WriteFile(file2, []byte(content), 0644))

	dupe := New(config.Config{Workers: 2})
	require.NoError(t, dupe.IndexFiles([]string{tmpDir}))

	err := dupe.CalculateHashes()
	require.NoError(t, err)

	assert.NotEmpty(t, dupe.database.Hashes, "Hashes should be calculated")

	for hash, files := range dupe.database.Hashes {
		assert.Len(t, files, 2, "Expected 2 duplicate files for hash %x", hash)
	}
}

func TestDupe_CalculateHashes_DifferentContent(t *testing.T) {
	tmpDir := t.TempDir()

	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")

	require.NoError(t, os.WriteFile(file1, []byte("content 1"), 0644))
	require.NoError(t, os.WriteFile(file2, []byte("content 2"), 0644))

	dupe := New(config.Config{Workers: 2})
	require.NoError(t, dupe.IndexFiles([]string{tmpDir}))

	err := dupe.CalculateHashes()
	if err != nil && err != ErrProcessStopped {
		require.NoError(t, err)
	}
}

func TestDupe_DeleteDuplicates_DryRun(t *testing.T) {
	tmpDir := t.TempDir()

	file1 := filepath.Join(tmpDir, "dup1.txt")
	file2 := filepath.Join(tmpDir, "dup2.txt")
	content := "duplicate content"

	require.NoError(t, os.WriteFile(file1, []byte(content), 0644))
	require.NoError(t, os.WriteFile(file2, []byte(content), 0644))

	dupe := New(config.Config{
		Workers:   2,
		Delete:    false,
		KeepFirst: true,
	})

	require.NoError(t, dupe.IndexFiles([]string{tmpDir}))
	require.NoError(t, dupe.CalculateHashes())

	err := dupe.DeleteDuplicates()
	require.NoError(t, err)

	assert.FileExists(t, file1, "File1 should not be deleted in dry-run mode")
	assert.FileExists(t, file2, "File2 should not be deleted in dry-run mode")
}

func TestDupe_DeleteDuplicates_ActualDelete_KeepFirst(t *testing.T) {
	tmpDir := t.TempDir()

	file1 := filepath.Join(tmpDir, "aaa_dup.txt")
	file2 := filepath.Join(tmpDir, "bbb_dup.txt")
	file3 := filepath.Join(tmpDir, "ccc_dup.txt")
	content := "duplicate content"

	require.NoError(t, os.WriteFile(file1, []byte(content), 0644))
	require.NoError(t, os.WriteFile(file2, []byte(content), 0644))
	require.NoError(t, os.WriteFile(file3, []byte(content), 0644))

	dupe := New(config.Config{
		Workers:   2,
		Delete:    true,
		KeepFirst: true,
	})

	require.NoError(t, dupe.IndexFiles([]string{tmpDir}))
	require.NoError(t, dupe.CalculateHashes())

	err := dupe.DeleteDuplicates()
	require.NoError(t, err)

	assert.FileExists(t, file1, "First file (aaa) should be kept")
	assert.NoFileExists(t, file2, "Second file (bbb) should be deleted")
	assert.NoFileExists(t, file3, "Third file (ccc) should be deleted")
}

func TestDupe_DeleteDuplicates_ActualDelete_KeepLast(t *testing.T) {
	tmpDir := t.TempDir()

	file1 := filepath.Join(tmpDir, "aaa_dup.txt")
	file2 := filepath.Join(tmpDir, "bbb_dup.txt")
	file3 := filepath.Join(tmpDir, "zzz_dup.txt")
	content := "duplicate content"

	require.NoError(t, os.WriteFile(file1, []byte(content), 0644))
	require.NoError(t, os.WriteFile(file2, []byte(content), 0644))
	require.NoError(t, os.WriteFile(file3, []byte(content), 0644))

	dupe := New(config.Config{
		Workers:  2,
		Delete:   true,
		KeepLast: true,
	})

	require.NoError(t, dupe.IndexFiles([]string{tmpDir}))
	require.NoError(t, dupe.CalculateHashes())

	err := dupe.DeleteDuplicates()
	require.NoError(t, err)

	assert.NoFileExists(t, file1, "First file (aaa) should be deleted")
	assert.NoFileExists(t, file2, "Second file (bbb) should be deleted")
	assert.FileExists(t, file3, "Last file (zzz) should be kept")
}

func TestDupe_DeleteDuplicates_ActualDelete_KeepRecent(t *testing.T) {
	tmpDir := t.TempDir()

	file1 := filepath.Join(tmpDir, "old.txt")
	file2 := filepath.Join(tmpDir, "recent.txt")
	content := "duplicate content"

	require.NoError(t, os.WriteFile(file1, []byte(content), 0644))

	time.Sleep(10 * time.Millisecond)

	require.NoError(t, os.WriteFile(file2, []byte(content), 0644))

	dupe := New(config.Config{
		Workers:    2,
		Delete:     true,
		KeepRecent: true,
	})

	require.NoError(t, dupe.IndexFiles([]string{tmpDir}))
	require.NoError(t, dupe.CalculateHashes())

	err := dupe.DeleteDuplicates()
	require.NoError(t, err)

	assert.NoFileExists(t, file1, "Older file should be deleted")
	assert.FileExists(t, file2, "Recent file should be kept")
}

func TestDupe_DeleteDuplicates_ActualDelete_KeepOldest(t *testing.T) {
	tmpDir := t.TempDir()

	file1 := filepath.Join(tmpDir, "oldest.txt")
	file2 := filepath.Join(tmpDir, "newer.txt")
	content := "duplicate content"

	require.NoError(t, os.WriteFile(file1, []byte(content), 0644))

	time.Sleep(10 * time.Millisecond)

	require.NoError(t, os.WriteFile(file2, []byte(content), 0644))

	dupe := New(config.Config{
		Workers:    2,
		Delete:     true,
		KeepOldest: true,
	})

	require.NoError(t, dupe.IndexFiles([]string{tmpDir}))
	require.NoError(t, dupe.CalculateHashes())

	err := dupe.DeleteDuplicates()
	require.NoError(t, err)

	assert.FileExists(t, file1, "Oldest file should be kept")
	assert.NoFileExists(t, file2, "Newer file should be deleted")
}

func TestDupe_DeleteDuplicates_ActualDelete_DelMatch(t *testing.T) {
	tmpDir := t.TempDir()

	file1 := filepath.Join(tmpDir, "keep.txt")
	file2 := filepath.Join(tmpDir, "delete.jpg")
	file3 := filepath.Join(tmpDir, "delete2.png")
	content := "duplicate content"

	require.NoError(t, os.WriteFile(file1, []byte(content), 0644))
	require.NoError(t, os.WriteFile(file2, []byte(content), 0644))
	require.NoError(t, os.WriteFile(file3, []byte(content), 0644))

	delRegex := regexp.MustCompile(`\.(jpg|png)$`)
	dupe := New(config.Config{
		Workers:  2,
		Delete:   true,
		DelMatch: delRegex,
	})

	require.NoError(t, dupe.IndexFiles([]string{tmpDir}))
	require.NoError(t, dupe.CalculateHashes())

	err := dupe.DeleteDuplicates()
	require.NoError(t, err)

	assert.FileExists(t, file1, "TXT file should be kept")
	assert.NoFileExists(t, file2, "JPG file should be deleted")
	assert.NoFileExists(t, file3, "PNG file should be deleted")
}

func TestDupe_DeleteDuplicates_ActualDelete_KeepMatch(t *testing.T) {
	tmpDir := t.TempDir()

	file1 := filepath.Join(tmpDir, "important_file.txt")
	file2 := filepath.Join(tmpDir, "temp_file.txt")
	file3 := filepath.Join(tmpDir, "cache_file.txt")
	content := "duplicate content"

	require.NoError(t, os.WriteFile(file1, []byte(content), 0644))
	require.NoError(t, os.WriteFile(file2, []byte(content), 0644))
	require.NoError(t, os.WriteFile(file3, []byte(content), 0644))

	keepRegex := regexp.MustCompile(`important`)
	dupe := New(config.Config{
		Workers:   2,
		Delete:    true,
		KeepMatch: keepRegex,
	})

	require.NoError(t, dupe.IndexFiles([]string{tmpDir}))
	require.NoError(t, dupe.CalculateHashes())

	err := dupe.DeleteDuplicates()
	require.NoError(t, err)

	assert.FileExists(t, file1, "Important file should be kept")
	assert.NoFileExists(t, file2, "Temp file should be deleted")
	assert.NoFileExists(t, file3, "Cache file should be deleted")
}

func TestDupe_DeleteDuplicates_OnlyOneCopy(t *testing.T) {
	tmpDir := t.TempDir()

	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")
	content := "duplicate content"

	require.NoError(t, os.WriteFile(file1, []byte(content), 0644))
	require.NoError(t, os.WriteFile(file2, []byte(content), 0644))

	dupe := New(config.Config{
		Workers:   2,
		Delete:    true,
		KeepFirst: true,
	})

	require.NoError(t, dupe.IndexFiles([]string{tmpDir}))
	require.NoError(t, dupe.CalculateHashes())

	err := dupe.DeleteDuplicates()
	require.NoError(t, err)

	remainingCount := 0
	if _, err := os.Stat(file1); err == nil {
		remainingCount++
	}
	if _, err := os.Stat(file2); err == nil {
		remainingCount++
	}

	assert.Equal(t, 1, remainingCount, "Exactly 1 file should remain")
}

func TestDupe_DeleteDuplicates_NoDuplicates(t *testing.T) {
	tmpDir := t.TempDir()

	file1 := filepath.Join(tmpDir, "unique1.txt")
	file2 := filepath.Join(tmpDir, "unique2.txt")

	require.NoError(t, os.WriteFile(file1, []byte("content 1"), 0644))
	require.NoError(t, os.WriteFile(file2, []byte("content 2"), 0644))

	dupe := New(config.Config{
		Workers:   2,
		Delete:    true,
		KeepFirst: true,
	})

	require.NoError(t, dupe.IndexFiles([]string{tmpDir}))
	require.NoError(t, dupe.CalculateHashes())

	err := dupe.DeleteDuplicates()
	require.NoError(t, err)

	assert.FileExists(t, file1, "Unique file1 should not be deleted")
	assert.FileExists(t, file2, "Unique file2 should not be deleted")
}
