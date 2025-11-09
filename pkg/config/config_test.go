package config

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfig_DefaultValues(t *testing.T) {
	conf := Config{}

	assert.False(t, conf.StoreOnly, "StoreOnly should default to false")
	assert.False(t, conf.Delete, "Delete should default to false")
	assert.False(t, conf.Verbose, "Verbose should default to false")
	assert.False(t, conf.KeepFirst, "KeepFirst should default to false")
	assert.False(t, conf.KeepLast, "KeepLast should default to false")
	assert.False(t, conf.KeepOldest, "KeepOldest should default to false")
	assert.False(t, conf.KeepRecent, "KeepRecent should default to false")
	assert.Empty(t, conf.Path, "Path should default to empty string")
	assert.Equal(t, 0, conf.Workers, "Workers should default to 0")
	assert.Nil(t, conf.DelMatch, "DelMatch should default to nil")
	assert.Nil(t, conf.KeepMatch, "KeepMatch should default to nil")
}

func TestConfig_WithValues(t *testing.T) {
	delRegex := regexp.MustCompile(`\.txt$`)
	keepRegex := regexp.MustCompile(`important`)

	conf := Config{
		StoreOnly:  true,
		Path:       "/tmp/test.db",
		Delete:     true,
		Verbose:    true,
		DelMatch:   delRegex,
		KeepMatch:  keepRegex,
		KeepFirst:  true,
		KeepLast:   false,
		KeepOldest: false,
		KeepRecent: true,
		Workers:    8,
	}

	assert.True(t, conf.StoreOnly, "StoreOnly should be true")
	assert.Equal(t, "/tmp/test.db", conf.Path, "Path should match")
	assert.True(t, conf.Delete, "Delete should be true")
	assert.True(t, conf.Verbose, "Verbose should be true")
	assert.NotNil(t, conf.DelMatch, "DelMatch should not be nil")
	assert.NotNil(t, conf.KeepMatch, "KeepMatch should not be nil")
	assert.True(t, conf.KeepFirst, "KeepFirst should be true")
	assert.False(t, conf.KeepLast, "KeepLast should be false")
	assert.True(t, conf.KeepRecent, "KeepRecent should be true")
	assert.Equal(t, 8, conf.Workers, "Workers should be 8")
}

func TestConfig_RegexMatching(t *testing.T) {
	tests := []struct {
		name        string
		pattern     string
		testPath    string
		shouldMatch bool
	}{
		{
			name:        "match jpg extension",
			pattern:     `\.jpg$`,
			testPath:    "/photos/image.jpg",
			shouldMatch: true,
		},
		{
			name:        "no match jpg extension",
			pattern:     `\.jpg$`,
			testPath:    "/photos/image.png",
			shouldMatch: false,
		},
		{
			name:        "match directory pattern",
			pattern:     `/temp/`,
			testPath:    "/home/user/temp/file.txt",
			shouldMatch: true,
		},
		{
			name:        "no match directory pattern",
			pattern:     `/temp/`,
			testPath:    "/home/user/documents/file.txt",
			shouldMatch: false,
		},
		{
			name:        "match any extension",
			pattern:     `\.(jpg|png|gif)$`,
			testPath:    "/images/photo.png",
			shouldMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			regex := regexp.MustCompile(tt.pattern)
			conf := Config{
				DelMatch: regex,
			}

			matched := conf.DelMatch.MatchString(tt.testPath)
			assert.Equal(t, tt.shouldMatch, matched,
				"Pattern %s against %s should match=%v", tt.pattern, tt.testPath, tt.shouldMatch)
		})
	}
}

func TestModeConstants(t *testing.T) {
	assert.Equal(t, 0, ModeOnTheFly, "ModeOnTheFly should be 0")
	assert.Equal(t, 1, ModeRead, "ModeRead should be 1")
	assert.Equal(t, 2, ModeStore, "ModeStore should be 2")
}

func TestConfig_MultipleKeepRules(t *testing.T) {
	conf := Config{
		KeepFirst:  true,
		KeepLast:   true,
		KeepOldest: true,
		KeepRecent: true,
	}

	ruleCount := 0
	if conf.KeepFirst {
		ruleCount++
	}
	if conf.KeepLast {
		ruleCount++
	}
	if conf.KeepOldest {
		ruleCount++
	}
	if conf.KeepRecent {
		ruleCount++
	}

	assert.Equal(t, 4, ruleCount, "All keep rules should be settable independently")
}

func TestConfig_NilRegex(t *testing.T) {
	conf := Config{
		DelMatch:  nil,
		KeepMatch: nil,
	}

	assert.Nil(t, conf.DelMatch, "DelMatch should be nil")
	assert.Nil(t, conf.KeepMatch, "KeepMatch should be nil")
}

func TestConfig_WorkersRange(t *testing.T) {
	tests := []struct {
		name    string
		workers int
	}{
		{"zero workers", 0},
		{"one worker", 1},
		{"multiple workers", 4},
		{"many workers", 16},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conf := Config{
				Workers: tt.workers,
			}
			assert.Equal(t, tt.workers, conf.Workers, "Workers should match expected value")
		})
	}
}
