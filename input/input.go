package input

import (
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
	Name      string
	Path      string
	File      *os.File
	keyStates map[uint16]bool // Track pressed state of each key
}

// InputManager manages multiple input sources
type InputManager struct {
	devices   []InputDevice
	eventChan chan KeyEvent
	stopChan  chan bool
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
	KEY_ESC        = 1
	KEY_1          = 2
	KEY_2          = 3
	KEY_3          = 4
	KEY_4          = 5
	KEY_5          = 6
	KEY_6          = 7
	KEY_7          = 8
	KEY_8          = 9
	KEY_9          = 10
	KEY_Q          = 16
	KEY_ENTER      = 28
	KEY_UP         = 103
	KEY_DOWN       = 108
	KEY_VOLUMEDOWN = 114
	KEY_VOLUMEUP   = 115
	KEY_POWER      = 116
)

// NewInputManager creates a new input manager
func NewInputManager() *InputManager {
	return &InputManager{
		devices:   make([]InputDevice, 0),
		eventChan: make(chan KeyEvent, 10),
		stopChan:  make(chan bool, 1),
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

			// Check if device supports EV_KEY events
			if supportsKeyEvents(file) {
				device := InputDevice{
					Name:      name,
					Path:      devicePath,
					File:      file,
					keyStates: make(map[uint16]bool),
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

// supportsKeyEvents checks if a device supports EV_KEY events
func supportsKeyEvents(file *os.File) bool {
	// Check if device supports EV_KEY events
	evBits := make([]byte, 4) // EV_MAX is 0x1f, so 4 bytes is enough
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL,
		file.Fd(),
		uintptr(0x80044520), // EVIOCGBIT(0, sizeof(ev_bits)) - get supported event types
		uintptr(unsafe.Pointer(&evBits[0])))

	if errno != 0 {
		return false
	}

	// Check if EV_KEY bit is set (bit 1)
	return evBits[0]&0x02 != 0 // EV_KEY = 1, so bit 1
}

// StartListening starts listening for input events
func (im *InputManager) StartListening() {
	// Start hardware device listeners
	for i := range im.devices {
		go im.listenDevice(&im.devices[i])
	}
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
			// Read raw input event
			n, err := device.File.Read(buf)
			if err != nil {
				// Device disconnected or error, exit this goroutine
				fmt.Printf("Error reading from device %s: %v\n", device.Path, err)
				return
			}

			if n != eventSize {
				// Incomplete read, skip this event
				continue
			}

			// Parse the input event
			event := (*inputEvent)(unsafe.Pointer(&buf[0]))

			// Only process EV_KEY events
			if event.Type == EV_KEY {
				im.handleKeyEvent(device, event.Code, event.Value)
			}
			// Ignore other event types (EV_SYN, etc.)
		}
	}
}

// handleKeyEvent processes key events and only sends KeyEvent when complete press cycle is detected
func (im *InputManager) handleKeyEvent(device *InputDevice, keyCode uint16, value int32) {
	switch value {
	case 1: // Key press
		device.keyStates[keyCode] = true
	case 0: // Key release
		// Only send key event if we previously recorded a press for this key
		if device.keyStates[keyCode] {
			device.keyStates[keyCode] = false
			keyEvent := im.translateKeyCode(keyCode)
			if keyEvent.Code != KeyUnknown {
				select {
				case im.eventChan <- keyEvent:
				default:
					// Channel full, drop event to avoid blocking
				}
			}
		}
	case 2: // Key repeat - ignore
		// Do nothing for key repeats
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
	// Support number keys 1-9 for direct selection (useful for menu navigation)
	case KEY_1, KEY_2, KEY_3, KEY_4, KEY_5, KEY_6, KEY_7, KEY_8, KEY_9:
		// Map numbers to special key codes above KeyQuit
		keyEvent.Code = KeyCode(int(KeyQuit) + int(linuxCode-KEY_1+1))
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
