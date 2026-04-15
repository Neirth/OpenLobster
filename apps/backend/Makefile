# OpenLobster Backend Makefile

.PHONY: build clean

# Version from frontend package.json
VERSION := $(shell jq -r '.version' ../frontend/package.json)

build:
	@echo "Building Backend for $(GOOS)/$(GOARCH) (Static)..."
	@CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) \
		go build -v -ldflags="-s -w -extldflags '-static' -X main.version=$(VERSION)" \
		-o ../../dist/openlobster ./cmd/openlobster

clean:
	@rm -f ../../dist/openlobster
	@go clean
