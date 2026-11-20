# muken build helpers. Run from the repository root: make <target>
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

GO := cd src && go

.PHONY: build test check check-ci t dist sync-legal

build:
	CGO_ENABLED=0 $(GO) build -ldflags '$(LDFLAGS)' -o ../dist/muken ./cmd/muken

test:
	$(GO) vet ./...
	$(GO) test ./...

# The full check in one short command. It prints only problems, then "check ok".
check:
	@out="$$(cd src && go vet ./... 2>&1; go test ./... 2>&1 | grep -vE '^(ok|\?)'; gofmt -l .)"; \
	if [ -n "$$out" ]; then echo "$$out"; exit 1; fi; echo "check ok"

# The same check as CI: no home and no ssh-agent. A test must not depend on the machine.
check-ci:
	@env HOME="$$(mktemp -d)" SSH_AUTH_SOCK= GOCACHE="$$(go env GOCACHE)" GOMODCACHE="$$(go env GOMODCACHE)" $(MAKE) --no-print-directory check

# A few tests, last 20 lines of output: make t PKG=./internal/deploy RUN=Maintenance
t:
	@bash -o pipefail -c "cd src && go test $(or $(PKG),./...) $(if $(RUN),-run '$(RUN)') 2>&1 | tail -20"

# Static binaries for every platform. The linux ones are also what `muken agent install` uploads.
dist:
	@for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64; do \
		os=$${target%/*}; arch=$${target#*/}; \
		echo "build dist/muken-$$os-$$arch"; \
		(cd src && CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags '$(LDFLAGS)' -o ../dist/muken-$$os-$$arch ./cmd/muken) || exit 1; \
	done

# The legal texts at the root are the source. go:embed cannot read a parent folder, so src/legal/ holds copies.
# A test fails when a copy differs.
sync-legal:
	cp LICENSE NOTICE TRADEMARK.md THIRD_PARTY_NOTICES.txt src/legal/
