# finddupes

Finds duplicate files based on hash and deletes them based on a given pattern.


## Background

`finddupes` tries to be efficient by

- comparing file size before running expensive hash calculations
- using hash tables to find duplicate sizes/hashes in constant time on avg
- using the fast [xxHash](https://github.com/Cyan4973/xxHash) algorithm to calculate hashes
- running things in parallel. However, this only really helps if directories to be searched for reside on different media
- using an optional "cache" that can be reused and extended for multiple searches/deletions

What does `finddupes` not do

- try to find very similar files (fuzzy search)

## Usage

Run finddupes with the `-help` flag to get all options:

    finddupes -help

The execution can be interrupted with `Ctrl-c`. This will gracefully finish all calculation
and write operations before shutting down.

### Find duplicates in given directories

This will list all found duplicates.

    finddupes <path> [path...]

Depending on the amount and size of files this can take a long time. For a large amount of files
it is recommended to index all duplicates and store them in a database file.
See next section.

### Index files

Index all files from given directories recursively and store them in a database file.

    finddupes -verbose -storeonly -path <db file path> <path> [path...]

e.g.

    finddupes -verbose -storeonly -path pics.db ~/Pictures ~/Videos ~/DCIM


After indexing files one or more actions can be run to delete duplicates.
A single last file will be always kept, regardless if there's a match or not.

The default is a dry run. To actually delete files, add the `-delete` flag.


Alternatively to indexing first, all actions can be run on the fly by not passing
the `-path <db file path>` parameter.

    finddupes -delmatch <pattern> ~/Pictures ~/Videos

See the next sections for a list of possible actions.


### Delete duplicates based on a pattern

Delete duplicates whose path matches the given regex.

    finddupes -path <db file path> -delmatch <pattern>

e.g.

    finddupes -path pics.db -delmatch '\.jpe?g$'


#### Keep duplicates based on a pattern

Keep duplicates whose path matches the given regex.

    finddupes -path <db file path> -keepmatch <pattern>

e.g.

    finddupes -path pics.db -keepmatch '_orignal$'


### Keep most recent duplicate

Keep the most recent duplicate, delete all others. Based on modification time (mtime).

    finddupes -path <db file path> -keeprecent


### Keep oldest duplicate

Keep the oldest duplicate, delete all others. Based on modification time (mtime).

    finddupes -path <db file path> -keepoldest


### Keep first duplicate

Keep the first duplicate based on lexically sorted file *paths* (not file names), delete all others.

    finddupes -path <db file path> -keepfirst


### Keep last duplicate

Keep the last duplicate based on lexically sorted file *paths* (not file names), delete all others.

    finddupes -path <db file path> -keeplast

