GO ?= go
BINDIR ?= $(HOME)/.local/bin
BINARY ?= diffman
BUILD_DIR ?= ./bin
BUILD_OUT ?= $(BUILD_DIR)/$(BINARY)

.PHONY: test lint run build install

test:
	$(GO) test ./...

lint:
	@unformatted="$$(gofmt -l $$(find . -name '*.go' -type f))"; \
	if [ -n "$$unformatted" ]; then \
		echo "The following files are not gofmt-formatted:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi
	$(GO) vet ./...

run:
	$(GO) run ./cmd/diffman

build:
	mkdir -p "$(BUILD_DIR)"
	$(GO) build -o "$(BUILD_OUT)" ./cmd/diffman

install: build
	mkdir -p "$(BINDIR)"
	cp "$(BUILD_OUT)" "$(BINDIR)/$(BINARY)"
