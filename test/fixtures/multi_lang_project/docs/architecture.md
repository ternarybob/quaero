# Architecture Documentation

## System Overview

The Multi-Language Test Project is a polyglot application demonstrating modern web architecture patterns with multiple programming languages working together.

## Components

### 1. Go Backend (`main.go`, `pkg/`)

**Purpose**: HTTP server handling API requests and business logic

**Key Features**:
- RESTful API endpoints
- Request routing with Gorilla Mux
- Structured logging with Zerolog
- Utility functions for data processing

**Entry Point**: `main.go`

**Key Functions**:
- `homeHandler()` - Serves home page
- `healthHandler()` - Health check endpoint
- `processHandler()` - Main processing endpoint

### 2. Python Scripts (`scripts/`)

**Purpose**: Automation and administrative tasks

**Key Features**:
- Database migration utilities
- Data validation tools
- Batch processing capabilities
- CLI interface with argparse

**Entry Point**: `scripts/helpers.py`

**Key Functions**:
- `migrate()` - Database migration
- `validate()` - Data validation
- `process_batch()` - Batch processing

### 3. JavaScript Frontend (`web/`)

**Purpose**: Web frontend and API proxy

**Key Features**:
- Express.js server
- API proxy to backend
- Request validation
- Response formatting

**Entry Point**: `web/index.js`

**Key Functions**:
- Express middleware setup
- Backend API proxy
- Health check aggregation

## Data Flow

```
User Request
    ↓
JavaScript Frontend (port 3000)
    ↓ (validates & forwards)
Go Backend (port 8080)
    ↓ (processes)
Python Scripts (async tasks)
    ↓
Response
```

## Directory Structure

```
multi_lang_project/
├── main.go              # Go entry point
├── go.mod               # Go dependencies
├── Makefile             # Build automation
├── README.md            # Documentation
├── pkg/
│   └── utils.go         # Go utilities
├── scripts/
│   ├── setup.py         # Python package setup
│   └── helpers.py       # Python utilities
├── web/
│   ├── package.json     # Node.js dependencies
│   ├── index.js         # Frontend server
│   └── utils.js         # JS utilities
└── docs/
    └── architecture.md  # This file
```

## Build System

The project uses Make as the primary build orchestration tool:

- `make build` - Build all components
- `make test` - Run all tests
- `make install` - Install dependencies
- `make run` - Start the server

Each language has its own build tools:
- Go: `go build`
- Python: `setup.py`
- JavaScript: `npm`

## Dependencies

### External Dependencies

**Go**:
- gorilla/mux - HTTP routing
- rs/zerolog - Structured logging

**Python**:
- requests - HTTP client
- pyyaml - YAML parsing
- click - CLI framework

**JavaScript**:
- express - Web framework
- axios - HTTP client
- dotenv - Environment configuration

### Internal Dependencies

- Frontend depends on Backend API
- Backend can trigger Python scripts
- All components share configuration

## Deployment

### Development

1. Start backend: `go run main.go`
2. Start frontend: `cd web && npm start`
3. Run scripts: `python scripts/helpers.py --task migrate`

### Production

1. Build all: `make build`
2. Run tests: `make test`
3. Start server: `make run`

## Extension Points

1. **Add New API Endpoint**: Edit `main.go`, add handler function
2. **Add New Script**: Create new Python file in `scripts/`
3. **Add Frontend Route**: Edit `web/index.js`, add Express route
4. **Add Utility Function**: Add to `pkg/utils.go`, `scripts/helpers.py`, or `web/utils.js`

## Testing Strategy

- **Unit Tests**: Each language has its own test framework
- **Integration Tests**: Test API endpoints end-to-end
- **Build Tests**: Verify all components build successfully
