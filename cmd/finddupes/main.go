package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"regexp"
	"runtime"
	"syscall"

	"github.com/lixmal/finddupes/pkg/config"
	"github.com/lixmal/finddupes/pkg/dupe"
)

var (
	workers int = runtime.NumCPU()
)

var (
	storeonly = flag.Bool("storeonly", false, "store hashes to database without trying to find duplicates")

	delete  = flag.Bool("delete", false, "delete duplicates based on rules")
	verbose = flag.Bool("verbose", false, "enable verbose messages")

	path = flag.String("path", "", "path to the hash database, will be read/written to/from if specified")

	delmatch  = flag.String("delmatch", "", "delete duplicates files matching the given regex")
	keepmatch = flag.String("keepmatch", "", "delete all duplicate files except those matching the given regex")

	keepfirst = flag.Bool("keepfirst", false, "keep lexically first file and delete all others")
	keeplast  = flag.Bool("keeplast", false, "keep lexically last file and delete all others")

	keepoldest = flag.Bool("keepoldest", false, "keep oldest file and delete all others")
	keeprecent = flag.Bool("keeprecent", false, "keep most recent file and delete all others")
)

func init() {
	flag.Parse()
}

func main() {
	args := flag.Args()

	if *storeonly {
		if *path == "" {
			log.Fatal("Storeonly given, but no path specified\n")
		}
		if len(args) == 0 {
			log.Fatal("Storeonly given, but no directories provided\n")
		}
	}

	var reDelMatch *regexp.Regexp
	var reKeepMatch *regexp.Regexp
	if *delmatch != "" {
		reDelMatch = regexp.MustCompile(*delmatch)
	}
	if *keepmatch != "" {
		reKeepMatch = regexp.MustCompile(*keepmatch)
	}

	conf := config.Config{
		StoreOnly:  *storeonly,
		Path:       *path,
		Delete:     *delete,
		Verbose:    *verbose,
		DelMatch:   reDelMatch,
		KeepMatch:  reKeepMatch,
		KeepFirst:  *keepfirst,
		KeepLast:   *keeplast,
		KeepOldest: *keepoldest,
		KeepRecent: *keeprecent,
		Workers:    workers,
	}

	dup := dupe.New(conf)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGHUP)

	go func() {
		sig := <-sigs
		fmt.Printf("\n>>>>> got %s, finishing up <<<<<\n", sig)
		dup.Stop()
	}()

	if err := dup.ProcessFiles(args); err != nil && !errors.Is(err, dupe.ErrProcessStopped) {
		log.Fatalf("Failed to process files: %s\n", err)
	}
}
