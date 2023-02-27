package file

import (
	"os"
	"syscall"
	"time"
)

type File struct {
	Path  string
	Hash  string
	Size  int64
	MTime time.Time
	Mode  os.FileMode
	Stat  *syscall.Stat_t
}

type Map map[string]*File
