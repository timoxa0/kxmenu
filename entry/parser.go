package entry

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// BootEntry represents a boot entry configuration
type BootEntry struct {
	Title      string
	Version    string
	Linux      string
	Initrd     string
	Devicetree string
	Options    string
	FilePath   string // Path to the entry file for reference
}

// ParseEntry parses a single boot entry configuration file
func ParseEntry(entryFile string) (*BootEntry, error) {
	file, err := os.Open(entryFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	entry := &BootEntry{
		FilePath: entryFile,
	}
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Split into key and value
		parts := strings.SplitN(line, " ", 2)
		if len(parts) < 2 {
			continue
		}

		key := parts[0]
		value := parts[1]

		// Assign values based on key
		switch key {
		case "title":
			entry.Title = value
		case "version":
			entry.Version = value
		case "linux":
			entry.Linux = value
		case "initrd":
			entry.Initrd = value
		case "devicetree":
			entry.Devicetree = value
		case "options":
			entry.Options = value
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return entry, nil
}

// FindEntries searches for boot entry configuration files in a directory
// It looks for files with .conf extension or specific entry file patterns
func FindEntries(dir string) ([]*BootEntry, error) {
	var entries []*BootEntry

	// Check if directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, fmt.Errorf("directory %s does not exist", dir)
	}

	// Walk through directory
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Check for entry files (.conf extension or specific patterns)
		if isEntryFile(info.Name()) {
			entry, parseErr := ParseEntry(path)
			if parseErr != nil {
				// Log warning but continue processing other files
				fmt.Fprintf(os.Stderr, "Warning: failed to parse %s: %v\n", path, parseErr)
				return nil
			}
			entries = append(entries, entry)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("error scanning directory %s: %v", dir, err)
	}

	return entries, nil
}

// isEntryFile checks if a filename matches boot entry file patterns
func isEntryFile(filename string) bool {
	// Check for .conf extension
	if strings.HasSuffix(filename, ".conf") {
		return true
	}

	// Check for common entry file names
	commonNames := []string{
		"entry",
		"boot.entry",
		"kernel.entry",
	}

	for _, name := range commonNames {
		if filename == name {
			return true
		}
	}

	return false
}

// CleanupEntry removes tuned parameters and performs other cleanup
func (e *BootEntry) CleanupEntry() {
	e.Initrd = strings.ReplaceAll(e.Initrd, " $tuned_initrd", "")
	e.Options = strings.ReplaceAll(e.Options, " $tuned_params", "")
}

// PrintEntry prints the boot entry information
func (e *BootEntry) PrintEntry() {
	if e.Title != "" {
		fmt.Printf("Title: %s\n", e.Title)
	}
	if e.Version != "" {
		fmt.Printf("Version: %s\n", e.Version)
	}
	if e.Linux != "" {
		fmt.Printf("Linux: %s\n", e.Linux)
	}
	if e.Initrd != "" {
		fmt.Printf("Initrd: %s\n", e.Initrd)
	}
	if e.Devicetree != "" {
		fmt.Printf("Devicetree: %s\n", e.Devicetree)
	}
	if e.Options != "" {
		fmt.Printf("Options: %s\n", e.Options)
	}
}
