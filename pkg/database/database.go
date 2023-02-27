package database

import (
	"encoding/gob"
	"fmt"
	"os"
	"sync"

	"github.com/lixmal/finddupes/pkg/file"
	"github.com/lixmal/finddupes/pkg/misc"
)

type Database struct {
	Files  map[int64]file.Map
	Hashes map[string]file.Map
	mutex  sync.Mutex
}

func New() *Database {
	return &Database{
		Files:  map[int64]file.Map{},
		Hashes: map[string]file.Map{},
		mutex:  sync.Mutex{},
	}
}

func (d *Database) Write(path string) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("write database: %w", err)
	}
	defer file.Close()

	if err := gob.NewEncoder(file).Encode(d); err != nil {
		return fmt.Errorf("write database: %w", err)
	}

	// explicit close to catch any errors writing
	if err = file.Close(); err != nil {
		return fmt.Errorf("write database: %w", err)
	}

	return nil
}

func (d *Database) Read(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("read database: %w", err)
	}
	defer misc.Close(path, file)

	// TODO: fix reading db from interface
	var db Database
	if err := gob.NewDecoder(file).Decode(&db); err != nil {
		return fmt.Errorf("read database: %w", err)
	}

	*d = db

	return nil
}

func (d *Database) Lock() {
	d.mutex.Lock()
}

func (d *Database) Unlock() {
	d.mutex.Unlock()
}
