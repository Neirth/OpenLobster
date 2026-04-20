NAME = $(shell grep '^name =' Cargo.toml | head -n 1 | cut -d '"' -f 2)
RUST_TARGET ?= host
IS_CROSS ?= 0

.PHONY: all build clean

all: build

build:
	@echo "Building $(NAME) for $(RUST_TARGET) (Cross: $(IS_CROSS))..."
	@if [ "$(IS_CROSS)" = "0" ]; then \
		cargo build --release; \
	else \
		cargo zigbuild --target $(RUST_TARGET) --release; \
	fi

clean:
	rm -rf target
