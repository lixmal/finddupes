package file

import (
	"os"
	"sort"
	"syscall"
	"time"
)

type direction int

const (
	SortAscending direction = iota
	SortDescending
)

type File struct {
	Path  string
	Hash  string
	Size  int64
	MTime time.Time
	Mode  os.FileMode
	Stat  *syscall.Stat_t
}

type Slice []*File

func (s Slice) Clone() Slice {
	c := make(Slice, len(s))
	copy(c, s)
	return c
}

// Sort slice by path lexically by ascending order
func (s Slice) SortByPath() Slice {
	sort.Slice(s, func(i, j int) bool {
		return s[i].Path < s[j].Path
	})
	return s
}

// Sort slice by mod time by ascending order (oldest first) or descending order (youngest first)
func (s Slice) SortByTime(dir direction) Slice {
	sort.Slice(s, func(i, j int) bool {
		if dir == SortAscending {
			return s[i].MTime.Before(s[j].MTime)
		} else {
			return s[i].MTime.After(s[j].MTime)
		}
	})
	return s
}

type Map map[string]*File

func (m Map) ToSlice() Slice {
	s := make(Slice, 0, len(m))
	for _, v := range m {
		s = append(s, v)
	}
	return s
}
