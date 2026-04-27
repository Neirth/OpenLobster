# OpenLobster Recursive Makefile (Shared Build Logic)

# Configuration
EMBED_DIR = apps/backend/internal/infrastructure/plugin/embedded_binaries
DIST_EXE  = dist/openlobster
WHITELIST = openlobster-ai-anthropic \
            openlobster-ai-ollama \
            openlobster-ai-openai \
            openlobster-audio-elevenlabs \
            openlobster-memory-gml \
            openlobster-memory-neo4j \
            openlobster-messages-discord \
            openlobster-messages-telegram \
            openlobster-messages-slack \
            openlobster-secrets-json \
            openlobster-secrets-openbao

DITTO_TYPES = ai messages memory secrets audio

# Targets: linux-amd64, linux-arm64, darwin-arm64 (Default: host)
TARGET ?= host

# Host detection
HOST_OS   := $(shell uname -s)
HOST_ARCH := $(shell uname -m)
HOST_RUST_TARGET := $(shell rustc -vV | grep host: | cut -d ' ' -f 2)

# Architecture Mappings
ifeq ($(TARGET),linux-amd64)
    GOOS=linux
    GOARCH=amd64
    RUST_TARGET=x86_64-unknown-linux-musl
    OS=Linux
else ifeq ($(TARGET),linux-arm64)
    GOOS=linux
    GOARCH=arm64
    RUST_TARGET=aarch64-unknown-linux-musl
    OS=Linux
else ifeq ($(TARGET),darwin-arm64)
    GOOS=darwin
    GOARCH=arm64
    RUST_TARGET=aarch64-apple-darwin
    OS=Darwin
else
    # Host Target
    GOOS=$(shell echo $(HOST_OS) | tr '[:upper:]' '[:lower:]' | sed 's/darwin/darwin/')
    OS=$(HOST_OS)
    GOARCH=$(shell echo $(HOST_ARCH) | sed 's/x86_64/amd64/' | sed 's/aarch64/arm64/')
    RUST_TARGET=$(HOST_RUST_TARGET)
endif

# Cross-compilation helpers for Rust (musl-cross)
ifeq ($(TARGET),linux-amd64)
    export CARGO_TARGET_X86_64_UNKNOWN_LINUX_MUSL_LINKER=x86_64-unknown-linux-musl-gcc
    export CC_x86_64_unknown_linux_musl=x86_64-unknown-linux-musl-gcc
else ifeq ($(TARGET),linux-arm64)
    export CARGO_TARGET_AARCH64_UNKNOWN_LINUX_MUSL_LINKER=aarch64-unknown-linux-musl-gcc
    export CC_aarch64_unknown_linux_musl=aarch64-unknown-linux-musl-gcc
endif

# Signing Configuration
ifeq ($(OS),Darwin)
    SIGN_ID ?= -
else
    # Placeholder for Linux signing
    BSIGN_OPTS ?= 
endif

# Export vars to sub-makes
export GOOS GOARCH RUST_TARGET

# Targets
.PHONY: all build-prod build-skeleton clean frontend backend plugins ditto prepare-prod prepare-skeleton lint format codegen sign release

all: build-prod

## --- High Level Targets ---

build-prod: frontend plugins sign-plugins prepare-prod backend sign-backend
	@echo "--- Production Build & Sign Complete ($(TARGET)): $(DIST_EXE) ---"

## --- Component Compilation (Delegated) ---

frontend:
	@$(MAKE) -C apps/frontend

plugins:
	@for p in $(WHITELIST); do \
		if [ -d "plugins/$$p" ]; then \
			$(MAKE) -C "plugins/$$p" build RUST_TARGET=$(RUST_TARGET) || exit 1; \
		elif [ -d "plugins/$$p-rust" ]; then \
			$(MAKE) -C "plugins/$$p-rust" build RUST_TARGET=$(RUST_TARGET) || exit 1; \
		fi; \
	done

ditto:
	@$(MAKE) -C plugins/openlobster-misc-ditto build

backend:
	@$(MAKE) -C apps/backend build-prod
	@if [ -f "dist/openlobster" ] && [ "dist/openlobster" != "$(DIST_EXE)" ]; then \
		mv dist/openlobster $(DIST_EXE); \
	fi

sign-backend:
	@echo "Signing backend binary for $(OS) (Target: $(TARGET))..."
	@if [ -f "$(DIST_EXE)" ]; then \
		if [ "$(OS)" = "Darwin" ]; then \
			if command -v codesign > /dev/null; then \
				codesign -s $(SIGN_ID) --force --deep --options runtime $(DIST_EXE); \
			fi; \
		else \
			echo "Linux binary signing placeholder for $(DIST_EXE)"; \
		fi; \
	fi

sign-plugins:
	@echo "Signing plugins for $(OS) (Target: $(TARGET))..."
	@for p in $(WHITELIST); do \
		src="plugins/$$p/target/$(RUST_TARGET)/release/$$p"; \
		if [ ! -f "$$src" ]; then src="plugins/$$p/target/release/$$p"; fi; \
		if [ -f "$$src" ]; then \
			if [ "$(OS)" = "Darwin" ]; then \
				if command -v codesign > /dev/null; then \
					codesign -s $(SIGN_ID) --force "$$src" || exit 1; \
				fi; \
			else \
				echo "Linux binary signing placeholder for $$p ($$src)"; \
			fi; \
		fi; \
	done

## --- Bundling Orchestration ---

prepare-prod:
	@echo "Bundling Real Plugins for $(RUST_TARGET)..."
	@mkdir -p $(EMBED_DIR)
	@rm -rf $(EMBED_DIR)/*
	@for p_id in $(WHITELIST); do \
		src="plugins/$$p_id/target/$(RUST_TARGET)/release/$$p_id"; \
		if [ ! -f "$$src" ]; then src="plugins/$$p_id/target/release/$$p_id"; fi; \
		if [ -f "$$src" ]; then \
			cp "$$src" "$(EMBED_DIR)/$$p_id"; \
		else \
			echo "Error: Binary for $$p_id not found at $$src"; \
			exit 1; \
		fi; \
	done

release:
	@for t in linux-amd64 linux-arm64 darwin-arm64; do \
		$(MAKE) build-prod TARGET=$$t DIST_EXE=dist/openlobster-$$t || exit 1; \
	done

clean:
	@$(MAKE) -C apps/frontend clean
	@$(MAKE) -C apps/backend clean
	@for d in plugins/*; do [ -f "$$d/Makefile" ] && $(MAKE) -C "$$d" clean || exit 1; done
	@rm -rf dist/
