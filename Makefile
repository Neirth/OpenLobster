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
            openlobster-messages-twilio \
            openlobster-messages-whatsapp \
            openlobster-secrets-json \
            openlobster-secrets-openbao

DITTO_TYPES = ai messages memory secrets audio

# Targets: linux-amd64, linux-arm64, macos-arm64 (Default: host)
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
else ifeq ($(TARGET),macos-arm64)
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

# Decide if we are cross-compiling
IS_CROSS = 0
ifeq ($(OS),Linux)
    # On Linux, musl targets for the same arch are native (if musl-tools is installed)
    ifneq ($(RUST_TARGET),$(HOST_RUST_TARGET))
        # It's cross if arch is different or if it's macOS
        IS_CROSS = 1
    endif
else ifneq ($(RUST_TARGET),$(HOST_RUST_TARGET))
    IS_CROSS = 1
endif

# Signing Configuration
ifeq ($(OS),Darwin)
    SIGN_ID ?= -
else
    # Placeholder for Linux signing
    BSIGN_OPTS ?= 
endif

# Export vars to sub-makes
export GOOS GOARCH RUST_TARGET IS_CROSS

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
			$(MAKE) -C "plugins/$$p" build RUST_TARGET=$(RUST_TARGET) IS_CROSS=$(IS_CROSS) || exit 1; \
		elif [ -d "plugins/$$p-rust" ]; then \
			$(MAKE) -C "plugins/$$p-rust" build RUST_TARGET=$(RUST_TARGET) IS_CROSS=$(IS_CROSS) || exit 1; \
		fi; \
	done

ditto:
	@$(MAKE) -C plugins/openlobster-misc-ditto build

backend:
	@$(MAKE) -C apps/backend build-prod

sign-backend:
	@echo "Signing backend binary for $(OS) (Target: $(TARGET))..."
	@if [ -f "$(DIST_EXE)" ]; then \
		if [ "$(OS)" = "Darwin" ]; then \
			if command -v codesign > /dev/null; then \
				codesign -s $(SIGN_ID) --force --deep --options runtime $(DIST_EXE); \
			else \
				rcodesign sign --p12-file "$(APPLE_CERTIFICATE_P12_PATH)" --p12-password "$(APPLE_CERTIFICATE_PASSWORD)" $(DIST_EXE); \
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
					codesign -s $(SIGN_ID) --force "$$src"; \
				else \
					rcodesign sign --p12-file "$(APPLE_CERTIFICATE_P12_PATH)" --p12-password "$(APPLE_CERTIFICATE_PASSWORD)" "$$src"; \
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
			echo "Warning: Binary for $$p_id not found at $$src"; \
		fi; \
	done

release:
	@for t in linux-amd64 linux-arm64 macos-arm64; do \
		$(MAKE) build-prod TARGET=$$t DIST_EXE=dist/openlobster-$$t; \
	done

clean:
	@$(MAKE) -C apps/frontend clean
	@$(MAKE) -C apps/backend clean
	@for d in plugins/*; do [ -f "$$d/Makefile" ] && $(MAKE) -C "$$d" clean; done
	@rm -rf dist/
