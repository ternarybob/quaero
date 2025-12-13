# Multi-Language Test Project

A test fixture project demonstrating a multi-language codebase for assessment testing.

## Overview

This project combines Go backend services, Python automation scripts, and JavaScript web frontend to demonstrate a typical polyglot application architecture.

## Build Instructions

### Go Backend

```bash
go build -o bin/server ./main.go
```

### Python Scripts

```bash
cd scripts
python setup.py install
```

### JavaScript Frontend

```bash
cd web
npm install
npm run build
```

## Running the Project

### Start the Server

```bash
./bin/server
```

### Run Python Scripts

```bash
python scripts/helpers.py --task migrate
```

### Start Frontend Dev Server

```bash
cd web
npm start
```

## Testing

### Go Tests

```bash
go test ./...
```

### Python Tests

```bash
cd scripts
pytest
```

### JavaScript Tests

```bash
cd web
npm test
```

## Project Structure

- `main.go` - Go application entry point
- `pkg/` - Go utility packages
- `scripts/` - Python automation scripts
- `web/` - JavaScript/Node.js frontend
- `docs/` - Documentation

## Dependencies

- Go 1.21+
- Python 3.9+
- Node.js 18+
- Make (for build automation)
