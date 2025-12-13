.PHONY: all build test clean install

# Build all components
all: build

# Build Go backend
build:
	@echo "Building Go backend..."
	go build -o bin/server ./main.go

# Run all tests
test:
	@echo "Running Go tests..."
	go test ./...
	@echo "Running Python tests..."
	cd scripts && pytest
	@echo "Running JavaScript tests..."
	cd web && npm test

# Clean build artifacts
clean:
	rm -rf bin/
	rm -rf web/dist/
	rm -rf scripts/__pycache__/

# Install dependencies
install:
	@echo "Installing Go dependencies..."
	go mod download
	@echo "Installing Python dependencies..."
	pip install -r scripts/requirements.txt
	@echo "Installing JavaScript dependencies..."
	cd web && npm install

# Run the server
run:
	./bin/server

# Format code
fmt:
	go fmt ./...
	black scripts/*.py
	cd web && npm run format
