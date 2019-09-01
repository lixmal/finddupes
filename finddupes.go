package main

import (
    "os"
    "fmt"
    "log"
    "github.com/cespare/xxhash"
    "io"
    "sync"
    "runtime"
    "path/filepath"
    "encoding/gob"
    "flag"
    "regexp"
    "time"
    "syscall"
    "os/signal"
)


const (
    workers int = 10
    jobcount    = 100
)

var readDb     = flag.String("readdb", "", "file path to the stored hash database. If path parameters are provided, found duplicates are appended to this database")
var writeDb    = flag.String("writedb", "", "file path to store the generated hash database. Execution stops after (new) file data was written to the database")
var delmatch   = flag.String("delmatch", "", "delete duplicates files matching the given regex")
var keepmatch  = flag.String("keepmatch", "", "delete all duplicate files except those matching the given regex")
var keepfirst  = flag.Bool("keepfirst", false, "keep first found file and delete all others")
var keeplast   = flag.Bool("keeplast", false, "keep last found file and delete all others")
var keepoldest = flag.Bool("keepoldest", false, "keep oldest file and delete all others")
var keeprecent = flag.Bool("keeprecent", false, "keep most recent file and delete all others")
var dry        = flag.Bool("dry", false, "don't actually delete files")
var verbose    = flag.Bool("verbose", false, "enable verbose messages")

var delre *regexp.Regexp
var keepre *regexp.Regexp
var sigs chan os.Signal
var sigMutex sync.Mutex


type File struct {
    Path string
    Hash string
    Size int64
    MTime time.Time
    Mode os.FileMode
    Stat *syscall.Stat_t
}

type Database struct {
    Files map[int64]map[string]*File
    Hashes map[string]map[string]*File
    mutex sync.Mutex
}

func (d *Database) Lock() {
    d.mutex.Lock()
}

func (d *Database) Unlock() {
    d.mutex.Unlock()
}

func init() {
    runtime.GOMAXPROCS(runtime.NumCPU())
    flag.Parse()

    if *delmatch != "" {
        delre = regexp.MustCompile(*delmatch)
    }
    if *keepmatch != "" {
        keepre = regexp.MustCompile(*keepmatch)
    }

    sigs = make(chan os.Signal, 1)
    signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

    go func() {
        sig := <-sigs
        fmt.Printf("\n%s, cleaning up\n", sig)
        sigMutex.Lock()
        os.Exit(0)
    }()
}

func main() {
    args := flag.Args()

    database := &Database{
        map[int64]map[string]*File{},
        map[string]map[string]*File{},
        sync.Mutex{},
    }

    if (*readDb != "") {
        read_database(readDb, database)
    } else if len(args) == 0 {
        log.Fatal("No parameters and no -readdb given\n")
    }

    if len(args) != 0 {
        paths := map[string]*File{}

        // index already known paths, so we can identify duplicates later
        for _, list := range database.Files {
            for _, file := range list {
                paths[file.Path] = file
            }
        }

        for _, path := range args {
            err := filepath.Walk(
                path,
                func(path string, info os.FileInfo, err error) error {
                    if err != nil {
                        return err
                    }

                    // only regular files
                    if info.Mode() & os.ModeType == 0 {
                        if *verbose {
                            fmt.Printf("Processing file %s\n", path)
                        }
                        size := info.Size()
                        mtime := info.ModTime()

                        // ignore duplicate paths and empty files
                        if file, exists := paths[path]; !exists && size != 0 {
                            sys, ok := info.Sys().(*syscall.Stat_t)
                            if !ok {
                                return fmt.Errorf("Not a syscall.Stat_t: %s", path)
                            }

                            // define all new files found with "need hash" (hash field: empty string)
                            file = &File{path, "", size, mtime, info.Mode(), sys}

                            if database.Files[size] == nil {
                                database.Files[size] = map[string]*File{}
                            }
                            database.Files[size][path] = file
                        }
                    }

                    return nil
                },
            )
            if err != nil {
                log.Println(err)
            }
        }
    }


    jobs := make(chan map[string]*File, jobcount)

    var wg sync.WaitGroup
    // start workers
    for w := 1; w <= workers; w++ {
        go func() {
            for list := range jobs {
                for _, file := range list {
                    // hash already calculated and placed in database.hashes
                    if file.Hash != "" {
                        continue
                    }

                    if *verbose {
                        fmt.Printf("  Calculating hash for %s\n", file.Path)
                    }
                    hash, err := hash(file.Path)
                    if (err != nil) {
                        log.Println(err)
                        continue
                    }
                    file.Hash = hash

                    database.Lock()
                    if database.Hashes[hash] == nil {
                        database.Hashes[hash] = map[string]*File{}
                    }
                    database.Hashes[hash][file.Path] = file
                    if *verbose {
                        fmt.Printf("  Path: %s\n", file.Path)
                        fmt.Printf("  Hash: %x\n", hash)
                    }
                    database.Unlock()
                }
                wg.Done()
            }
        }()
    }

    // distribute work
    // go through all files and see if we need to calculate hashes somewhere
    for size, list := range database.Files {
        // only process possible dupes (based on file size)
        length := len(list)
        if (length > 1) {
            if *verbose {
                fmt.Printf("Found %d elements for size %d\n", length, size)
            }
            wg.Add(1)
            jobs <- list
        }
    }
    close(jobs)

    // wait for all workers to finish their work
    wg.Wait()

    // writeDb given => no further processing
    if (*writeDb != "") {
        writeGob(*writeDb, database)
        os.Exit(0)
    }

    for hash, list := range database.Hashes {
        length := len(list)
        deleted := 0
        if (length > 1) {
            fmt.Printf("Found %d elements for hash %x:\n", length, hash)
            for _, file := range list {
                fmt.Printf("  %s\n", file.Path)

                if length - deleted > 1 {
                    del := false
                    if *delmatch != "" && delre.MatchString(file.Path) {
                        fmt.Printf("  * Path %s matches del regex, deleting...\n", file.Path)
                        del = true
                    } else if *keepmatch != "" && !keepre.MatchString(file.Path) {
                        fmt.Printf("  * Path %s doesn't match keep regex, deleting...\n", file.Path)
                        del = true
                    }

                    if del {
                        if *dry {
                            deleted++;
                        } else {
                            err := os.Remove(file.Path)
                            if err != nil {
                                fmt.Printf("  * Error deleting %s: %s\n", file.Path, err)
                            }

                            // add to deleted if file isn't accessible anymore under any circumstances
                            if _, err := os.Stat(file.Path); err != nil {
                                deleted++

                                if database.Files[file.Size] != nil {
                                    delete(database.Files[file.Size], file.Path)
                                }
                                delete(database.Hashes[file.Hash], file.Path)
                            }
                        }
                    }
                }
            }
            if length - deleted == 1 {
                fmt.Println("  * Done - only one file left")
            }
        }
    }

    // store back in read database
    if (*readDb != "") {
        writeGob(*readDb, database)
    }
}

func hash(path string) (string, error) {
    f, err := os.Open(path)
    defer f.Close()
    if err != nil {
        return "", err
    }

    h := xxhash.New()
    if _, err := io.Copy(h, f); err != nil {
        return "", err
    }

    return string(h.Sum(nil)), nil
}

func read_database(readDb *string, database *Database) {
    readGob(*readDb, database)

    // check stored files for changes
    for hash, list := range database.Hashes {
        for _, file := range list {
            path := file.Path
            // doesn't exist or not accessible
            if info, err := os.Stat(path); err != nil {
                if *verbose {
                    fmt.Printf("%s vanished or not accessible, removing\n", path)
                }
                delete(database.Hashes[hash], path)
                if database.Files[file.Size] != nil {
                    delete(database.Files[file.Size], path)
                }
            // mtime changed
            } else if info.ModTime() != file.MTime {
                if *verbose {
                    fmt.Printf("Mtime of %s changed, need to recalculate hash\n", path)
                }

                // always remove first
                delete(database.Hashes[file.Hash], path)
                if database.Files[file.Size] != nil {
                    delete(database.Files[file.Size], path)
                }

                mode := info.Mode()
                size := info.Size()
                // remove if not a regular file anymore or size is 0
                if mode & os.ModeType != file.Mode & os.ModeType || size == 0 {
                    if *verbose {
                        fmt.Printf("%s not a file anymore or file size 0, removing\n", path)
                    }

                    // don't readd to map further below
                    continue
                }

                sys, ok := info.Sys().(*syscall.Stat_t)
                if !ok {
                    log.Println("Warning: not a syscall.Stat_t: %s", path)
                }

                file.MTime = info.ModTime()
                file.Size = size
                file.Hash = ""
                file.Mode = mode
                file.Stat = sys

                // add to new one
                if database.Files[size] == nil {
                    database.Files[size] = map[string]*File{}
                }
                database.Files[size][path] = file
            }
        }
    }
}

func writeGob(filePath string, object interface{}) {
    // lock signal mutex so we can finish writing
    sigMutex.Lock()
    defer sigMutex.Unlock()

    file, err := os.Create(filePath)
    defer file.Close()

    if err != nil {
        log.Fatal(err)
    }

    encoder := gob.NewEncoder(file)
    err = encoder.Encode(object)
    if err != nil {
        log.Fatal(err)
    }
}

func readGob(filePath string, object interface{}) {
    file, err := os.Open(filePath)
    defer file.Close()

    if err != nil {
        log.Fatal(err)
    }

    decoder := gob.NewDecoder(file)
    err = decoder.Decode(object)
    if err != nil {
        log.Fatal(err)
    }
}
