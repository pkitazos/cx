package main

import (
	"fmt"
	"os"
	"time"

	"github.com/dustin/go-humanize"
)

// FormatLastModTime returns a formatted string containing the human-readable
// last modified time of the provided os.FileInfo.
func FormatLastModTime(modTime time.Time) string {
	fileLastModified := humanize.Time(modTime)
	return fmt.Sprintf("(%s)", fileLastModified)
}

// FormatLastModTime returns a formatted string containing the human-readable
// size of the provided os.FileInfo.
func FormatSize(fileInfo os.FileInfo) string {
	return humanize.Bytes(uint64(fileInfo.Size()))
}
