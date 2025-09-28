package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/timoxa0/kxmenu/entry"
	"github.com/timoxa0/kxmenu/input"
	"github.com/timoxa0/kxmenu/kexec"
	"github.com/timoxa0/kxmenu/menu"
)

// menuCmd represents the menu command
var menuCmd = &cobra.Command{
	Use:   "menu [directory]",
	Short: "Show interactive GRUB2-style boot menu with hardware key support",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		dir := "/boot"
		if len(args) > 0 {
			dir = args[0]
		}

		bootRoot, _ := cmd.Flags().GetString("boot-root")
		timeout, _ := cmd.Flags().GetInt("timeout")
		noHardware, _ := cmd.Flags().GetBool("no-hardware")

		showEnhancedBootMenu(dir, bootRoot, timeout, !noHardware)
	},
}

func init() {
	menuCmd.Flags().IntP("timeout", "t", 0, "Menu timeout in seconds (0 = no timeout)")
	menuCmd.Flags().BoolP("no-hardware", "n", false, "Disable hardware key detection")
}

func showEnhancedBootMenu(dir, bootRoot string, timeout int, enableHardware bool) {
	// Find boot entries
	entries, err := entry.FindEntries(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error scanning directory: %v\n", err)
		os.Exit(1)
	}

	if len(entries) == 0 {
		fmt.Printf("No boot entries found in %s\n", dir)
		os.Exit(1)
	}

	// Initialize input manager
	var inputMgr *input.InputManager
	if enableHardware {
		inputMgr = input.NewInputManager()

		// Discover hardware input devices
		err := inputMgr.DiscoverDevices()
		if err != nil {
			fmt.Printf("Warning: Failed to discover input devices: %v\n", err)
			fmt.Println("Falling back to keyboard-only mode")
		} else {
			fmt.Printf("Hardware input support enabled\n")
		}

		// Start listening for input events
		inputMgr.StartListening()
		defer inputMgr.Stop()
	}

	// Create enhanced boot menu
	bootMenu := menu.NewBootMenuWithInput(entries, "kxboot - kexec-based bootloader", inputMgr)

	if timeout > 0 {
		bootMenu.SetTimeout(timeout)
	}

	fmt.Println("")

	selectedEntry, err := bootMenu.Show()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Menu error: %v\n", err)
		os.Exit(1)
	}

	if selectedEntry == nil {
		fmt.Println("No entry selected")
		os.Exit(1)
	}

	fmt.Printf("\nLoading entry: %s\n", getEntryDisplayName(selectedEntry))

	// Load the selected entry using kexec
	err = kexec.LoadEntryFromParsed(selectedEntry, bootRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading entry: %v\n", err)
		os.Exit(1)
	}
}

func getEntryDisplayName(e *entry.BootEntry) string {
	if e.Title != "" {
		return e.Title
	}
	return "Entry"
}
