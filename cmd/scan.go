package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/timoxa0/kxmenu/entry"
	"github.com/timoxa0/kxmenu/kexec"
)

// scanCmd represents the scan command (legacy simple mode)
var scanCmd = &cobra.Command{
	Use:   "scan [directory]",
	Short: "Scan directory and select entry interactively (simple text mode)",
	Long: `Scan a directory for boot entries and present a simple numbered
selection menu. This is the legacy mode that works in all terminal
environments without requiring advanced input handling.`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		dir := "/boot"
		if len(args) > 0 {
			dir = args[0]
		}

		bootRoot, _ := cmd.Flags().GetString("boot-root")
		scanAndSelect(dir, bootRoot)
	},
}

func scanAndSelect(dir, bootRoot string) {
	entries, err := entry.FindEntries(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error scanning directory: %v\n", err)
		os.Exit(1)
	}

	if len(entries) == 0 {
		fmt.Printf("No boot entries found in %s\n", dir)
		os.Exit(1)
	}

	// Display available entries
	fmt.Printf("Available boot entries:\n\n")
	for i, e := range entries {
		title := e.Title
		if title == "" {
			title = filepath.Base(e.FilePath)
		}
		fmt.Printf("%d. %s", i+1, title)
		if e.Version != "" {
			fmt.Printf(" (%s)", e.Version)
		}
		fmt.Println()
	}

	// Prompt for selection
	fmt.Printf("\nSelect entry (1-%d): ", len(entries))
	var input string
	fmt.Scanln(&input)

	selection, err := strconv.Atoi(input)
	if err != nil || selection < 1 || selection > len(entries) {
		fmt.Fprintf(os.Stderr, "Invalid selection: %s\n", input)
		os.Exit(1)
	}

	selectedEntry := entries[selection-1]
	fmt.Printf("Loading entry: %s\n", filepath.Base(selectedEntry.FilePath))

	// Load the selected entry using kexec
	err = kexec.LoadEntryFromParsed(selectedEntry, bootRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading entry: %v\n", err)
		os.Exit(1)
	}
}

func loadSingleEntry(entryFile, bootRoot string) {
	err := kexec.LoadEntry(entryFile, bootRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
