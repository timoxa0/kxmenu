# kxmenu Makefile
# Build configuration for cross-platform support

# Default values
GOOS ?= linux
GOARCH ?= amd64
BINARY_NAME ?= kxmenu
BUILD_DIR ?= build
PREFIX ?= /usr/local
LDFLAGS = -s -w -extldflags '-static'
CGO_ENABLED = 0

# Version info
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS += -X github.com/timoxa0/kxmenu/cmd.Version=$(VERSION) -X github.com/timoxa0/kxmenu/cmd.BuildTime=$(BUILD_TIME)

# Source files
SRC = $(shell find . -name "*.go" -type f)

# Default target
.PHONY: all
all: build

# Build binary
.PHONY: build
build: $(BUILD_DIR)/$(BINARY_NAME)

$(BUILD_DIR)/$(BINARY_NAME): $(SRC)
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) go build \
		-ldflags "$(LDFLAGS)" \
		-o $(BUILD_DIR)/$(BINARY_NAME) \
		.

# Clean build artifacts
.PHONY: clean
clean:
	rm -rf $(BUILD_DIR)
	rm -f $(BINARY_NAME)

# Install binary to system
.PHONY: install
install: build
	@mkdir -p $(PREFIX)/sbin
	install -m 755 $(BUILD_DIR)/$(BINARY_NAME) $(PREFIX)/sbin/

# Show build information
.PHONY: info
info:
	@echo "Build Configuration:"
	@echo "  GOOS:          $(GOOS)"
	@echo "  GOARCH:        $(GOARCH)"
	@echo "  CGO_ENABLED:   $(CGO_ENABLED)"
	@echo "  BINARY_NAME:   $(BINARY_NAME)"
	@echo "  BUILD_DIR:     $(BUILD_DIR)"
	@echo "  PREFIX:        $(PREFIX)"
	@echo "  VERSION:       $(VERSION)"
	@echo "  BUILD_TIME:    $(BUILD_TIME)"
	@echo "  LDFLAGS:       $(LDFLAGS)"

# Show help
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build                - Build binary for specified GOOS/GOARCH"
	@echo "  clean                - Clean build artifacts"
	@echo "  install              - Install binary to PREFIX/sbin (default: /usr/local/sbin)"
	@echo "  info                 - Show build configuration"
	@echo "  help                 - Show this help"
	@echo ""
	@echo "Variables:"
	@echo "  GOOS=linux           - Target operating system"
	@echo "  GOARCH=amd64         - Target architecture"
	@echo "  BINARY_NAME=kxmenu   - Output binary name"
	@echo "  BUILD_DIR=build      - Build output directory"
	@echo "  PREFIX=/usr/local    - Installation prefix"