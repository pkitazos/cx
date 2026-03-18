package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/sys/unix"
)

// Entry represents a clipboard entry containing file/directory information
type Entry struct {
	OriginalPath string    `json:"original_path"`
	CurrentPath  string    `json:"current_path"`
	CutAt        time.Time `json:"timestamp"`
}

// Clipboard represents the collection of clipboard entries
type Clipboard struct {
	Entries []Entry `json:"entries"`
}

// getClipboardPath returns the path to the clipboard file, creating it if it doesn't exist
func getClipboardPath() (string, error) {
	_, err := os.Stat(clipboardPath)
	if err != nil {
		clipboardJSON, err := json.Marshal(Clipboard{Entries: []Entry{}})
		if err != nil {
			return "", err
		}

		err = os.WriteFile(clipboardPath, clipboardJSON, 0o644)
		if err != nil {
			return "", err
		}
	}

	return clipboardPath, nil
}

// readClipboard reads and parses the clipboard file
func readClipboard() (Clipboard, error) {
	var clipboard Clipboard

	clipboardPath, err := getClipboardPath()
	if err != nil {
		return clipboard, err
	}

	clipboardFile, err := os.Open(clipboardPath)
	if err != nil {
		return clipboard, err
	}
	defer clipboardFile.Close()

	clipboardJSON, err := io.ReadAll(clipboardFile)
	if err != nil {
		return clipboard, err
	}

	err = json.Unmarshal(clipboardJSON, &clipboard)
	return clipboard, err
}

// writeClipboard writes the clipboard data to the clipboard file
func writeClipboard(clipboard Clipboard) error {
	clipboardPath, err := getClipboardPath()
	if err != nil {
		return err
	}

	clipboardJSON, err := json.MarshalIndent(clipboard, "", "  ")
	if err != nil {
		return err
	}

	err = os.WriteFile(clipboardPath, clipboardJSON, 0o644)
	return err
}

type Options struct {
	persist  bool
	quiet    bool
	detailed bool
	json     bool
}

// cutFile adds a file or directory to the clipboard
func cutFile(w io.Writer, path string, opts Options) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	fileInfo, err := os.Lstat(absPath)
	if err != nil {
		return err
	}

	if !(fileInfo.Mode()&os.ModeSymlink != 0) {
		err = unix.Access(absPath, unix.R_OK)
		if err != nil {
			return fmt.Errorf("no read permission for %s: %w", absPath, err)
		}
	}

	clipboard, err := readClipboard()
	if err != nil {
		return err
	}

	// prepend entry since clipboard is a stack
	clipboard.Entries = append([]Entry{Entry{
		OriginalPath: absPath,
		CurrentPath:  absPath,
		CutAt:        time.Now(),
	}}, clipboard.Entries...)

	err = writeClipboard(clipboard)
	if err != nil {
		return err
	}

	if opts.quiet {
		w = io.Discard
	}

	fmt.Fprintf(w, "Cut: %s\n", absPath)
	return nil
}

// handlePasteAt pastes a specific clipboard entry by index
func handlePasteAt(w io.Writer, index int, opts Options) error {
	pwd, err := os.Getwd()
	if err != nil {
		return err
	}

	clipboard, err := readClipboard()
	if err != nil {
		return err
	}

	if index < 0 || index >= len(clipboard.Entries) {
		return fmt.Errorf("invalid clipboard index: %d", index)
	}

	entry := clipboard.Entries[index]
	if _, err := os.Lstat(entry.CurrentPath); err != nil {
		return fmt.Errorf("source path no longer exists: %s", entry.CurrentPath)
	}

	result, err := pasteEntry(entry, pwd, opts)
	if err != nil {
		return err
	}

	if opts.quiet {
		w = io.Discard
	}

	if opts.persist {
		if err := updateEntryPath(index, result); err != nil {
			return err
		}
		fmt.Fprintf(w, "Copied: %s -> %s\n", entry.CurrentPath, result)
	} else {
		if err := removeFromClipboard(index); err != nil {
			return err
		}
		fmt.Fprintf(w, "Moved: %s -> %s\n", entry.CurrentPath, result)
	}

	return nil
}

// pasteEntry performs the actual paste operation (copy or move)
func pasteEntry(entry Entry, destDir string, opts Options) (string, error) {
	srcInfo, err := os.Lstat(entry.CurrentPath)
	if err != nil {
		return "", err
	}

	destPath := filepath.Join(destDir, filepath.Base(entry.CurrentPath))

	if opts.persist {
		if srcInfo.IsDir() {
			err = copyDir(entry.CurrentPath, destPath)
		} else if srcInfo.Mode()&os.ModeSymlink != 0 {
			err = copySymlink(entry.CurrentPath, destPath)
		} else {
			err = copyFile(entry.CurrentPath, destPath)
		}
	} else {
		err = os.Rename(entry.CurrentPath, destPath)
	}

	if err != nil {
		return "", err
	}

	return destPath, nil
}

// copyDir recursively copies a directory
func copyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	dirEntries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range dirEntries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// copyFile copies a single file
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

func copySymlink(src, dst string) error {
	target, err := os.Readlink(src)
	if err != nil {
		return err
	}
	return os.Symlink(target, dst)
}

// updateEntryPath updates the current path of a clipboard entry
func updateEntryPath(index int, newPath string) error {
	clipboard, err := readClipboard()
	if err != nil {
		return err
	}

	if index < 0 || index >= len(clipboard.Entries) {
		return fmt.Errorf("invalid clipboard index: %d", index)
	}

	entry := clipboard.Entries[index]
	entry.CurrentPath = newPath

	clipboard.Entries[index] = entry

	return writeClipboard(clipboard)
}

// removeFromClipboard removes an entry from the clipboard by index
func removeFromClipboard(index int) error {
	clipboard, err := readClipboard()
	if err != nil {
		return err
	}

	if index < 0 || index >= len(clipboard.Entries) {
		return fmt.Errorf("invalid clipboard index: %d", index)
	}

	clipboard.Entries = append(clipboard.Entries[:index], clipboard.Entries[index+1:]...)

	return writeClipboard(clipboard)
}

// is it better to build this at display time or at cut time?
// should this just be nesting the clipboard Entry?
type listEntry struct {
	index         int
	basePath      string
	symlinkTarget string
	size          int64
	sizeDisplay   string
	perms         string
	modTime       time.Time
	cutTime       time.Time
	isDir         bool
	isLink        bool
	isMissing     bool
}

var (
	colorMuted = lipgloss.Color("8")
	colorWhite = lipgloss.Color("15")
	colorCyan  = lipgloss.Color("14")
	colorBlue  = lipgloss.Color("4")
	colorRed   = lipgloss.Color("9")
)

var (
	fileStyle = lipgloss.NewStyle().
			Foreground(colorWhite)

	symlinkStyle = lipgloss.NewStyle().
			Foreground(colorCyan)

	dirStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorBlue)

	missingPathStyle = lipgloss.NewStyle().
				Foreground(colorRed).
				Strikethrough(true)

	detailsStyle = lipgloss.NewStyle().
			Foreground(colorMuted)
)

func indexStyle(width int) lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(colorMuted).
		Width(width).
		Align(lipgloss.Right)
}

func renderPath(entry listEntry, width int) string {
	padded := fmt.Sprintf("%-*s", width, entry.basePath)
	switch {
	case entry.isMissing:
		return missingPathStyle.Render(padded)
	case entry.isDir:
		return dirStyle.Render(padded)
	case entry.isLink:
		path := fmt.Sprintf("%s -> %s", entry.basePath, entry.symlinkTarget)
		padded := fmt.Sprintf("%-*s", width, path)
		return symlinkStyle.Render(padded)
	default:
		return fileStyle.Render(padded)
	}
}

func renderTable(w io.Writer, entries []listEntry, opts Options, maxPathWidth, maxSizeWidth, maxIndexWidth int) {

	idxStyle := indexStyle(maxIndexWidth)

	for _, entry := range entries {
		indexStr := idxStyle.Render(fmt.Sprintf("%d:", entry.index))
		pathStr := renderPath(entry, maxPathWidth)

		if entry.isMissing {
			fmt.Fprintf(w, "%s %s %s\n", indexStr, pathStr, detailsStyle.Render("(file not found)"))
			continue
		}

		if opts.detailed {

			fmt.Fprintf(w, "%s %s %s %s %s %s\n", indexStr, pathStr,
				detailsStyle.Render(fmt.Sprintf("%*s", maxSizeWidth, entry.sizeDisplay)),
				detailsStyle.Render(entry.perms),
				detailsStyle.Render(entry.modTime.Format("2006-01-02 15:04:05")),
				detailsStyle.Render(FormatCutAtTime(entry.cutTime)),
			)
			continue
		}

		fmt.Fprintf(w, "%s %s\n", indexStr, pathStr)

	}
}

type jsonEntry struct {
	Path         string    `json:"path"`
	Symlink      string    `json:"symlink,omitempty"`
	Size         int64     `json:"size,omitempty"`
	Permissions  string    `json:"permissions,omitempty"`
	LastModified time.Time `json:"last_modified,omitzero"`
	CutAt        time.Time `json:"cut_at,omitzero"`
	Error        string    `json:"error,omitempty"`
}

func renderJSON(w io.Writer, entries []listEntry, opts Options) error {
	jsonEntries := make([]jsonEntry, 0, len(entries))

	for _, entry := range entries {
		e := jsonEntry{Path: entry.basePath}

		if entry.isMissing {
			e.Error = "file not found"
			jsonEntries = append(jsonEntries, e)
			continue
		}

		if entry.isLink {
			e.Symlink = entry.symlinkTarget
		}

		if opts.detailed {
			e.Size = entry.size
			e.Permissions = entry.perms
			e.LastModified = entry.modTime
			e.CutAt = entry.cutTime
		}
		jsonEntries = append(jsonEntries, e)

	}

	b, err := json.MarshalIndent(jsonEntries, "", " ")
	if err == nil {
		fmt.Fprintf(w, "%s\n", string(b))
	}

	return err

}

// handleList displays all clipboard entries with proper column alignment
func handleList(w io.Writer, opts Options) error {
	// todo: use relative paths
	clipboard, err := readClipboard()
	if err != nil {
		return err
	}

	numEntries := len(clipboard.Entries)
	if numEntries == 0 {
		fmt.Fprintln(w, "Clipboard is empty")
		return nil
	}

	maxIndexWidth := len(strconv.Itoa(numEntries)) + 1

	entries := make([]listEntry, 0, numEntries)

	maxPathWidth := 0
	maxSizeWidth := 0

	for i, entry := range clipboard.Entries {
		var e listEntry
		e.index = i
		e.basePath = entry.OriginalPath
		e.cutTime = entry.CutAt

		fileInfo, err := os.Lstat(entry.OriginalPath)
		if err != nil {
			e.isMissing = true
			entries = append(entries, e)
			if len(e.basePath) > maxPathWidth {
				maxPathWidth = len(e.basePath)
			}
			continue
		}

		e.size = fileInfo.Size()
		e.sizeDisplay = FormatSize(e.size)
		e.perms = fileInfo.Mode().String()
		e.modTime = fileInfo.ModTime()
		e.isDir = fileInfo.IsDir()
		e.isLink = fileInfo.Mode()&os.ModeSymlink != 0
		displayPathWidth := len(e.basePath)

		if e.isLink {
			if target, err := os.Readlink(entry.OriginalPath); err == nil {
				displayPathWidth = len(fmt.Sprintf("%s -> %s", e.basePath, target))
				e.symlinkTarget = target
			} else {
				displayPathWidth = len(fmt.Sprintf("%s -> (broken)", e.basePath))
				e.symlinkTarget = "(broken)"
			}

		}

		entries = append(entries, e)

		if displayPathWidth > maxPathWidth {
			maxPathWidth = displayPathWidth
		}

		if len(e.sizeDisplay) > maxSizeWidth {
			maxSizeWidth = len(e.sizeDisplay)
		}
	}

	if opts.json {
		return renderJSON(w, entries, opts)
	}

	renderTable(w, entries, opts, maxPathWidth, maxSizeWidth, maxIndexWidth)
	return nil
}

// handleClear clears all clipboard entries
func handleClear(w io.Writer, opts Options) error {
	clipboard, err := readClipboard()
	if err != nil {
		return err
	}

	clipboard.Entries = []Entry{}

	err = writeClipboard(clipboard)
	if err != nil {
		return err
	}

	if opts.quiet {
		w = io.Discard
	}

	fmt.Fprintln(w, "Clipboard cleared")
	return nil
}
