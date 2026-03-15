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
	Timestamp    time.Time `json:"timestamp"`
}

// Clipboard represents the collection of clipboard entries
type Clipboard struct {
	Entries []Entry `json:"entries"`
}

// getClipboardPath returns the path to the clipboard file, creating it if it doesn't exist
func getClipboardPath() (string, error) {
	_, err := os.Stat(clipboardPath)
	if err != nil {
		clipboardJson, err := json.Marshal(Clipboard{Entries: []Entry{}})
		if err != nil {
			return "", err
		}

		err = os.WriteFile(clipboardPath, clipboardJson, 0644)
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

	clipboardJson, err := io.ReadAll(clipboardFile)
	if err != nil {
		return clipboard, err
	}

	err = json.Unmarshal(clipboardJson, &clipboard)
	return clipboard, err
}

// writeClipboard writes the clipboard data to the clipboard file
func writeClipboard(clipboard Clipboard) error {
	clipboardPath, err := getClipboardPath()
	if err != nil {
		return err
	}

	clipboardJson, err := json.MarshalIndent(clipboard, "", "  ")
	if err != nil {
		return err
	}

	err = os.WriteFile(clipboardPath, clipboardJson, 0644)
	return err
}

// cutFile adds a file or directory to the clipboard
func cutFile(path string) error {
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

	clipboard.Entries = append(clipboard.Entries, Entry{
		OriginalPath: absPath,
		CurrentPath:  absPath,
		Timestamp:    time.Now(),
	})

	err = writeClipboard(clipboard)
	if err != nil {
		return err
	}

	fmt.Printf("Cut: %s\n", absPath)
	return nil
}

// handlePaste pastes the most recent clipboard entry
func handlePaste(persist bool) error {
	clipboard, err := readClipboard()
	if err != nil {
		return err
	}

	numEntries := len(clipboard.Entries)
	if numEntries == 0 {
		return fmt.Errorf("clipboard is empty")
	}

	entry := clipboard.Entries[numEntries-1]
	if _, err := os.Lstat(entry.CurrentPath); err != nil {
		return fmt.Errorf("source path no longer exists: %s", entry.CurrentPath)
	}

	return handlePasteAt(numEntries-1, persist)
}

// handlePasteAt pastes a specific clipboard entry by index
func handlePasteAt(index int, persist bool) error {
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

	result, err := pasteEntry(entry, pwd, persist)
	if err != nil {
		return err
	}

	if persist {
		if err := updateEntryPath(index, result); err != nil {
			return err
		}
		fmt.Printf("Copied: %s -> %s\n", entry.CurrentPath, result)
	} else {
		if err := removeFromClipboard(index); err != nil {
			return err
		}
		fmt.Printf("Moved: %s -> %s\n", entry.CurrentPath, result)
	}

	return nil
}

// pasteEntry performs the actual paste operation (copy or move)
func pasteEntry(entry Entry, destDir string, persist bool) (string, error) {
	srcInfo, err := os.Lstat(entry.CurrentPath)
	if err != nil {
		return "", err
	}

	destPath := filepath.Join(destDir, filepath.Base(entry.CurrentPath))

	if persist {
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

type displayEntry struct {
	index     int
	path      string // display path, including " -> target" for symlinks
	size      string
	perms     string
	modTime   time.Time
	isDir     bool
	isLink    bool
	isMissing bool
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

func renderPath(entry displayEntry, width int) string {
	padded := fmt.Sprintf("%-*s", width, entry.path)
	switch {
	case entry.isMissing:
		return missingPathStyle.Render(padded)
	case entry.isDir:
		return dirStyle.Render(padded)
	case entry.isLink:
		return symlinkStyle.Render(padded)
	default:
		return fileStyle.Render(padded)
	}
}

// handleList displays all clipboard entries with proper column alignment
func handleList(detailed bool) error {
	clipboard, err := readClipboard()
	if err != nil {
		return err
	}

	numEntries := len(clipboard.Entries)
	if numEntries == 0 {
		fmt.Println("Clipboard is empty")
		return nil
	}

	maxIndexWidth := len(strconv.Itoa(numEntries)) + 1

	entries := make([]displayEntry, 0, numEntries)
	maxPathWidth := 0
	maxSizeWidth := 0

	for i, entry := range clipboard.Entries {
		var de displayEntry
		de.index = i
		de.path = entry.OriginalPath

		fileInfo, err := os.Lstat(entry.OriginalPath)
		if err != nil {
			de.isMissing = true
			entries = append(entries, de)
			if len(de.path) > maxPathWidth {
				maxPathWidth = len(de.path)
			}
			continue
		}

		de.size = FormatSize(fileInfo)
		de.perms = fileInfo.Mode().String()
		de.modTime = fileInfo.ModTime()
		de.isDir = fileInfo.IsDir()
		de.isLink = fileInfo.Mode()&os.ModeSymlink != 0

		if de.isLink {
			if target, err := os.Readlink(entry.OriginalPath); err == nil {
				de.path = fmt.Sprintf("%s -> %s", entry.OriginalPath, target)
			} else {
				de.path = fmt.Sprintf("%s -> (broken)", entry.OriginalPath)
			}
		}

		entries = append(entries, de)

		if len(de.path) > maxPathWidth {
			maxPathWidth = len(de.path)
		}

		if len(de.size) > maxSizeWidth {
			maxSizeWidth = len(de.size)
		}
	}

	idxStyle := indexStyle(maxIndexWidth)

	for _, entry := range entries {
		indexStr := idxStyle.Render(fmt.Sprintf("%d:", entry.index))
		pathStr := renderPath(entry, maxPathWidth)

		if entry.isMissing {
			// todo: decide whether this level of info is fine
			fmt.Printf("%s %s %s\n", indexStr, pathStr, detailsStyle.Render("(file not found)"))
			continue
		}

		if detailed {
			fmt.Printf("%s %s %s %s %s\n", indexStr, pathStr,
				detailsStyle.Render(fmt.Sprintf("%*s", maxSizeWidth, entry.size)),
				detailsStyle.Render(entry.perms),
				detailsStyle.Render(entry.modTime.Format("2006-01-02 15:04:05")),
			)
			continue
		}

		fmt.Printf("%s %s %s %s\n", indexStr, pathStr,
			detailsStyle.Render(fmt.Sprintf("%*s", maxSizeWidth, entry.size)),
			detailsStyle.Render(FormatLastModTime(entry.modTime)),
		)

	}

	return nil
}

// handleClear clears all clipboard entries
func handleClear() error {
	clipboard, err := readClipboard()
	if err != nil {
		return err
	}

	clipboard.Entries = []Entry{}

	err = writeClipboard(clipboard)
	if err != nil {
		return err
	}

	fmt.Println("Clipboard cleared")
	return nil
}
