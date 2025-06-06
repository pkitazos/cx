package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

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
	clipboardPath, err := getClipboardPath()
	if err != nil {
		return Clipboard{}, err
	}

	clipboardFile, err := os.Open(clipboardPath)
	if err != nil {
		return Clipboard{}, err
	}
	defer clipboardFile.Close()

	clipboardJson, err := io.ReadAll(clipboardFile)
	if err != nil {
		return Clipboard{}, err
	}

	var clipboard Clipboard
	err = json.Unmarshal(clipboardJson, &clipboard)
	if err != nil {
		return Clipboard{}, err
	}

	return clipboard, nil
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
	if err != nil {
		return err
	}

	return nil
}

// cutFile adds a file or directory to the clipboard
func cutFile(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	_, err = os.Stat(absPath)
	if err != nil {
		return err
	}

	err = unix.Access(absPath, unix.R_OK)
	if err != nil {
		return fmt.Errorf("no read permission for %s: %w", absPath, err)
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
	if _, err := os.Stat(entry.CurrentPath); err != nil {
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
	if _, err := os.Stat(entry.CurrentPath); err != nil {
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
	srcInfo, err := os.Stat(entry.CurrentPath)
	if err != nil {
		return "", err
	}

	destPath := filepath.Join(destDir, filepath.Base(entry.CurrentPath))

	if persist {
		if srcInfo.IsDir() {
			if err := copyDir(entry.CurrentPath, destPath); err != nil {
				return "", err
			}
		} else {
			if err := copyFile(entry.CurrentPath, destPath); err != nil {
				return "", err
			}
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

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}
	return nil
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

// handleList displays all clipboard entries
func handleList() error {
	clipboard, err := readClipboard()
	if err != nil {
		return err
	}

	if len(clipboard.Entries) == 0 {
		fmt.Println("Clipboard is empty")
		return nil
	}

	fmt.Println("Clipboard contents:")
	for i, entry := range clipboard.Entries {
		fmt.Printf("%d: %s\n", i, entry.OriginalPath)
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
