package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var (
	// configuration
	clipboardPath string
	persistFlag   bool
)

func init() {
	// set default clipboard path
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

// rootCmd represents the base command when called without any subcommands
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

// pasteCmd represents the paste command
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

// listCmd represents the list command
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

// clearCmd represents the clear command
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

// main is the entry point of the application
func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
