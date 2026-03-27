GOBIN ?= $(shell go env GOPATH)/bin
WHISPER_INC ?= /usr/local/include/whisper
WHISPER_LIB ?= /usr/local/lib/whisper
CGO_CFLAGS ?= -I$(WHISPER_INC)
CGO_LDFLAGS ?= -L$(WHISPER_LIB)

# Detect OS for platform-specific targets.
UNAME_S := $(shell uname -s 2>/dev/null || echo Windows)
ifeq ($(UNAME_S),Darwin)
  BINARY := bin/brabble
  LD_ENV := DYLD_LIBRARY_PATH=$(WHISPER_LIB)
else ifeq ($(OS),Windows_NT)
  BINARY := bin/brabble.exe
  LD_ENV :=
else
  BINARY := bin/brabble
  LD_ENV := LD_LIBRARY_PATH=$(WHISPER_LIB)
endif

.PHONY: lint fmt test build

fmt:
	gofmt -w -s .

lint:
	@command -v golangci-lint >/dev/null 2>&1 || { echo "golangci-lint not installed. Install via: brew install golangci-lint"; exit 1; }
	$(LD_ENV) CGO_CFLAGS='$(CGO_CFLAGS)' CGO_LDFLAGS='$(CGO_LDFLAGS)' golangci-lint run

test:
	$(LD_ENV) CGO_CFLAGS='$(CGO_CFLAGS)' CGO_LDFLAGS='$(CGO_LDFLAGS)' go test ./...

build:
	CGO_CFLAGS='$(CGO_CFLAGS)' CGO_LDFLAGS='$(CGO_LDFLAGS)' go build -o $(BINARY) ./cmd/brabble
	@command -v install_name_tool >/dev/null 2>&1 && install_name_tool -add_rpath $(WHISPER_LIB) $(BINARY) || true
