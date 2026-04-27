NAME = $(shell grep '^name =' Cargo.toml | head -n 1 | cut -d '"' -f 2)
RUST_TARGET ?= host

.PHONY: all build clean

all: build

build:
	@echo "Building $(NAME) for $(RUST_TARGET)..."
	@if [ "$(RUST_TARGET)" = "host" ] || [ -z "$(RUST_TARGET)" ]; then \
		cargo build --release; \
	else \
		cargo build --release --target $(RUST_TARGET); \
	fi

clean:
	rm -rf target
