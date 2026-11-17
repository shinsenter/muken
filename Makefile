# muken build helpers. Run from the repository root: make <target>
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build test dist

build:
	CGO_ENABLED=0 go build -ldflags '$(LDFLAGS)' -o dist/muken ./cmd/muken

test:
	go vet ./...
	go test ./...

# Static binaries for every platform. The linux ones are also what `muken agent install` uploads.
dist:
	@for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64; do \
		os=$${target%/*}; arch=$${target#*/}; \
		echo "build dist/muken-$$os-$$arch"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags '$(LDFLAGS)' -o dist/muken-$$os-$$arch ./cmd/muken || exit 1; \
	done
