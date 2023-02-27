package config

import "regexp"

const (
	ModeOnTheFly = iota
	ModeRead
	ModeStore
)

type Config struct {
	StoreOnly  bool
	Path       string
	Delete     bool
	Verbose    bool
	DelMatch   *regexp.Regexp
	KeepMatch  *regexp.Regexp
	KeepFirst  bool
	KeepLast   bool
	KeepOldest bool
	KeepRecent bool
	Workers    int
}
