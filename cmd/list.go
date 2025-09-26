package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/timoxa0/kxmenu/entry"
)

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list [directory]",
	Short: "List available boot entries in directory",
	Long: `Scan a directory for boot entry configuration files and display
them in a simple list format. This is useful for seeing what entries
are available before using the interactive menu.`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		dir := "/boot"
		if len(args) > 0 {
			dir = args[0]
		}

		listEntries(dir)
	},
}

func listEntries(dir string) {
	entries, err := entry.FindEntries(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error scanning directory: %v\n", err)
		os.Exit(1)
	}

	if len(entries) == 0 {
		fmt.Printf("No boot entries found in %s\n", dir)
		return
	}

	fmt.Printf("Found %d boot entries in %s:\n\n", len(entries), dir)
	for i, e := range entries {
		fmt.Printf("%d. %s\n", i+1, filepath.Base(e.FilePath))
		if e.Title != "" {
			fmt.Printf("   Title: %s\n", e.Title)
		}
		if e.Version != "" {
			fmt.Printf("   Version: %s\n", e.Version)
		}
		if e.Linux != "" {
			fmt.Printf("   Kernel: %s\n", e.Linux)
		}
		fmt.Println()
	}
}
