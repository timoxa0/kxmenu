package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var (
	// Version information (set by build flags)
	Version   = "dev"
	BuildTime = "unknown"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "kxmenu",
	Short: "Kernel execution menu utility",
	Long: `kxmenu is a powerful kernel execution utility that provides interactive
boot menus and kexec functionality. It supports both traditional keyboard
navigation and hardware button controls (volume up/down, power button).

The tool can scan directories for boot entries, display interactive menus
similar to GRUB2, and execute kernel switching via kexec.`,
	Version: Version,
	Run: func(cmd *cobra.Command, args []string) {
		// Default behavior: show help if no arguments
		if len(args) == 0 {
			cmd.Help()
			return
		}

		// Legacy mode: load specific entry file
		entryFile := args[0]
		bootRoot := "/mnt"
		if len(args) > 1 {
			bootRoot = args[1]
		}

		loadSingleEntry(entryFile, bootRoot)
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Add version template
	rootCmd.SetVersionTemplate(`{{printf "kxmenu version %s (built %s)\n" .Version "` + BuildTime + `"}}`)

	// Global flags can be added here
	rootCmd.PersistentFlags().StringP("boot-root", "r", "/mnt", "Root directory for boot files")

	// Add commands
	rootCmd.AddCommand(menuCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(scanCmd)
}
