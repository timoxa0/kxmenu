package kexec

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/timoxa0/kxmenu/entry"
)

// LoadEntry handles kexec operations for boot entries
func LoadEntry(entryFile, bootRoot string) error {
	// Set defaults if not provided
	if entryFile == "" {
		entryFile = "entry.conf"
	}
	if bootRoot == "" {
		bootRoot = "/mnt"
	}

	// Parse the boot entry configuration
	bootEntry, err := entry.ParseEntry(entryFile)
	if err != nil {
		return fmt.Errorf("failed to parse boot entry: %v", err)
	}

	return LoadEntryFromParsed(bootEntry, bootRoot)
}

// LoadEntryFromParsed handles kexec operations for an already parsed boot entry
func LoadEntryFromParsed(bootEntry *entry.BootEntry, bootRoot string) error {
	// Set default if not provided
	if bootRoot == "" {
		bootRoot = "/mnt"
	}

	// Clean up tuned parameters
	bootEntry.CleanupEntry()

	// Print boot entry information
	bootEntry.PrintEntry()

	// Prepare kernel path and handle decompression if needed
	kernelPath := filepath.Join(bootRoot, bootEntry.Linux)
	if strings.HasPrefix(filepath.Base(bootEntry.Linux), "vmlinuz") {
		decompressedPath, err := decompressKernel(kernelPath)
		if err != nil {
			return fmt.Errorf("decompression failed: %v", err)
		}
		kernelPath = decompressedPath
		defer os.Remove(decompressedPath)
	}

	// Load kernel with kexec
	err := loadKernel(kernelPath, bootRoot, bootEntry)
	if err != nil {
		return fmt.Errorf("failed to load kernel: %v", err)
	}

	return executeKexec()
}

// decompressKernel decompresses a gzipped vmlinuz kernel to a temporary file
func decompressKernel(kernelPath string) (string, error) {
	fmt.Println("Decompressing linux...")

	// Open the compressed kernel file
	file, err := os.Open(kernelPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// Create gzip reader
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return "", err
	}
	defer gzipReader.Close()

	// Create temporary file for decompressed kernel
	tmpFile, err := os.CreateTemp("/tmp", "kexec-decompressed-*.img")
	if err != nil {
		return "", err
	}
	defer tmpFile.Close()

	// Copy decompressed data to temporary file
	_, err = io.Copy(tmpFile, gzipReader)
	if err != nil {
		os.Remove(tmpFile.Name())
		return "", err
	}

	return tmpFile.Name(), nil
}

// loadKernel loads the kernel using kexec with the specified parameters
func loadKernel(kernelPath, bootRoot string, bootEntry *entry.BootEntry) error {
	fmt.Println("Loading linux...")

	args := []string{"--load", kernelPath}

	// Add initrd if specified
	if bootEntry.Initrd != "" {
		initrdPath := filepath.Join(bootRoot, bootEntry.Initrd)
		args = append(args, "--initrd="+initrdPath)
	}

	// Add device tree if specified
	if bootEntry.Devicetree != "" {
		dtbPath := filepath.Join(bootRoot, bootEntry.Devicetree)
		args = append(args, "--dtb="+dtbPath)
	}

	// Add command line options if specified
	if bootEntry.Options != "" {
		args = append(args, "--command-line="+bootEntry.Options)
	}

	cmd := exec.Command("kexec", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// executeKexec executes the loaded kernel
func executeKexec() error {
	cmd := exec.Command("kexec", "-e")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
