.PHONY: build install

build:
	@echo "Building goliath-cli in Docker..."
	@mkdir -p dist
	docker build --target cli --output type=local,dest=./dist .

install:
	@echo "Installing goliath-cli to /usr/local/bin..."
	@test -f dist/goliath-cli || (echo "Error: dist/goliath-cli not found. Run 'make build' first." && exit 1)
	install -m 755 dist/goliath-cli /usr/local/bin/goliath-cli
