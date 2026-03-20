package main

import (
	"fmt"
	"time"

	"github.com/dustin/go-humanize"
)

// FormatCutAtTime returns a formatted string containing the human-readable
// last modified time of the provided os.FileInfo.
func FormatCutAtTime(t time.Time) string {
	fileLastModified := humanize.Time(t)
	return fmt.Sprintf("(%s)", fileLastModified)
}

// FormatLastModTime returns a formatted string containing the human-readable
// size of the provided os.FileInfo.
func FormatSize(size int64) string {
	return humanize.Bytes(uint64(size))
}
