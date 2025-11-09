package database

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lixmal/finddupes/pkg/file"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	db := New()

	require.NotNil(t, db, "New() should return a valid database")
	assert.NotNil(t, db.Files, "Files map should be initialized")
	assert.NotNil(t, db.Hashes, "Hashes map should be initialized")
	assert.Empty(t, db.Files, "Files map should be empty")
	assert.Empty(t, db.Hashes, "Hashes map should be empty")
}

func TestDatabase_WriteAndRead(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	originalDB := New()
	originalDB.Files[1024] = file.Map{
		"/path/file1": &file.File{
			Path:  "/path/file1",
			Hash:  "hash1",
			Size:  1024,
			MTime: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			Mode:  os.FileMode(0644),
		},
	}
	originalDB.Hashes["hash1"] = file.Map{
		"/path/file1": originalDB.Files[1024]["/path/file1"],
	}

	err := originalDB.Write(dbPath)
	require.NoError(t, err)

	_, err = os.Stat(dbPath)
	assert.NoError(t, err, "Database file should be created")

	readDB := New()
	err = readDB.Read(dbPath)
	require.NoError(t, err)

	assert.Len(t, readDB.Files, len(originalDB.Files), "Files length should match")
	assert.Len(t, readDB.Hashes, len(originalDB.Hashes), "Hashes length should match")

	readFile := readDB.Files[1024]["/path/file1"]
	originalFile := originalDB.Files[1024]["/path/file1"]

	assert.Equal(t, originalFile.Path, readFile.Path, "Path should match")
	assert.Equal(t, originalFile.Hash, readFile.Hash, "Hash should match")
	assert.Equal(t, originalFile.Size, readFile.Size, "Size should match")
}

func TestDatabase_WriteAndRead_MultipleFiles(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "multi.db")

	originalDB := New()

	file1 := &file.File{Path: "/path/file1", Hash: "hash1", Size: 1024, MTime: time.Now()}
	file2 := &file.File{Path: "/path/file2", Hash: "hash1", Size: 1024, MTime: time.Now()}
	file3 := &file.File{Path: "/path/file3", Hash: "hash2", Size: 2048, MTime: time.Now()}

	originalDB.Files[1024] = file.Map{
		"/path/file1": file1,
		"/path/file2": file2,
	}
	originalDB.Files[2048] = file.Map{
		"/path/file3": file3,
	}
	originalDB.Hashes["hash1"] = file.Map{
		"/path/file1": file1,
		"/path/file2": file2,
	}
	originalDB.Hashes["hash2"] = file.Map{
		"/path/file3": file3,
	}

	err := originalDB.Write(dbPath)
	require.NoError(t, err)

	readDB := New()
	err = readDB.Read(dbPath)
	require.NoError(t, err)

	assert.Len(t, readDB.Files[1024], 2, "Should have 2 files of size 1024")
	assert.Len(t, readDB.Files[2048], 1, "Should have 1 file of size 2048")
	assert.Len(t, readDB.Hashes["hash1"], 2, "Should have 2 files with hash1")
	assert.Len(t, readDB.Hashes["hash2"], 1, "Should have 1 file with hash2")
}

func TestDatabase_Read_NonExistent(t *testing.T) {
	db := New()
	err := db.Read("/path/that/does/not/exist.db")

	assert.Error(t, err, "Should return error when reading non-existent file")
}

func TestDatabase_Write_InvalidPath(t *testing.T) {
	db := New()
	err := db.Write("/invalid/path/that/cannot/exist/test.db")

	assert.Error(t, err, "Should return error when writing to invalid path")
}

func TestDatabase_Write_EmptyDatabase(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "empty.db")

	db := New()
	err := db.Write(dbPath)
	require.NoError(t, err)

	readDB := New()
	err = readDB.Read(dbPath)
	require.NoError(t, err)

	assert.Empty(t, readDB.Files, "Files map should be empty")
	assert.Empty(t, readDB.Hashes, "Hashes map should be empty")
}

func TestDatabase_LockUnlock(t *testing.T) {
	db := New()

	done := make(chan bool)
	locked := false
	go func() {
		db.Lock()
		locked = true
		time.Sleep(50 * time.Millisecond)
		db.Unlock()
		done <- true
	}()

	time.Sleep(10 * time.Millisecond)
	db.Lock()
	assert.True(t, locked, "Goroutine should have acquired lock first")
	db.Unlock()

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		assert.Fail(t, "Lock/Unlock mechanism not working properly")
	}
}

func TestDatabase_ConcurrentAccess(t *testing.T) {
	db := New()
	iterations := 100

	done := make(chan bool, 2)

	go func() {
		for i := 0; i < iterations; i++ {
			db.Lock()
			db.Files[int64(i)] = file.Map{}
			db.Unlock()
		}
		done <- true
	}()

	go func() {
		for i := 0; i < iterations; i++ {
			db.Lock()
			db.Hashes[string(rune(i))] = file.Map{}
			db.Unlock()
		}
		done <- true
	}()

	<-done
	<-done

	assert.Len(t, db.Files, iterations, "Files should have expected number of entries")
	assert.Len(t, db.Hashes, iterations, "Hashes should have expected number of entries")
}

func TestDatabase_Write_Overwrite(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "overwrite.db")

	db1 := New()
	db1.Files[1024] = file.Map{
		"/file1": &file.File{Path: "/file1", Hash: "hash1", Size: 1024},
	}

	err := db1.Write(dbPath)
	require.NoError(t, err)

	db2 := New()
	db2.Files[2048] = file.Map{
		"/file2": &file.File{Path: "/file2", Hash: "hash2", Size: 2048},
	}

	err = db2.Write(dbPath)
	require.NoError(t, err)

	readDB := New()
	err = readDB.Read(dbPath)
	require.NoError(t, err)

	assert.Empty(t, readDB.Files[1024], "Old data should be overwritten")
	assert.Len(t, readDB.Files[2048], 1, "New data should be present")
}
