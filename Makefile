BINARY   := vconnect
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS  := -ldflags "-X main.version=$(VERSION) -s -w"
GO       := go

.PHONY: build test clean install vet

build:
	$(GO) build $(LDFLAGS) -o $(BINARY) ./cmd/vconnect/

# Static binary (for distribution)
build-static:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
	$(GO) build $(LDFLAGS) -o $(BINARY)-linux-amd64 ./cmd/vconnect/

test:
	$(GO) test -race -count=1 ./...

vet:
	$(GO) vet ./...
	$(GO) build ./... # catches type errors not caught by vet

install:
	$(GO) install $(LDFLAGS) ./cmd/vconnect/

clean:
	rm -f $(BINARY) $(BINARY)-linux-amd64

# Run in development mode (stdio MCP, uses ./tcl/bootstrap.tcl)
run:
	./$(BINARY) -vivado $(VIVADO_BIN)

tidy:
	$(GO) mod tidy
