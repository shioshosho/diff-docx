.PHONY: build install uninstall clean test

# Binary name
BINARY_NAME=ddx
BINARY_ALT=diff-docx

# Build directory
BUILD_DIR=./build

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Build flags for static binary
LDFLAGS=-ldflags "-s -w"

# Default target
all: build

# Build the binary
build:
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/ddx

# Build for multiple platforms
build-all: build-linux build-darwin build-windows

build-linux:
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/ddx
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd/ddx

build-darwin:
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/ddx
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/ddx

build-windows:
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe ./cmd/ddx

# Install directory
PREFIX ?= /usr/local
BINDIR = $(PREFIX)/bin

# Install to /usr/local/bin (run 'make build' first, then 'sudo make install')
install:
	@test -f $(BUILD_DIR)/$(BINARY_NAME) || (echo "Error: Run 'make build' first" && exit 1)
	install -d $(BINDIR)
	install -m 755 $(BUILD_DIR)/$(BINARY_NAME) $(BINDIR)/$(BINARY_NAME)
	ln -sf $(BINDIR)/$(BINARY_NAME) $(BINDIR)/$(BINARY_ALT)
	@echo "Installed $(BINARY_NAME) and $(BINARY_ALT) to $(BINDIR)"

# Uninstall from /usr/local/bin
uninstall:
	rm -f $(BINDIR)/$(BINARY_NAME)
	rm -f $(BINDIR)/$(BINARY_ALT)
	@echo "Uninstalled $(BINARY_NAME) and $(BINARY_ALT) from $(BINDIR)"

# Clean build artifacts
clean:
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)

# Run tests
test:
	$(GOTEST) -v ./...

# Download dependencies
deps:
	$(GOMOD) download
	$(GOMOD) tidy

# Check dependencies (external tools)
check-deps:
	@echo "Checking external dependencies..."
	@which markitdown > /dev/null 2>&1 || (echo "ERROR: markitdown not found. Install with: pip install markitdown" && exit 1)
	@which delta > /dev/null 2>&1 || (echo "ERROR: delta not found. Install from: https://github.com/dandavison/delta" && exit 1)
	@which magick > /dev/null 2>&1 || (echo "ERROR: ImageMagick not found. Install with your package manager" && exit 1)
	@echo "All dependencies found!"

# Help
help:
	@echo "Available targets:"
	@echo "  build        - Build the binary for current platform"
	@echo "  build-all    - Build for Linux, macOS, and Windows"
	@echo "  install      - Install to /usr/local/bin (requires prior build)"
	@echo "  uninstall    - Remove from /usr/local/bin"
	@echo "  clean        - Remove build artifacts"
	@echo "  test         - Run tests"
	@echo "  deps         - Download and tidy dependencies"
	@echo "  check-deps   - Check if external tools are installed"
	@echo "  help         - Show this help message"
	@echo ""
	@echo "Installation:"
	@echo "  make build && sudo make install"
	@echo ""
	@echo "Custom install location:"
	@echo "  make build && sudo make install PREFIX=/opt"
