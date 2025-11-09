package file

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSlice_Clone(t *testing.T) {
	original := Slice{
		&File{Path: "/path/1", Size: 100},
		&File{Path: "/path/2", Size: 200},
	}

	cloned := original.Clone()

	require.Len(t, cloned, len(original))

	for i := range original {
		assert.Equal(t, cloned[i], original[i], "Clone[%d] should point to same file", i)
	}

	cloned[0] = &File{Path: "/path/3", Size: 300}
	assert.NotEqual(t, original[0].Path, "/path/3", "Modifying clone should not affect original slice structure")
}

func TestSlice_SortByPath(t *testing.T) {
	tests := []struct {
		name  string
		input Slice
		want  []string
	}{
		{
			name: "already sorted",
			input: Slice{
				&File{Path: "/a"},
				&File{Path: "/b"},
				&File{Path: "/c"},
			},
			want: []string{"/a", "/b", "/c"},
		},
		{
			name: "reverse order",
			input: Slice{
				&File{Path: "/z"},
				&File{Path: "/m"},
				&File{Path: "/a"},
			},
			want: []string{"/a", "/m", "/z"},
		},
		{
			name: "mixed paths",
			input: Slice{
				&File{Path: "/home/user/file.txt"},
				&File{Path: "/etc/config"},
				&File{Path: "/var/log/app.log"},
			},
			want: []string{"/etc/config", "/home/user/file.txt", "/var/log/app.log"},
		},
		{
			name:  "empty slice",
			input: Slice{},
			want:  []string{},
		},
		{
			name: "single element",
			input: Slice{
				&File{Path: "/single"},
			},
			want: []string{"/single"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.input.SortByPath()

			require.Len(t, result, len(tt.want))

			for i, wantPath := range tt.want {
				assert.Equal(t, wantPath, result[i].Path, "Result[%d].Path should match expected", i)
			}
		})
	}
}

func TestSlice_SortByTime(t *testing.T) {
	now := time.Now()
	oneHourAgo := now.Add(-1 * time.Hour)
	twoDaysAgo := now.Add(-48 * time.Hour)

	tests := []struct {
		name      string
		input     Slice
		direction direction
		want      []time.Time
	}{
		{
			name: "ascending order",
			input: Slice{
				&File{Path: "/new", MTime: now},
				&File{Path: "/old", MTime: twoDaysAgo},
				&File{Path: "/middle", MTime: oneHourAgo},
			},
			direction: SortAscending,
			want:      []time.Time{twoDaysAgo, oneHourAgo, now},
		},
		{
			name: "descending order",
			input: Slice{
				&File{Path: "/old", MTime: twoDaysAgo},
				&File{Path: "/new", MTime: now},
				&File{Path: "/middle", MTime: oneHourAgo},
			},
			direction: SortDescending,
			want:      []time.Time{now, oneHourAgo, twoDaysAgo},
		},
		{
			name: "already sorted ascending",
			input: Slice{
				&File{Path: "/oldest", MTime: twoDaysAgo},
				&File{Path: "/newer", MTime: oneHourAgo},
				&File{Path: "/newest", MTime: now},
			},
			direction: SortAscending,
			want:      []time.Time{twoDaysAgo, oneHourAgo, now},
		},
		{
			name: "single element",
			input: Slice{
				&File{Path: "/only", MTime: now},
			},
			direction: SortAscending,
			want:      []time.Time{now},
		},
		{
			name:      "empty slice",
			input:     Slice{},
			direction: SortAscending,
			want:      []time.Time{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.input.SortByTime(tt.direction)

			require.Len(t, result, len(tt.want))

			for i, wantTime := range tt.want {
				assert.True(t, result[i].MTime.Equal(wantTime), "Result[%d].MTime should equal expected time", i)
			}
		})
	}
}

func TestMap_ToSlice(t *testing.T) {
	tests := []struct {
		name     string
		input    Map
		wantSize int
	}{
		{
			name: "multiple files",
			input: Map{
				"/path/1": &File{Path: "/path/1", Size: 100},
				"/path/2": &File{Path: "/path/2", Size: 200},
				"/path/3": &File{Path: "/path/3", Size: 300},
			},
			wantSize: 3,
		},
		{
			name:     "empty map",
			input:    Map{},
			wantSize: 0,
		},
		{
			name: "single file",
			input: Map{
				"/single": &File{Path: "/single", Size: 500},
			},
			wantSize: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.input.ToSlice()

			assert.Len(t, result, tt.wantSize, "ToSlice() should return expected number of elements")

			seen := make(map[string]bool)
			for _, file := range result {
				assert.Contains(t, tt.input, file.Path, "Result should contain only files from original map")
				assert.False(t, seen[file.Path], "Result should not contain duplicate file: %s", file.Path)
				seen[file.Path] = true
			}
		})
	}
}

func TestFile_Fields(t *testing.T) {
	now := time.Now()
	file := &File{
		Path:  "/test/path",
		Hash:  "testhash",
		Size:  1024,
		MTime: now,
		Mode:  os.FileMode(0644),
	}

	if file.Path != "/test/path" {
		t.Errorf("Path = %s, want /test/path", file.Path)
	}
	if file.Hash != "testhash" {
		t.Errorf("Hash = %s, want testhash", file.Hash)
	}
	if file.Size != 1024 {
		t.Errorf("Size = %d, want 1024", file.Size)
	}
	if !file.MTime.Equal(now) {
		t.Errorf("MTime = %v, want %v", file.MTime, now)
	}
	if file.Mode != os.FileMode(0644) {
		t.Errorf("Mode = %v, want 0644", file.Mode)
	}
}
