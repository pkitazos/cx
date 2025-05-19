package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"
)

type Entry struct {
	OriginalPath string    `json:"original_path"`
	CurrentPath  string    `json:"current_path"`
	Timestamp    time.Time `json:"timestamp"`
}

type Clipboard struct {
	Entries []Entry `json:"entries"`
}

var (
	// Configuration
	clipboardPath string
	persistFlag   bool
)

func init() {
	// Set default clipboard path
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	defaultClipboardPath := filepath.Join(homeDir, ".cx_clipboard.json")

	// Root command
	rootCmd.PersistentFlags().StringVar(&clipboardPath, "clipboard", defaultClipboardPath, "path to the clipboard file")

	// Cut command - no additional flags needed as it works with arguments

	// Paste command
	pasteCmd.Flags().BoolVarP(&persistFlag, "persist", "p", false, "keep entry in clipboard after paste")
	rootCmd.AddCommand(pasteCmd)

	// List command
	rootCmd.AddCommand(listCmd)

	// Clear command
	rootCmd.AddCommand(clearCmd)
}

var rootCmd = &cobra.Command{
	Use:   "cx [path]",
	Short: "A command line tool for cut and paste operations on files and directories",
	Long:  `cx allows you to cut and paste files and directories from the command line.`,
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// If no arguments provided and no subcommand, show help
		if len(args) == 0 {
			cmd.Help()
			return
		}

		// Otherwise, treat as cut operation
		err := cutFile(args[0])
		if err != nil {
			log.Fatal(err)
		}
	},
}

var pasteCmd = &cobra.Command{
	Use:   "paste",
	Short: "Paste the most recent clipboard entry",
	Run: func(cmd *cobra.Command, args []string) {
		err := handlePaste(persistFlag)
		if err != nil {
			log.Fatal(err)
		}
	},
}

var listCmd = &cobra.Command{
	Use:     "list",
	Short:   "List clipboard contents",
	Aliases: []string{"ls", "l"},
	Run: func(cmd *cobra.Command, args []string) {
		err := handleList()
		if err != nil {
			log.Fatal(err)
		}
	},
}

var clearCmd = &cobra.Command{
	Use:     "clear",
	Short:   "Clear clipboard contents",
	Aliases: []string{"c"},
	Run: func(cmd *cobra.Command, args []string) {
		err := handleClear()
		if err != nil {
			log.Fatal(err)
		}
	},
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

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
