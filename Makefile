# dnsplane Makefile
# Usage: make, make build, make test, make fuzz, make vet, make lint, make clean
# Build profiles: make build GOOS=windows, make build GOOS=linux GOARCH=arm64, make build CGO_ENABLED=0

BINARY_NAME := dnsplane
BUILD_DIR   := builds
FUZZTIME    ?= 12s

# Build profile (static by default; set CGO_ENABLED=1 if you need cgo)
CGO_ENABLED ?= 0
# Cross-compile: set GOOS and optionally GOARCH (e.g. make build GOOS=windows GOARCH=amd64)
GOOS        ?=
GOARCH      ?=

# Output path: native -> ./dnsplane; cross -> builds/dnsplane-OS-ARCH[.exe]
CROSS_SUFFIX := $(GOOS)-$(or $(GOARCH),amd64)
OUTPUT      := $(if $(GOOS),$(BUILD_DIR)/$(BINARY_NAME)-$(CROSS_SUFFIX)$(if $(filter windows,$(GOOS)),.exe,),$(BINARY_NAME))
# For go build: only set GOARCH when cross-compiling (GOOS set), default amd64
BUILD_GOARCH := $(if $(GOOS),$(or $(GOARCH),amd64),)

.PHONY: build test fuzz vet lint clean deps all help build-windows build-linux build-darwin build-all

# Default target: build (static, native)
all: build

# Build the binary. Static by default (CGO_ENABLED=0).
# Examples:
#   make build
#   make build GOOS=windows
#   make build GOOS=linux GOARCH=arm64
#   make build GOOS=darwin GOARCH=arm64
build:
	@mkdir -p $(dir $(OUTPUT))
	@echo "Building $(OUTPUT) (CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(BUILD_GOARCH))"
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(BUILD_GOARCH) go build -o $(OUTPUT) .

# Convenience: static builds for common targets (output in builds/)
build-windows:
	$(MAKE) build GOOS=windows GOARCH=amd64

build-linux:
	$(MAKE) build GOOS=linux GOARCH=amd64

build-linux-arm64:
	$(MAKE) build GOOS=linux GOARCH=arm64

build-darwin:
	$(MAKE) build GOOS=darwin GOARCH=amd64

build-darwin-arm64:
	$(MAKE) build GOOS=darwin GOARCH=arm64

# Build Windows, Linux (amd64), and Darwin (amd64 + arm64)
build-all: build-windows build-linux build-linux-arm64 build-darwin build-darwin-arm64

# Run all tests (no fuzz)
test:
	go test ./...

# Run tests with verbose output
test-verbose:
	go test -v ./...

# Run tests with race detector
test-race:
	go test -race ./...

# Run all fuzz targets (FUZZTIME=12s by default; override: make fuzz FUZZTIME=60s)
fuzz:
	go test -fuzz=FuzzValidateIP -fuzztime=$(FUZZTIME) ./ipvalidator/
	go test -fuzz=FuzzConvertIPToReverseDNS -fuzztime=$(FUZZTIME) ./converters/
	go test -fuzz=FuzzConvertReverseDNSToIP -fuzztime=$(FUZZTIME) ./converters/
	go test -fuzz=FuzzAddRecord -fuzztime=$(FUZZTIME) ./dnsrecords/
	go test -fuzz=FuzzNormalizeRecordNameKey -fuzztime=$(FUZZTIME) ./dnsrecords/
	go test -fuzz=FuzzNormalizeRecordTypeAndValue -fuzztime=$(FUZZTIME) ./dnsrecords/
	go test -fuzz=FuzzFindAllRecords -fuzztime=$(FUZZTIME) ./dnsrecords/
	go test -fuzz=FuzzServerMatchesQuery -fuzztime=$(FUZZTIME) ./dnsservers/
	go test -fuzz=FuzzGetServersForQuery -fuzztime=$(FUZZTIME) ./dnsservers/

# Run go vet
vet:
	go vet ./...

# Run go vet and staticcheck-style checks (go vet + errcheck-style is just vet)
# Extend with: staticcheck ./... if you add staticcheck
lint: vet

# Download module dependencies
deps:
	go mod download

# Tidy module
tidy:
	go mod tidy

# Remove built binary, builds/ directory, and go build cache
clean:
	rm -f $(BINARY_NAME)
	rm -rf $(BUILD_DIR)
	go clean -cache

# Show targets and usage
help:
	@echo "dnsplane Makefile"
	@echo ""
	@echo "Build (static by default: CGO_ENABLED=0):"
	@echo "  make / make build              - Build native binary ($(BINARY_NAME))"
	@echo "  make build GOOS=windows        - Cross-build for Windows (-> $(BUILD_DIR)/...)"
	@echo "  make build GOOS=linux GOARCH=arm64"
	@echo "  make build-windows             - Same as build GOOS=windows GOARCH=amd64"
	@echo "  make build-linux               - Linux amd64"
	@echo "  make build-linux-arm64         - Linux arm64"
	@echo "  make build-darwin / build-darwin-arm64"
	@echo "  make build-all                 - Windows + Linux (amd64/arm64) + Darwin (amd64/arm64)"
	@echo ""
	@echo "Tests and checks:"
	@echo "  make test           - Run tests"
	@echo "  make test-verbose   - Run tests with -v"
	@echo "  make test-race      - Run tests with -race"
	@echo "  make fuzz           - Run all fuzz targets (FUZZTIME=$(FUZZTIME); override: make fuzz FUZZTIME=60s)"
	@echo "  make vet            - Run go vet"
	@echo "  make lint           - Run vet"
	@echo ""
	@echo "Other:"
	@echo "  make deps           - go mod download"
	@echo "  make tidy           - go mod tidy"
	@echo "  make clean          - Remove binary, builds/, and go cache"
	@echo "  make help           - Show this help"
