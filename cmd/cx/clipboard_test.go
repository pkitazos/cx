package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupTestEnvironment creates a temporary test directory with test files and sets up clipboard path
func setupTestEnvironment(t *testing.T) (tempDir string, cleanup func()) {
	t.Helper()

	// Create temporary test directory
	tempDir, err := os.MkdirTemp("", "cx_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Set clipboardPath to use temp directory
	originalClipboardPath := clipboardPath
	clipboardPath = filepath.Join(tempDir, ".cx_clipboard.json")

	// Create test files and directories
	testFiles := map[string]string{
		"file1.txt":            "This is file 1",
		"file2.txt":            "This is file 2",
		"nested/file3.txt":     "This is a nested file",
		"config/settings.json": `{"setting": "value"}`,
		"config/config.ini":    "key=value",
		"empty_dir/.gitkeep":   "",
	}

	for relativePath, content := range testFiles {
		fullPath := filepath.Join(tempDir, relativePath)

		// Create directory if it doesn't exist
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}

		// Write file
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write file %s: %v", fullPath, err)
		}
	}

	cleanup = func() {
		clipboardPath = originalClipboardPath
		os.RemoveAll(tempDir)
	}

	return tempDir, cleanup
}

func TestCutFile(t *testing.T) {
	tempDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	testFile := filepath.Join(tempDir, "file1.txt")

	// Test cutting a valid file
	err := cutFile(testFile)
	if err != nil {
		t.Fatalf("cutFile failed: %v", err)
	}

	// Verify clipboard contains the file
	clipboard, err := readClipboard()
	if err != nil {
		t.Fatalf("Failed to read clipboard: %v", err)
	}

	if len(clipboard.Entries) != 1 {
		t.Fatalf("Expected 1 clipboard entry, got %d", len(clipboard.Entries))
	}

	entry := clipboard.Entries[0]
	if entry.OriginalPath != testFile {
		t.Errorf("Expected OriginalPath %s, got %s", testFile, entry.OriginalPath)
	}
	if entry.CurrentPath != testFile {
		t.Errorf("Expected CurrentPath %s, got %s", testFile, entry.CurrentPath)
	}
}

func TestCutNonexistentFile(t *testing.T) {
	tempDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	nonexistentFile := filepath.Join(tempDir, "nonexistent.txt")

	err := cutFile(nonexistentFile)
	if err == nil {
		t.Fatal("Expected error when cutting nonexistent file, got nil")
	}
}

func TestCutDirectory(t *testing.T) {
	tempDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	testDir := filepath.Join(tempDir, "config")

	err := cutFile(testDir)
	if err != nil {
		t.Fatalf("cutFile failed for directory: %v", err)
	}

	// Verify clipboard contains the directory
	clipboard, err := readClipboard()
	if err != nil {
		t.Fatalf("Failed to read clipboard: %v", err)
	}

	if len(clipboard.Entries) != 1 {
		t.Fatalf("Expected 1 clipboard entry, got %d", len(clipboard.Entries))
	}

	entry := clipboard.Entries[0]
	if entry.OriginalPath != testDir {
		t.Errorf("Expected OriginalPath %s, got %s", testDir, entry.OriginalPath)
	}
}

func TestMultipleCuts(t *testing.T) {
	tempDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	files := []string{
		filepath.Join(tempDir, "file1.txt"),
		filepath.Join(tempDir, "file2.txt"),
	}

	// Cut multiple files
	for _, file := range files {
		err := cutFile(file)
		if err != nil {
			t.Fatalf("cutFile failed for %s: %v", file, err)
		}
	}

	// Verify clipboard contains both files
	clipboard, err := readClipboard()
	if err != nil {
		t.Fatalf("Failed to read clipboard: %v", err)
	}

	if len(clipboard.Entries) != 2 {
		t.Fatalf("Expected 2 clipboard entries, got %d", len(clipboard.Entries))
	}

	// Verify files are in clipboard in correct order (most recent last)
	if clipboard.Entries[0].OriginalPath != files[0] {
		t.Errorf("Expected first entry to be %s, got %s", files[0], clipboard.Entries[0].OriginalPath)
	}
	if clipboard.Entries[1].OriginalPath != files[1] {
		t.Errorf("Expected second entry to be %s, got %s", files[1], clipboard.Entries[1].OriginalPath)
	}
}

func TestPasteMove(t *testing.T) {
	tempDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	sourceFile := filepath.Join(tempDir, "file1.txt")
	destDir := filepath.Join(tempDir, "destination")

	// Create destination directory
	err := os.MkdirAll(destDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create destination directory: %v", err)
	}

	// Cut the file
	err = cutFile(sourceFile)
	if err != nil {
		t.Fatalf("cutFile failed: %v", err)
	}

	// Change to destination directory
	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)

	err = os.Chdir(destDir)
	if err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	// Paste (move) the file
	err = handlePaste(false) // persist = false means move
	if err != nil {
		t.Fatalf("handlePaste failed: %v", err)
	}

	// Verify file was moved
	expectedDest := filepath.Join(destDir, "file1.txt")
	if _, err := os.Stat(expectedDest); os.IsNotExist(err) {
		t.Errorf("File was not moved to destination: %s", expectedDest)
	}

	if _, err := os.Stat(sourceFile); !os.IsNotExist(err) {
		t.Errorf("Source file still exists after move: %s", sourceFile)
	}

	// Verify clipboard is empty after non-persistent paste
	clipboard, err := readClipboard()
	if err != nil {
		t.Fatalf("Failed to read clipboard: %v", err)
	}

	if len(clipboard.Entries) != 0 {
		t.Errorf("Expected empty clipboard after move, got %d entries", len(clipboard.Entries))
	}
}

func TestPasteCopy(t *testing.T) {
	tempDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	sourceFile := filepath.Join(tempDir, "file1.txt")
	destDir := filepath.Join(tempDir, "destination")

	// Create destination directory
	err := os.MkdirAll(destDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create destination directory: %v", err)
	}

	// Read original content
	originalContent, err := os.ReadFile(sourceFile)
	if err != nil {
		t.Fatalf("Failed to read source file: %v", err)
	}

	// Cut the file
	err = cutFile(sourceFile)
	if err != nil {
		t.Fatalf("cutFile failed: %v", err)
	}

	// Change to destination directory
	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)

	err = os.Chdir(destDir)
	if err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	// Paste (copy) the file
	err = handlePaste(true) // persist = true means copy
	if err != nil {
		t.Fatalf("handlePaste failed: %v", err)
	}

	// Verify file was copied
	expectedDest := filepath.Join(destDir, "file1.txt")
	if _, err := os.Stat(expectedDest); os.IsNotExist(err) {
		t.Errorf("File was not copied to destination: %s", expectedDest)
	}

	// Verify original file still exists
	if _, err := os.Stat(sourceFile); os.IsNotExist(err) {
		t.Errorf("Source file was removed after copy: %s", sourceFile)
	}

	// Verify content is identical
	copiedContent, err := os.ReadFile(expectedDest)
	if err != nil {
		t.Fatalf("Failed to read copied file: %v", err)
	}

	if string(originalContent) != string(copiedContent) {
		t.Errorf("File content mismatch after copy")
	}

	// Verify clipboard still has entry after persistent paste
	clipboard, err := readClipboard()
	if err != nil {
		t.Fatalf("Failed to read clipboard: %v", err)
	}

	if len(clipboard.Entries) != 1 {
		t.Errorf("Expected 1 clipboard entry after copy, got %d", len(clipboard.Entries))
	}
}

func TestPasteDirectory(t *testing.T) {
	tempDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	sourceDir := filepath.Join(tempDir, "config")
	destDir := filepath.Join(tempDir, "destination")

	// Create destination directory
	err := os.MkdirAll(destDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create destination directory: %v", err)
	}

	// Cut the directory
	err = cutFile(sourceDir)
	if err != nil {
		t.Fatalf("cutFile failed: %v", err)
	}

	// Change to destination directory
	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)

	err = os.Chdir(destDir)
	if err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	// Paste (copy) the directory
	err = handlePaste(true) // persist = true means copy
	if err != nil {
		t.Fatalf("handlePaste failed: %v", err)
	}

	// Verify directory and its contents were copied
	expectedDest := filepath.Join(destDir, "config")
	if _, err := os.Stat(expectedDest); os.IsNotExist(err) {
		t.Errorf("Directory was not copied to destination: %s", expectedDest)
	}

	// Check that files inside were copied
	expectedFile := filepath.Join(expectedDest, "settings.json")
	if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
		t.Errorf("File inside directory was not copied: %s", expectedFile)
	}
}

func TestPasteEmptyClipboard(t *testing.T) {
	_, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Try to paste from empty clipboard
	err := handlePaste(false)
	if err == nil {
		t.Fatal("Expected error when pasting from empty clipboard, got nil")
	}

	if !strings.Contains(err.Error(), "clipboard is empty") {
		t.Errorf("Expected 'clipboard is empty' error, got: %v", err)
	}
}

func TestPasteNonexistentFile(t *testing.T) {
	tempDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	sourceFile := filepath.Join(tempDir, "file1.txt")

	// Cut the file
	err := cutFile(sourceFile)
	if err != nil {
		t.Fatalf("cutFile failed: %v", err)
	}

	// Remove the source file to simulate it being deleted
	err = os.Remove(sourceFile)
	if err != nil {
		t.Fatalf("Failed to remove source file: %v", err)
	}

	// Try to paste - should fail
	err = handlePaste(false)
	if err == nil {
		t.Fatal("Expected error when pasting nonexistent file, got nil")
	}

	if !strings.Contains(err.Error(), "no longer exists") {
		t.Errorf("Expected 'no longer exists' error, got: %v", err)
	}
}

func TestHandleList(t *testing.T) {
	tempDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Test empty clipboard
	err := handleList()
	if err != nil {
		t.Fatalf("handleList failed on empty clipboard: %v", err)
	}

	// Add some files to clipboard
	files := []string{
		filepath.Join(tempDir, "file1.txt"),
		filepath.Join(tempDir, "file2.txt"),
	}

	for _, file := range files {
		err := cutFile(file)
		if err != nil {
			t.Fatalf("cutFile failed: %v", err)
		}
	}

	// Test list with entries
	err = handleList()
	if err != nil {
		t.Fatalf("handleList failed: %v", err)
	}
}

func TestHandleClear(t *testing.T) {
	tempDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Add a file to clipboard
	sourceFile := filepath.Join(tempDir, "file1.txt")
	err := cutFile(sourceFile)
	if err != nil {
		t.Fatalf("cutFile failed: %v", err)
	}

	// Verify clipboard has entry
	clipboard, err := readClipboard()
	if err != nil {
		t.Fatalf("Failed to read clipboard: %v", err)
	}
	if len(clipboard.Entries) != 1 {
		t.Fatalf("Expected 1 clipboard entry, got %d", len(clipboard.Entries))
	}

	// Clear clipboard
	err = handleClear()
	if err != nil {
		t.Fatalf("handleClear failed: %v", err)
	}

	// Verify clipboard is empty
	clipboard, err = readClipboard()
	if err != nil {
		t.Fatalf("Failed to read clipboard: %v", err)
	}
	if len(clipboard.Entries) != 0 {
		t.Errorf("Expected empty clipboard after clear, got %d entries", len(clipboard.Entries))
	}
}

func TestClipboardPersistence(t *testing.T) {
	tempDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	sourceFile := filepath.Join(tempDir, "file1.txt")

	// Cut a file
	err := cutFile(sourceFile)
	if err != nil {
		t.Fatalf("cutFile failed: %v", err)
	}

	// Read clipboard directly from file
	clipboardData, err := os.ReadFile(clipboardPath)
	if err != nil {
		t.Fatalf("Failed to read clipboard file: %v", err)
	}

	var clipboard Clipboard
	err = json.Unmarshal(clipboardData, &clipboard)
	if err != nil {
		t.Fatalf("Failed to unmarshal clipboard JSON: %v", err)
	}

	if len(clipboard.Entries) != 1 {
		t.Errorf("Expected 1 entry in persisted clipboard, got %d", len(clipboard.Entries))
	}

	if clipboard.Entries[0].OriginalPath != sourceFile {
		t.Errorf("Expected persisted entry path %s, got %s", sourceFile, clipboard.Entries[0].OriginalPath)
	}
}
