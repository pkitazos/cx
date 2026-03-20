package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/spf13/cobra"
)

// configuration
var clipboardPath string

func init() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	defaultClipboardPath := filepath.Join(homeDir, ".cx_clipboard.json")

	rootCmd.PersistentFlags().StringVar(&clipboardPath, "clipboard", defaultClipboardPath, "path to the clipboard file")
	rootCmd.Flags().BoolP("quiet", "q", false, "suppress all output, except errors")

	rootCmd.AddCommand(pasteCmd)
	pasteCmd.Flags().BoolP("persist", "p", false, "keep file at original path after paste")
	pasteCmd.Flags().BoolP("quiet", "q", false, "suppress all output, except errors")

	rootCmd.AddCommand(listCmd)
	listCmd.Flags().BoolP("detailed", "d", false, "show detailed file information")
	listCmd.Flags().Bool("json", false, "output clipboard as JSON")

	rootCmd.AddCommand(clearCmd)
	clearCmd.Flags().BoolP("quiet", "q", false, "suppress all output, except errors")

	rootCmd.AddCommand(completionCmd)
}

// completionCmd generates shell completion scripts
var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish]",
	Short: "Generate shell completion script",
	Long: `Generate shell completion script for cx.

To load completions:

  # zsh (add to ~/.zshrc for persistence)
  source <(cx completion zsh)

  # bash (add to ~/.bashrc for persistence)
  source <(cx completion bash)

  # fish
  cx completion fish | source`,
	Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	ValidArgs: []string{"bash", "zsh", "fish"},
	Run: func(cmd *cobra.Command, args []string) {
		switch args[0] {
		case "bash":
			rootCmd.GenBashCompletion(os.Stdout)
		case "zsh":
			rootCmd.GenZshCompletion(os.Stdout)
		case "fish":
			rootCmd.GenFishCompletion(os.Stdout, true)
		}
	},
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "cx [path]",
	Short: "A command line tool for cut and paste operations on files and directories",
	Long:  `cx allows you to cut and paste files and directories from the command line.`,
	Args:  cobra.ExactArgs(1),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return nil, cobra.ShellCompDirectiveDefault
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		quiet, _ := cmd.Flags().GetBool("quiet")
		return cutFile(cmd.OutOrStdout(), args[0], Options{quiet: quiet})
	},
}

// pasteCmd represents the paste command
var pasteCmd = &cobra.Command{
	Use:   "paste [index]",
	Short: "Paste the most recent clipboard entry",
	Args:  cobra.RangeArgs(0, 1),
	RunE: func(cmd *cobra.Command, args []string) error {
		persist, _ := cmd.Flags().GetBool("persist")
		quiet, _ := cmd.Flags().GetBool("quiet")

		index := 0
		if len(args) == 1 {
			var err error
			index, err = strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid index: %s", args[0])
			}
		}
		return handlePasteAt(cmd.OutOrStdout(), index, Options{persist: persist, quiet: quiet})

	},
}

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:     "list",
	Short:   "List clipboard contents",
	Aliases: []string{"ls"},
	RunE: func(cmd *cobra.Command, _ []string) error {
		detailed, _ := cmd.Flags().GetBool("detailed")
		json, _ := cmd.Flags().GetBool("json")
		return handleList(cmd.OutOrStdout(), Options{detailed: detailed, json: json})
	},
}

// clearCmd represents the clear command
var clearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear clipboard contents",
	RunE: func(cmd *cobra.Command, _ []string) error {
		quiet, _ := cmd.Flags().GetBool("quiet")
		return handleClear(cmd.OutOrStdout(), Options{quiet: quiet})
	},
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
