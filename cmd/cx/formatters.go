package main

import (
	"fmt"
	"os"

	"github.com/dustin/go-humanize"
)

// FormatFileInfo returns a formatted string containing the human-readable size and
// last modified time of the provided os.FileInfo.
func FormatFileInfo(fileInfo os.FileInfo) string {
	fileSize := humanize.Bytes(uint64(fileInfo.Size()))
	fileLastModified := humanize.Time(fileInfo.ModTime())

	return fmt.Sprintf("(%s, %s)", fileSize, fileLastModified)
}
