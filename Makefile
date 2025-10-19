export PATH := /usr/local/go/bin:$(PATH)

.PHONY: build install

build:
	@echo "Building goliath-cli..."
	@mkdir -p dist
	go build -o dist/goliath-cli ./cmd/goliath-cli

install:
	@echo "Installing goliath-cli..."
	go install ./cmd/goliath-cli
