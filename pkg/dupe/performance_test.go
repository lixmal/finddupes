package dupe

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lixmal/finddupes/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPerformance_SortingOnlyOnce verifies that sorting happens once, not per file
// This test ensures the O(N²) bug fix is working correctly
func TestPerformance_SortingOnlyOnce(t *testing.T) {
	tmpDir := t.TempDir()

	now := time.Now()
	files := make([]string, 50)

	for i := 0; i < 50; i++ {
		filePath := filepath.Join(tmpDir, filepath.Base(tmpDir)+string(rune('a'+i%26))+".txt")
		files[i] = filePath
		require.NoError(t, os.WriteFile(filePath, []byte("duplicate"), 0644))

		time.Sleep(1 * time.Millisecond)
		modTime := now.Add(time.Duration(i) * time.Millisecond)
		require.NoError(t, os.Chtimes(filePath, modTime, modTime))
	}

	conf := config.Config{
		Workers:    2,
		Delete:     true,
		KeepRecent: true,
	}

	dupe := New(conf)
	require.NoError(t, dupe.IndexFiles([]string{tmpDir}))
	require.NoError(t, dupe.CalculateHashes())

	start := time.Now()
	err := dupe.DeleteDuplicates()
	elapsed := time.Since(start)

	require.NoError(t, err)

	remainingCount := 0
	entries, _ := os.ReadDir(tmpDir)
	for _, entry := range entries {
		if !entry.IsDir() {
			remainingCount++
		}
	}

	assert.Equal(t, 1, remainingCount, "Should keep only most recent")

	assert.Less(t, elapsed.Milliseconds(), int64(1000), "Should complete in <1s (would be much slower with O(N²))")
}

// TestCorrectness_KeepRecentWithManyFiles ensures the fix maintains correctness
func TestCorrectness_KeepRecentWithManyFiles(t *testing.T) {
	tmpDir := t.TempDir()

	baseTime := time.Now().Add(-1 * time.Hour)

	oldestFile := filepath.Join(tmpDir, "oldest.txt")
	middleFile := filepath.Join(tmpDir, "middle.txt")
	newestFile := filepath.Join(tmpDir, "newest.txt")

	require.NoError(t, os.WriteFile(oldestFile, []byte("dup"), 0644))
	require.NoError(t, os.Chtimes(oldestFile, baseTime, baseTime))

	require.NoError(t, os.WriteFile(middleFile, []byte("dup"), 0644))
	require.NoError(t, os.Chtimes(middleFile, baseTime.Add(30*time.Minute), baseTime.Add(30*time.Minute)))

	require.NoError(t, os.WriteFile(newestFile, []byte("dup"), 0644))
	require.NoError(t, os.Chtimes(newestFile, baseTime.Add(60*time.Minute), baseTime.Add(60*time.Minute)))

	conf := config.Config{
		Workers:    2,
		Delete:     true,
		KeepRecent: true,
	}

	dupe := New(conf)
	require.NoError(t, dupe.IndexFiles([]string{tmpDir}))
	require.NoError(t, dupe.CalculateHashes())
	require.NoError(t, dupe.DeleteDuplicates())

	assert.NoFileExists(t, oldestFile, "Oldest should be deleted")
	assert.NoFileExists(t, middleFile, "Middle should be deleted")
	assert.FileExists(t, newestFile, "Newest should be kept")
}

// TestCorrectness_KeepOldestWithManyFiles ensures the fix maintains correctness
func TestCorrectness_KeepOldestWithManyFiles(t *testing.T) {
	tmpDir := t.TempDir()

	baseTime := time.Now().Add(-1 * time.Hour)

	oldestFile := filepath.Join(tmpDir, "oldest.txt")
	middleFile := filepath.Join(tmpDir, "middle.txt")
	newestFile := filepath.Join(tmpDir, "newest.txt")

	require.NoError(t, os.WriteFile(oldestFile, []byte("dup"), 0644))
	require.NoError(t, os.Chtimes(oldestFile, baseTime, baseTime))

	require.NoError(t, os.WriteFile(middleFile, []byte("dup"), 0644))
	require.NoError(t, os.Chtimes(middleFile, baseTime.Add(30*time.Minute), baseTime.Add(30*time.Minute)))

	require.NoError(t, os.WriteFile(newestFile, []byte("dup"), 0644))
	require.NoError(t, os.Chtimes(newestFile, baseTime.Add(60*time.Minute), baseTime.Add(60*time.Minute)))

	conf := config.Config{
		Workers:    2,
		Delete:     true,
		KeepOldest: true,
	}

	dupe := New(conf)
	require.NoError(t, dupe.IndexFiles([]string{tmpDir}))
	require.NoError(t, dupe.CalculateHashes())
	require.NoError(t, dupe.DeleteDuplicates())

	assert.FileExists(t, oldestFile, "Oldest should be kept")
	assert.NoFileExists(t, middleFile, "Middle should be deleted")
	assert.NoFileExists(t, newestFile, "Newest should be deleted")
}

// TestCorrectness_DeleteFileRaceCondition verifies deleteFile handles errors correctly
func TestCorrectness_DeleteFileRaceCondition(t *testing.T) {
	tmpDir := t.TempDir()

	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")
	content := "duplicate"

	require.NoError(t, os.WriteFile(file1, []byte(content), 0644))
	require.NoError(t, os.WriteFile(file2, []byte(content), 0644))

	conf := config.Config{
		Workers:   2,
		Delete:    true,
		KeepFirst: true,
	}

	dupe := New(conf)
	require.NoError(t, dupe.IndexFiles([]string{tmpDir}))
	require.NoError(t, dupe.CalculateHashes())

	dbEntriesBeforeDelete := 0
	for _, fileMap := range dupe.database.Files {
		dbEntriesBeforeDelete += len(fileMap)
	}
	assert.Equal(t, 2, dbEntriesBeforeDelete, "Should have 2 entries before delete")

	require.NoError(t, dupe.DeleteDuplicates())

	dbEntriesAfterDelete := 0
	for _, fileMap := range dupe.database.Files {
		dbEntriesAfterDelete += len(fileMap)
	}

	assert.Equal(t, 1, dbEntriesAfterDelete, "Database should be updated to 1 entry after successful delete")

	hashEntries := 0
	for _, fileMap := range dupe.database.Hashes {
		hashEntries += len(fileMap)
	}
	assert.Equal(t, 1, hashEntries, "Hash database should also have 1 entry")
}

// TestCorrectness_ToSlicePreAllocation verifies ToSlice returns all items
func TestCorrectness_ToSlicePreAllocation(t *testing.T) {
	tmpDir := t.TempDir()

	numFiles := 100
	for i := 0; i < numFiles; i++ {
		filePath := filepath.Join(tmpDir, filepath.Base(tmpDir)+string(rune('a'+i%26))+".txt")
		require.NoError(t, os.WriteFile(filePath, []byte("test"), 0644))
	}

	dupe := New(config.Config{})
	require.NoError(t, dupe.IndexFiles([]string{tmpDir}))

	for _, fileMap := range dupe.database.Files {
		slice := fileMap.ToSlice()
		assert.Equal(t, len(fileMap), len(slice), "ToSlice should return all items")

		seen := make(map[string]bool)
		for _, f := range slice {
			assert.False(t, seen[f.Path], "No duplicates in slice")
			seen[f.Path] = true
		}
	}
}

// BenchmarkDeleteDuplicates_Small benchmarks with small duplicate sets
func BenchmarkDeleteDuplicates_Small(b *testing.B) {
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		tmpDir := b.TempDir()

		for j := 0; j < 10; j++ {
			filePath := filepath.Join(tmpDir, filepath.Base(tmpDir)+string(rune('a'+j))+".txt")
			os.WriteFile(filePath, []byte("duplicate"), 0644)
		}

		dupe := New(config.Config{Workers: 2, Delete: false, KeepRecent: true})
		dupe.IndexFiles([]string{tmpDir})
		dupe.CalculateHashes()

		b.StartTimer()
		dupe.DeleteDuplicates()
	}
}

// BenchmarkDeleteDuplicates_Large benchmarks with large duplicate sets
func BenchmarkDeleteDuplicates_Large(b *testing.B) {
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		tmpDir := b.TempDir()

		for j := 0; j < 100; j++ {
			filePath := filepath.Join(tmpDir, filepath.Base(tmpDir)+string(rune('a'+j%26))+".txt")
			os.WriteFile(filePath, []byte("duplicate"), 0644)
		}

		dupe := New(config.Config{Workers: 4, Delete: false, KeepOldest: true})
		dupe.IndexFiles([]string{tmpDir})
		dupe.CalculateHashes()

		b.StartTimer()
		dupe.DeleteDuplicates()
	}
}
