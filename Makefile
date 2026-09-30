.PHONY: build install clean test lint fmt deps release upgrade-local release-dry-run release-snapshot release-check

BINARY := ssmsearch
BUILD_DIR := bin
VERSION := $(shell git describe --tags --exact-match 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.gitCommit=$(COMMIT)"

build:
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY) ./cmd/ssmsearch

install:
	go install $(LDFLAGS) ./cmd/ssmsearch

clean:
	rm -rf $(BUILD_DIR)

test:
	go test -race -v ./...

lint:
	golangci-lint run ./...

fmt:
	go fmt ./...

deps:
	go mod download
	go mod tidy

# Create and push a release tag
# Usage: make release [TAG=v1.0.0]
release:
	@if [ -z "$(TAG)" ]; then \
		LATEST=$$(git tag --sort=-version:refname 2>/dev/null | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$$' | head -1); \
		if [ -z "$$LATEST" ]; then \
			SUGGESTED="v0.1.0"; \
		else \
			MAJOR=$$(echo $$LATEST | sed 's/v\([0-9]*\)\.\([0-9]*\)\.\([0-9]*\)/\1/'); \
			MINOR=$$(echo $$LATEST | sed 's/v\([0-9]*\)\.\([0-9]*\)\.\([0-9]*\)/\2/'); \
			PATCH=$$(echo $$LATEST | sed 's/v\([0-9]*\)\.\([0-9]*\)\.\([0-9]*\)/\3/'); \
			PATCH=$$((PATCH + 1)); \
			SUGGESTED="v$$MAJOR.$$MINOR.$$PATCH"; \
		fi; \
		echo "Latest tag: $${LATEST:-none}"; \
		printf "Enter tag [$$SUGGESTED]: "; \
		read INPUT_TAG; \
		TAG=$${INPUT_TAG:-$$SUGGESTED}; \
		echo "Creating release $$TAG..."; \
		git tag -a $$TAG -m "Release $$TAG" && \
		git push origin $$TAG && \
		echo "Release $$TAG pushed. GitHub Actions will build and publish artifacts."; \
	else \
		echo "Creating release $(TAG)..."; \
		git tag -a $(TAG) -m "Release $(TAG)" && \
		git push origin $(TAG) && \
		echo "Release $(TAG) pushed. GitHub Actions will build and publish artifacts."; \
	fi

# Watch the GitHub Actions release run, then upgrade/install via Homebrew if available
# Usage: make upgrade-local [TAG=v1.0.0]  (defaults to latest tag)
BREW_FORMULA := eugenetaranov/tap/ssmsearch
upgrade-local:
	@TAG=$${TAG:-$$(git tag --sort=-version:refname | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$$' | head -1)}; \
	if [ -z "$$TAG" ]; then echo "No release tag found"; exit 1; fi; \
	echo "Waiting for release run for $$TAG..."; \
	for i in $$(seq 1 30); do \
		RUN_ID=$$(gh run list --workflow release.yaml --branch $$TAG --limit 1 --json databaseId --jq '.[0].databaseId'); \
		[ -n "$$RUN_ID" ] && break; \
		sleep 2; \
	done; \
	if [ -z "$$RUN_ID" ]; then echo "No release run found for $$TAG"; exit 1; fi; \
	gh run watch $$RUN_ID --exit-status || exit 1; \
	if command -v brew >/dev/null 2>&1; then \
		brew update && \
		if brew list --formula ssmsearch >/dev/null 2>&1; then \
			brew upgrade $(BREW_FORMULA); \
		else \
			brew install $(BREW_FORMULA); \
		fi && \
		ssmsearch -v; \
	else \
		echo "brew not found, skipping install"; \
	fi

# GoReleaser: test release configuration without publishing
release-dry-run:
	goreleaser release --snapshot --clean --skip=publish

# GoReleaser: create snapshot release (for testing)
release-snapshot:
	goreleaser release --snapshot --clean

# GoReleaser: check configuration
release-check:
	goreleaser check
