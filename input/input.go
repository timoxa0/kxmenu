package input

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

// KeyEvent represents a key press event
type KeyEvent struct {
	Code KeyCode
	Type EventType
}

// KeyCode represents different input keys
type KeyCode int

const (
	KeyUnknown KeyCode = iota
	KeyUp              // Volume Up or Arrow Up
	KeyDown            // Volume Down or Arrow Down
	KeySelect          // Power button or Enter
	KeyEscape          // Escape
	KeyQuit            // Q key
)

// EventType represents the type of key event
type EventType int

const (
	KeyPress EventType = iota
	KeyRelease
)

// InputDevice represents an input device
type InputDevice struct {
	Name string
	Path string
	File *os.File
}

// InputManager manages multiple input sources
type InputManager struct {
	devices    []InputDevice
	eventChan  chan KeyEvent
	stopChan   chan bool
	keyboardCh chan byte
}

// Linux input event structure
type inputEvent struct {
	Time  syscall.Timeval
	Type  uint16
	Code  uint16
	Value int32
}

// Input event constants
const (
	EV_KEY = 0x01
	EV_SYN = 0x00
)

// Key codes for hardware buttons
const (
	KEY_VOLUMEUP   = 115
	KEY_VOLUMEDOWN = 114
	KEY_POWER      = 116
	KEY_ENTER      = 28
	KEY_UP         = 103
	KEY_DOWN       = 108
	KEY_ESC        = 1
	KEY_Q          = 16
)

// NewInputManager creates a new input manager
func NewInputManager() *InputManager {
	return &InputManager{
		devices:    make([]InputDevice, 0),
		eventChan:  make(chan KeyEvent, 10),
		stopChan:   make(chan bool, 1),
		keyboardCh: make(chan byte, 10),
	}
}

// DiscoverDevices finds available input devices
func (im *InputManager) DiscoverDevices() error {
	// Look for input devices in /dev/input/
	inputDir := "/dev/input"
	entries, err := os.ReadDir(inputDir)
	if err != nil {
		return fmt.Errorf("failed to read input directory: %v", err)
	}

	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "event") {
			devicePath := filepath.Join(inputDir, entry.Name())

			// Try to open the device
			file, err := os.OpenFile(devicePath, os.O_RDONLY, 0)
			if err != nil {
				continue // Skip devices we can't open
			}

			// Get device name
			name, err := getDeviceName(file)
			if err != nil {
				file.Close()
				continue
			}

			// Check if this device has the keys we're interested in
			if hasRelevantKeys(file, name) {
				device := InputDevice{
					Name: name,
					Path: devicePath,
					File: file,
				}
				im.devices = append(im.devices, device)
				fmt.Printf("Found input device: %s (%s)\n", name, devicePath)
			} else {
				file.Close()
			}
		}
	}

	return nil
}

// getDeviceName gets the name of an input device
func getDeviceName(file *os.File) (string, error) {
	name := make([]byte, 256)
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL,
		file.Fd(),
		uintptr(0x80ff4506), // EVIOCGNAME
		uintptr(unsafe.Pointer(&name[0])))

	if errno != 0 {
		return "", fmt.Errorf("failed to get device name")
	}

	// Find null terminator and convert to string
	end := 0
	for i, b := range name {
		if b == 0 {
			end = i
			break
		}
	}

	return string(name[:end]), nil
}

// hasRelevantKeys checks if a device has volume/power keys
func hasRelevantKeys(file *os.File, name string) bool {
	// For simplicity, check device name patterns
	name = strings.ToLower(name)

	// Common patterns for devices with hardware buttons
	patterns := []string{
		"gpio-keys",
		"power",
		"volume",
		"button",
		"pmic",
		"keyboard",
	}

	for _, pattern := range patterns {
		if strings.Contains(name, pattern) {
			return true
		}
	}

	return false
}

// StartListening starts listening for input events
func (im *InputManager) StartListening() {
	// Start hardware device listeners
	for i := range im.devices {
		go im.listenDevice(&im.devices[i])
	}

	// Start keyboard listener for fallback
	go im.listenKeyboard()
}

// listenDevice listens for events from a hardware device
func (im *InputManager) listenDevice(device *InputDevice) {
	eventSize := int(unsafe.Sizeof(inputEvent{}))
	buf := make([]byte, eventSize)

	for {
		select {
		case <-im.stopChan:
			return
		default:
			n, err := device.File.Read(buf)
			if err != nil || n != eventSize {
				continue
			}

			// Parse the input event
			event := (*inputEvent)(unsafe.Pointer(&buf[0]))

			if event.Type == EV_KEY && event.Value == 1 { // Key press
				keyEvent := im.translateKeyCode(event.Code)
				if keyEvent.Code != KeyUnknown {
					select {
					case im.eventChan <- keyEvent:
					default:
						// Channel full, drop event
					}
				}
			}
		}
	}
}

// listenKeyboard listens for keyboard input as fallback
func (im *InputManager) listenKeyboard() {
	// Set up raw keyboard input
	reader := bufio.NewReader(os.Stdin)

	for {
		select {
		case <-im.stopChan:
			return
		default:
			char, err := reader.ReadByte()
			if err != nil {
				continue
			}

			var keyEvent KeyEvent
			keyEvent.Type = KeyPress

			switch char {
			case 27: // ESC sequence
				// Try to read arrow keys
				if next, err := reader.ReadByte(); err == nil && next == '[' {
					if arrow, err := reader.ReadByte(); err == nil {
						switch arrow {
						case 'A': // Up arrow
							keyEvent.Code = KeyUp
						case 'B': // Down arrow
							keyEvent.Code = KeyDown
						default:
							keyEvent.Code = KeyEscape
						}
					} else {
						keyEvent.Code = KeyEscape
					}
				} else {
					keyEvent.Code = KeyEscape
				}
			case 10, 13: // Enter
				keyEvent.Code = KeySelect
			case 'j': // Vi-style down
				keyEvent.Code = KeyDown
			case 'k': // Vi-style up
				keyEvent.Code = KeyUp
			case 'q', 'Q': // Quit
				keyEvent.Code = KeyQuit
			default:
				// Number keys for direct selection
				if char >= '1' && char <= '9' {
					// Store the number in a special way
					keyEvent.Code = KeyCode(int(KeyQuit) + int(char-'0'))
				} else {
					continue // Ignore other keys
				}
			}

			select {
			case im.eventChan <- keyEvent:
			default:
				// Channel full, drop event
			}
		}
	}
}

// translateKeyCode translates Linux key codes to our KeyCode enum
func (im *InputManager) translateKeyCode(linuxCode uint16) KeyEvent {
	var keyEvent KeyEvent
	keyEvent.Type = KeyPress

	switch linuxCode {
	case KEY_VOLUMEUP, KEY_UP:
		keyEvent.Code = KeyUp
	case KEY_VOLUMEDOWN, KEY_DOWN:
		keyEvent.Code = KeyDown
	case KEY_POWER, KEY_ENTER:
		keyEvent.Code = KeySelect
	case KEY_ESC:
		keyEvent.Code = KeyEscape
	case KEY_Q:
		keyEvent.Code = KeyQuit
	default:
		keyEvent.Code = KeyUnknown
	}

	return keyEvent
}

// GetEvent returns the next input event (blocking)
func (im *InputManager) GetEvent() KeyEvent {
	return <-im.eventChan
}

// GetEventNonBlocking returns the next input event (non-blocking)
func (im *InputManager) GetEventNonBlocking() (KeyEvent, bool) {
	select {
	case event := <-im.eventChan:
		return event, true
	default:
		return KeyEvent{}, false
	}
}

// Stop stops listening for input events
func (im *InputManager) Stop() {
	close(im.stopChan)

	// Close device files
	for _, device := range im.devices {
		device.File.Close()
	}
}

// SetupTerminal prepares terminal for raw input
func SetupTerminal() error {
	// Put terminal in raw mode for keyboard input
	cmd := "stty -echo cbreak"
	return syscall.Exec("/bin/sh", []string{"/bin/sh", "-c", cmd}, os.Environ())
}

// RestoreTerminal restores normal terminal mode
func RestoreTerminal() error {
	cmd := "stty echo -cbreak"
	return syscall.Exec("/bin/sh", []string{"/bin/sh", "-c", cmd}, os.Environ())
}
