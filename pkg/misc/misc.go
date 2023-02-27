package misc

import (
	"io"
	"log"
	"os"

	"github.com/cespare/xxhash"
)

func Close(path string, file io.Closer) {
	if err := file.Close(); err != nil {
		log.Printf("Failed to close file '%s': %s", path, err)
	}
}

func Hash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer Close(path, f)

	h := xxhash.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return string(h.Sum(nil)), nil
}
