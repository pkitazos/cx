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
)

func init() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	defaultClipboardPath := filepath.Join(homeDir, ".cx_clipboard.json")

	rootCmd.PersistentFlags().StringVar(&clipboardPath, "clipboard", defaultClipboardPath, "path to the clipboard file")

	rootCmd.AddCommand(pasteCmd)

	rootCmd.AddCommand(listCmd)
	listCmd.Flags().BoolP("detailed", "d", false, "show detailed file information")

	rootCmd.AddCommand(clearCmd)
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "cx [path]",
	Short: "A command line tool for cut and paste operations on files and directories",
	Long:  `cx allows you to cut and paste files and directories from the command line.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return cutFile(args[0])
	},
}

// pasteCmd represents the paste command
var pasteCmd = &cobra.Command{
	Use:   "paste",
	Short: "Paste the most recent clipboard entry",
	RunE: func(cmd *cobra.Command, args []string) error {
		return handlePaste(false)
	},
}

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:     "list",
	Short:   "List clipboard contents",
	Aliases: []string{"ls"},
	RunE: func(cmd *cobra.Command, args []string) error {
		detailed, _ := cmd.Flags().GetBool("detailed")
		return handleList(detailed)
	},
}

// clearCmd represents the clear command
var clearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear clipboard contents",
	RunE: func(cmd *cobra.Command, args []string) error {
		return handleClear()
	},
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
