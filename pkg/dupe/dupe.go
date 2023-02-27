package dupe

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/lixmal/finddupes/pkg/config"
	"github.com/lixmal/finddupes/pkg/database"
	"github.com/lixmal/finddupes/pkg/file"
	"github.com/lixmal/finddupes/pkg/misc"
)

var ErrProcessStopped = errors.New("process was stopped")

type Dupe struct {
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}

	paths file.Map

	config   config.Config
	database *database.Database
}

func New(conf config.Config) *Dupe {
	ctx, cancel := context.WithCancel(context.Background())
	db := database.New()

	return &Dupe{
		ctx:      ctx,
		cancel:   cancel,
		done:     make(chan struct{}),
		config:   conf,
		database: db,
	}
}

func (d *Dupe) Done() <-chan struct{} {
	return d.done
}

func (d *Dupe) Stop() {
	d.cancel()
}

func (d *Dupe) ProcessFiles(filePaths []string) (err error) {
	defer close(d.done)

	if d.config.Path != "" {
		// ignore non-existent databases
		if err := d.ReadDatabase(); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("process files: %w", err)
		}
		d.VerifyDatabase()
	}

	defer func() {
		if d.config.Path != "" {
			if err2 := d.WriteDatabase(); err2 != nil {
				// overwriting return err value
				err = fmt.Errorf("process files: %w", err)
				return
			}
		}
	}()

	if err = d.IndexFiles(filePaths); err != nil {
		return fmt.Errorf("process files: index files: %w", err)
	}

	if err = d.CalculcateHashes(); err != nil {
		return fmt.Errorf("process files: calculate hashes: %w", err)
	}

	if !d.config.StoreOnly {
		if err = d.DeleteDuplicates(); err != nil {
			return fmt.Errorf("process files: delete duplicates: %w", err)
		}
	}

	return
}

func (d *Dupe) walkDir(path string, entry fs.DirEntry, err error) error {
	select {
	case <-d.ctx.Done():
		return ErrProcessStopped
	default:
	}

	if err != nil {
		return fmt.Errorf("walk: %w", err)
	}

	info, err := entry.Info()
	if err != nil {
		return fmt.Errorf("walk: info: %w", err)
	}

	// only regular files
	if info.Mode()&os.ModeType != 0 {
		return nil
	}

	if d.config.Verbose {
		fmt.Printf("Processing file %s\n", path)
	}
	size := info.Size()

	// ignore empty files
	if size == 0 {
		return nil
	}
	mtime := info.ModTime()

	// ignore duplicate paths
	if _, exists := d.paths[path]; exists {
		return nil
	}

	sys, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("not a syscall.Stat_t: %s", path)
	}

	// define all new files found with "need hash" (hash field: empty string)
	fil := &file.File{Path: path, Hash: "", Size: size, MTime: mtime, Mode: info.Mode(), Stat: sys}

	if d.database.Files[size] == nil {
		d.database.Files[size] = file.Map{}
	}
	d.database.Files[size][path] = fil

	return nil
}

func (d *Dupe) IndexFiles(filePaths []string) error {
	d.paths = file.Map{}
	defer func() {
		d.paths = nil
	}()

	// index already known paths, so we can identify duplicates later
	for _, list := range d.database.Files {
		for _, file := range list {
			d.paths[file.Path] = file
		}
	}

	for _, path := range filePaths {
		if err := filepath.WalkDir(path, d.walkDir); err == ErrProcessStopped {
			return err
		} else if err != nil {
			log.Println(err)
		}
	}

	return nil
}

func (d *Dupe) calculateHash(wg *sync.WaitGroup, jobs <-chan *file.File) {
	for fil := range jobs {
		if fil == nil {
			return
		}

		select {
		case <-d.ctx.Done():
			wg.Done()
			return
		default:
		}

		// hash already calculated and placed in database.hashes
		if fil.Hash != "" {
			wg.Done()
			continue
		}

		if d.config.Verbose {
			fmt.Printf("  Calculating hash for %s\n", fil.Path)
		}
		hash, err := misc.Hash(fil.Path)
		if err != nil {
			log.Println(err)
			wg.Done()
			continue
		}
		fil.Hash = hash

		d.database.Lock()
		if d.database.Hashes[hash] == nil {
			d.database.Hashes[hash] = file.Map{}
		}
		d.database.Hashes[hash][fil.Path] = fil
		if d.config.Verbose {
			fmt.Printf("  Path: %s\n", fil.Path)
			fmt.Printf("  Hash: %x\n", hash)
		}
		d.database.Unlock()

		wg.Done()
	}
}

func (d *Dupe) CalculcateHashes() (err error) {
	jobs := make(chan *file.File)

	var wg sync.WaitGroup
	// start workers
	for w := 1; w <= d.config.Workers; w++ {
		go d.calculateHash(&wg, jobs)
	}

	// distribute work
	// go through all files and see if we need to calculate hashes somewhere
outer:
	for size, files := range d.database.Files {
		// only process possible dupes (based on file size)
		length := len(files)
		if length < 2 {
			continue
		}

		if d.config.Verbose {
			fmt.Printf("Found %d elements for size %d\n", length, size)
		}

		for _, file := range files {
			select {
			case <-d.ctx.Done():
				err = ErrProcessStopped
				break outer
			default:
			}

			wg.Add(1)
			jobs <- file
		}
	}
	close(jobs)

	// wait for all workers to finish their work
	wg.Wait()

	return
}

func (d *Dupe) DeleteDuplicates() error {
	for hash, files := range d.database.Hashes {
		length := len(files)

		// no duplicates for this hash
		if length < 2 {
			continue
		}

		processed := 0
		fmt.Printf("Found %d elements for hash %x:\n", length, hash)

		fileSlice := files.ToSlice().SortByPath()

		for i, file := range fileSlice {
			select {
			case <-d.ctx.Done():
				return ErrProcessStopped
			default:
			}

			fmt.Printf("  %s\n", file.Path)

			// no duplicates left
			if length-processed < 2 {
				break
			}

			// no deletion rules matched
			if !d.matchRules(fileSlice, i, file) {
				continue
			}

			// add processed even if deletion fails, to be safe
			processed++

			if d.config.Delete {
				d.deleteFile(file)
			}

		}
	}

	return nil
}

func (d *Dupe) matchRules(fileSlice file.Slice, i int, fil *file.File) (matched bool) {
	switch {
	case d.config.KeepRecent && fil != fileSlice.Clone().SortByTime(file.SortDescending)[0]:
		fmt.Printf("  ↳ not most recent entry\n")
		matched = true
	case d.config.KeepOldest && fil != fileSlice.Clone().SortByTime(file.SortAscending)[0]:
		fmt.Printf("  ↳ not oldest entry\n")
		matched = true
	case d.config.KeepFirst && i != 0:
		fmt.Printf("  ↳ not first entry\n")
		matched = true
	case d.config.KeepLast && i != len(fileSlice)-1:
		fmt.Printf("  ↳ not last entry\n")
		matched = true
	case d.config.DelMatch != nil && d.config.DelMatch.MatchString(fil.Path):
		fmt.Printf("  ↳ matches del regex\n")
		matched = true
	case d.config.KeepMatch != nil && !d.config.KeepMatch.MatchString(fil.Path):
		fmt.Printf("  ↳ does not match keep regex\n")
		matched = true
	}
	return
}

func (d *Dupe) deleteFile(file *file.File) {
	fmt.Printf("  ↳ deleting...\n")
	if err := os.Remove(file.Path); err != nil {
		fmt.Printf("  ↳ error deleting %s\n", err)
	}

	if _, err := os.Stat(file.Path); err != nil {
		if d.database.Files[file.Size] != nil {
			delete(d.database.Files[file.Size], file.Path)
		}
		delete(d.database.Hashes[file.Hash], file.Path)
	}
}

func (d *Dupe) ReadDatabase() error {
	return d.database.Read(d.config.Path)
}

func (d *Dupe) WriteDatabase() error {
	return d.database.Write(d.config.Path)
}

func (d *Dupe) VerifyDatabase() {
	// check stored files for changes
	for hash, files := range d.database.Hashes {
		for _, fil := range files {
			path := fil.Path
			if info, err := os.Stat(path); err != nil {
				// doesn't exist or not accessible

				if d.config.Verbose {
					fmt.Printf("%s vanished or not accessible, removing\n", path)
				}
				delete(d.database.Hashes[hash], path)
				if d.database.Files[fil.Size] != nil {
					delete(d.database.Files[fil.Size], path)
				}

			} else if info.ModTime() != fil.MTime {
				// mtime changed, mark for hash recalculation

				if d.config.Verbose {
					fmt.Printf("Mtime of %s changed, need to recalculate hash\n", path)
				}

				// always remove first
				delete(d.database.Hashes[fil.Hash], path)
				if d.database.Files[fil.Size] != nil {
					delete(d.database.Files[fil.Size], path)
				}

				mode := info.Mode()
				size := info.Size()
				// remove if not a regular file anymore or size is 0
				if mode&os.ModeType != fil.Mode&os.ModeType || size == 0 {
					if d.config.Verbose {
						fmt.Printf("%s not a file anymore or file size 0, removing\n", path)
					}

					// don't read to map further below
					continue
				}

				sys, ok := info.Sys().(*syscall.Stat_t)
				if !ok {
					log.Printf("Warning: not a syscall.Stat_t: %s\n", path)
				}

				fil.MTime = info.ModTime()
				fil.Size = size
				fil.Hash = ""
				fil.Mode = mode
				fil.Stat = sys

				// add to new one
				if d.database.Files[size] == nil {
					d.database.Files[size] = file.Map{}
				}
				d.database.Files[size][path] = fil
			}
		}
	}
}
