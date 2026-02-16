VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
LDFLAGS  = -ldflags "-X github.com/jamesatintegratnio/hctl/cmd.Version=$(VERSION) -X github.com/jamesatintegratnio/hctl/cmd.Commit=$(COMMIT)"
BINARY   = hctl

.PHONY: build clean test lint install

build:
	go build $(LDFLAGS) -o bin/$(BINARY) .

install:
	go install $(LDFLAGS) .

test:
	go test ./... -v

lint:
	golangci-lint run ./...

clean:
	rm -rf bin/

# Development helpers
.PHONY: run fmt tidy

run: build
	./bin/$(BINARY) $(ARGS)

fmt:
	gofmt -s -w .

tidy:
	go mod tidy

# Release
.PHONY: snapshot release

snapshot:
	goreleaser release --snapshot --clean

release:
	goreleaser release --clean
