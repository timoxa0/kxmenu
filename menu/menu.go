package menu

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/timoxa0/kxmenu/entry"
	"github.com/timoxa0/kxmenu/input"
)

// Terminal represents terminal capabilities and state
type Terminal struct {
	Width  int
	Height int
	IsTTY  bool
}

// MenuItem represents a menu item in the boot menu
type MenuItem struct {
	Entry       *entry.BootEntry
	DisplayName string
	Description string
}

// BootMenu represents the interactive boot menu
type BootMenu struct {
	Items         []MenuItem
	SelectedIndex int
	Terminal      *Terminal
	Title         string
	Timeout       int                 // seconds, 0 = no timeout
	InputManager  *input.InputManager // Hardware input support
}

// ANSI escape codes for terminal control
const (
	EscSeq        = "\033["
	ClearScreen   = EscSeq + "2J"
	ClearLine     = EscSeq + "K"
	HideCursor    = EscSeq + "?25l"
	ShowCursor    = EscSeq + "?25h"
	SaveCursor    = EscSeq + "s"
	RestoreCursor = EscSeq + "u"
	ResetColor    = EscSeq + "0m"
	BoldText      = EscSeq + "1m"
	ReverseVideo  = EscSeq + "7m"
	BlueText      = EscSeq + "34m"
	WhiteText     = EscSeq + "37m"
	CyanText      = EscSeq + "36m"
)

// NewTerminal detects terminal capabilities
func NewTerminal() *Terminal {
	term := &Terminal{
		Width:  80, // default fallback
		Height: 24, // default fallback
		IsTTY:  false,
	}

	// Check if stdout is a TTY
	if isatty(int(os.Stdout.Fd())) {
		term.IsTTY = true
		// Try to get actual terminal size
		if width, height := getTerminalSize(); width > 0 && height > 0 {
			term.Width = width
			term.Height = height
		}
	}

	return term
}

// isatty checks if the file descriptor is a terminal
func isatty(fd int) bool {
	var termios syscall.Termios
	_, _, err := syscall.Syscall6(syscall.SYS_IOCTL, uintptr(fd), 0x5401, uintptr(unsafe.Pointer(&termios)), 0, 0, 0) // TCGETS
	return err == 0
}

// getTerminalSize gets the current terminal dimensions
func getTerminalSize() (int, int) {
	type winsize struct {
		Row    uint16
		Col    uint16
		Xpixel uint16
		Ypixel uint16
	}

	ws := &winsize{}
	ret, _, _ := syscall.Syscall(syscall.SYS_IOCTL,
		uintptr(syscall.Stdin),
		uintptr(0x5413), // TIOCGWINSZ
		uintptr(unsafe.Pointer(ws)))

	if int(ret) == -1 {
		// Fallback: try using tput command
		if cmd := exec.Command("tput", "cols"); cmd.Err == nil {
			if output, err := cmd.Output(); err == nil {
				if cols, err := strconv.Atoi(strings.TrimSpace(string(output))); err == nil {
					if cmd := exec.Command("tput", "lines"); cmd.Err == nil {
						if output, err := cmd.Output(); err == nil {
							if lines, err := strconv.Atoi(strings.TrimSpace(string(output))); err == nil {
								return cols, lines
							}
						}
					}
				}
			}
		}
		return 0, 0
	}

	return int(ws.Col), int(ws.Row)
}

// NewBootMenu creates a new boot menu
func NewBootMenu(entries []*entry.BootEntry, title string) *BootMenu {
	menu := &BootMenu{
		Items:         make([]MenuItem, len(entries)),
		SelectedIndex: 0,
		Terminal:      NewTerminal(),
		Title:         title,
		Timeout:       0,
	}

	// Convert entries to menu items
	for i, e := range entries {
		displayName := e.Title
		if displayName == "" {
			displayName = fmt.Sprintf("Boot Entry %d", i+1)
		}

		description := ""
		if e.Version != "" {
			description = fmt.Sprintf("Version: %s", e.Version)
		}
		if e.Linux != "" {
			if description != "" {
				description += " | "
			}
			description += fmt.Sprintf("Kernel: %s", e.Linux)
		}

		menu.Items[i] = MenuItem{
			Entry:       e,
			DisplayName: displayName,
			Description: description,
		}
	}

	return menu
}

// NewBootMenuWithInput creates a new boot menu with hardware input support
func NewBootMenuWithInput(entries []*entry.BootEntry, title string, inputMgr *input.InputManager) *BootMenu {
	menu := NewBootMenu(entries, title)
	menu.InputManager = inputMgr
	return menu
}

// SetTimeout sets the menu timeout in seconds (0 = no timeout)
func (m *BootMenu) SetTimeout(seconds int) {
	m.Timeout = seconds
}

// Show displays the boot menu and handles user interaction
func (m *BootMenu) Show() (*entry.BootEntry, error) {
	if !m.Terminal.IsTTY {
		// Fallback to simple text menu for non-TTY
		return m.showSimpleMenu()
	}

	// Setup terminal for interactive mode
	if err := m.setupTerminal(); err != nil {
		return m.showSimpleMenu()
	}
	defer m.restoreTerminal()

	// Show the menu
	return m.showInteractiveMenu()
}

// showSimpleMenu shows a simple numbered list for non-TTY environments
func (m *BootMenu) showSimpleMenu() (*entry.BootEntry, error) {
	fmt.Printf("\n%s\n", m.Title)
	fmt.Println(strings.Repeat("=", len(m.Title)))

	for i, item := range m.Items {
		fmt.Printf("%d. %s\n", i+1, item.DisplayName)
		if item.Description != "" {
			fmt.Printf("   %s\n", item.Description)
		}
	}

	fmt.Printf("\nSelect entry (1-%d) [default: 1]: ", len(m.Items))

	var input string
	fmt.Scanln(&input)

	if input == "" {
		return m.Items[0].Entry, nil
	}

	selection, err := strconv.Atoi(input)
	if err != nil || selection < 1 || selection > len(m.Items) {
		return nil, fmt.Errorf("invalid selection: %s", input)
	}

	return m.Items[selection-1].Entry, nil
}

// setupTerminal prepares terminal for interactive mode
func (m *BootMenu) setupTerminal() error {
	// Put terminal in raw mode
	cmd := exec.Command("stty", "-echo", "cbreak")
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return err
	}

	// Hide cursor and clear screen
	fmt.Print(HideCursor + ClearScreen)
	return nil
}

// restoreTerminal restores normal terminal mode
func (m *BootMenu) restoreTerminal() {
	// Restore normal terminal mode
	fmt.Print(ShowCursor + ResetColor)
	cmd := exec.Command("stty", "echo", "-cbreak")
	cmd.Stdin = os.Stdin
	cmd.Run()
}

// showInteractiveMenu shows the interactive GRUB2-style menu with hardware input support
func (m *BootMenu) showInteractiveMenu() (*entry.BootEntry, error) {
	// Channel for timeout handling
	timeoutCh := make(chan bool, 1)
	inputCh := make(chan byte, 1)

	// Start timeout if configured
	if m.Timeout > 0 {
		go func() {
			time.Sleep(time.Duration(m.Timeout) * time.Second)
			timeoutCh <- true
		}()
	}

	// Start input readers
	if m.InputManager != nil {
		// Use hardware input manager
		go func() {
			for {
				event := m.InputManager.GetEvent()
				switch event.Code {
				case input.KeyUp:
					inputCh <- 'k' // Simulate Vi up key
				case input.KeyDown:
					inputCh <- 'j' // Simulate Vi down key
				case input.KeySelect:
					inputCh <- 13 // Simulate Enter
				case input.KeyQuit, input.KeyEscape:
					inputCh <- 'q' // Simulate quit
				}
			}
		}()
	}

	// Keyboard fallback
	go func() {
		buf := make([]byte, 1)
		for {
			os.Stdin.Read(buf)
			inputCh <- buf[0]
		}
	}()

	// Main menu loop
	for {
		m.drawMenu()

		select {
		case <-timeoutCh:
			// Timeout reached, select current item
			return m.Items[m.SelectedIndex].Entry, nil

		case key := <-inputCh:
			switch key {
			case 10, 13: // Enter
				return m.Items[m.SelectedIndex].Entry, nil

			case 27: // ESC sequence
				// Read the next two bytes for arrow keys
				buf := make([]byte, 2)
				os.Stdin.Read(buf)
				if buf[0] == '[' {
					switch buf[1] {
					case 'A': // Up arrow
						if m.SelectedIndex > 0 {
							m.SelectedIndex--
						}
					case 'B': // Down arrow
						if m.SelectedIndex < len(m.Items)-1 {
							m.SelectedIndex++
						}
					}
				}

			case 'q', 'Q': // Quit
				return nil, fmt.Errorf("menu cancelled by user")

			case 'j': // Vi-style down
				if m.SelectedIndex < len(m.Items)-1 {
					m.SelectedIndex++
				}

			case 'k': // Vi-style up
				if m.SelectedIndex > 0 {
					m.SelectedIndex--
				}

			default:
				// Number key selection
				if key >= '1' && key <= '9' {
					index := int(key - '1')
					if index < len(m.Items) {
						m.SelectedIndex = index
						return m.Items[m.SelectedIndex].Entry, nil
					}
				}
			}
		}
	}
}

// drawMenu renders the boot menu
func (m *BootMenu) drawMenu() {
	// Calculate menu dimensions
	menuHeight := len(m.Items) + 6 // title + border + footer
	startRow := max(1, (m.Terminal.Height-menuHeight)/2)

	// Clear screen and position cursor
	fmt.Print(ClearScreen + EscSeq + fmt.Sprintf("%d;1H", startRow))

	// Draw title
	titlePadding := max(0, (m.Terminal.Width-len(m.Title))/2)
	fmt.Print(strings.Repeat(" ", titlePadding) + BoldText + CyanText + m.Title + ResetColor + "\n")

	// Draw separator
	separator := strings.Repeat("─", min(m.Terminal.Width-4, len(m.Title)+10))
	separatorPadding := max(0, (m.Terminal.Width-len(separator))/2)
	fmt.Print(strings.Repeat(" ", separatorPadding) + separator + "\n\n")

	// Draw menu items
	for i, item := range m.Items {
		prefix := "  "
		suffix := ""

		if i == m.SelectedIndex {
			prefix = BoldText + ReverseVideo + "> "
			suffix = ResetColor
		}

		// Truncate long names to fit terminal width
		displayName := item.DisplayName
		maxNameWidth := m.Terminal.Width - 6
		if len(displayName) > maxNameWidth {
			displayName = displayName[:maxNameWidth-3] + "..."
		}

		fmt.Printf("%s%s%s\n", prefix, displayName, suffix)

		// Show description for selected item
		if i == m.SelectedIndex && item.Description != "" {
			descPadding := "    "
			maxDescWidth := m.Terminal.Width - 8
			description := item.Description
			if len(description) > maxDescWidth {
				description = description[:maxDescWidth-3] + "..."
			}
			fmt.Printf("%s%s%s%s\n", descPadding, BlueText, description, ResetColor)
		}
	}

	// Draw footer
	fmt.Print("\n")
	footer := "Use ↑↓ arrows, Enter to select, 'q' to quit"
	if m.Timeout > 0 {
		footer += fmt.Sprintf(" (timeout: %ds)", m.Timeout)
	}

	footerPadding := max(0, (m.Terminal.Width-len(footer))/2)
	fmt.Print(strings.Repeat(" ", footerPadding) + WhiteText + footer + ResetColor)
}

// Helper functions
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
