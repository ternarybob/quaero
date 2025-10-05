# Quaero

**Quaero** (Latin: "I seek, I search") - A knowledge collection system with web-based interface.

## Overview

Quaero collects documentation from Atlassian (Confluence, Jira) using browser extension authentication and provides a web-based interface for browsing and searching the data.

### Key Features

- 🔐 **Automatic Authentication** - Chrome extension captures credentials
- 📊 **Real-time Updates** - WebSocket-based live log streaming
- 💾 **SQLite Storage** - Local database with full-text search
- 🌐 **Web Interface** - Browser-based UI for collection and browsing
- ⚡ **Fast Collection** - Efficient scraping and storage

## Technology Stack

- **Language:** Go 1.25+
- **Storage:** SQLite with FTS5 (full-text search)
- **Web UI:** HTML templates, vanilla JavaScript, WebSockets
- **Authentication:** Chrome extension → WebSocket → HTTP service
- **Logging:** github.com/ternarybob/arbor (structured logging)
- **Configuration:** TOML via github.com/pelletier/go-toml/v2

## Quick Start

### Prerequisites

- Go 1.25+
- Chrome browser
- SQLite support

### Installation

```bash
# Clone the repository
git clone https://github.com/ternarybob/quaero.git
cd quaero

# Build
./scripts/build.ps1

# Or use Go directly
go build -o bin/quaero ./cmd/quaero
```

### Configuration

Create `quaero.toml` in your project directory:

```toml
[server]
host = "localhost"
port = 8080

[logging]
level = "info"
format = "json"

[storage]
type = "sqlite"

[storage.sqlite]
path = "./quaero.db"
enable_fts5 = true
enable_wal = true
```

### Running the Server

```bash
# Start the server
./bin/quaero serve

# Or with custom config
./bin/quaero serve --config /path/to/quaero.toml --port 8080
```

### Installing Chrome Extension

1. Open Chrome and navigate to `chrome://extensions/`
2. Enable "Developer mode" (top right)
3. Click "Load unpacked"
4. Select the `cmd/quaero-chrome-extension/` directory

### Using Quaero

1. **Start the server:**
   ```bash
   ./bin/quaero serve
   ```

2. **Navigate to Atlassian:**
   - Go to your Confluence or Jira instance
   - Log in normally (handles 2FA, SSO, etc.)

3. **Capture Authentication:**
   - Click the Quaero extension icon
   - Click "Send to Quaero"
   - Extension sends credentials to server

4. **Access Web UI:**
   - Open http://localhost:8080
   - Click "Confluence" or "Jira"
   - Click "Collect" to start gathering data

5. **Browse Data:**
   - View collected spaces/projects
   - Browse pages/issues
   - Real-time log updates

## Project Structure

```
quaero/
├── cmd/
│   ├── quaero/                      # Main application
│   └── quaero-chrome-extension/     # Chrome extension
├── internal/
│   ├── common/                      # Utilities (config, logging, banner)
│   ├── app/                         # Application orchestration
│   ├── services/atlassian/          # Jira & Confluence services
│   ├── handlers/                    # HTTP & WebSocket handlers
│   ├── storage/sqlite/              # SQLite storage layer
│   ├── server/                      # HTTP server
│   ├── interfaces/                  # Service interfaces
│   └── models/                      # Data models
├── pages/                           # Web UI templates
├── test/                            # Tests
├── scripts/                         # Build scripts
└── docs/                            # Documentation
```

## Commands

### Server

```bash
# Start server
quaero serve

# With custom port
quaero serve --port 8080

# With custom config
quaero serve --config /path/to/quaero.toml
```

### Version

```bash
# Show version
quaero version
```

## Architecture

### Authentication Flow

```
1. User logs into Atlassian
   ↓
2. Extension captures cookies/tokens
   ↓
3. Extension connects to ws://localhost:8080/ws
   ↓
4. Extension sends auth data
   ↓
5. Server stores credentials
   ↓
6. Collectors use credentials for API calls
```

### Collection Flow

```
1. User clicks "Collect" in Web UI
   ↓
2. Handler triggers service
   ↓
3. Service loads auth from database
   ↓
4. Service fetches data from Atlassian API
   ↓
5. Service stores in SQLite
   ↓
6. Service streams logs via WebSocket
   ↓
7. UI updates in real-time
```

## Web UI

### Dashboard (/)
- System status
- Authentication status
- Quick links

### Confluence (/confluence)
- Space browser
- Collection trigger
- Real-time logs

### Jira (/jira)
- Project browser
- Collection trigger
- Real-time logs

## API Endpoints

### HTTP Endpoints

```
GET  /                          - Dashboard
GET  /confluence                - Confluence UI
GET  /jira                      - Jira UI

POST /api/collect/jira          - Trigger Jira collection
POST /api/collect/confluence    - Trigger Confluence collection

GET  /api/data/jira/projects    - Get Jira projects
GET  /api/data/jira/issues      - Get Jira issues
GET  /api/data/confluence/spaces - Get Confluence spaces
GET  /api/data/confluence/pages - Get Confluence pages

GET  /health                    - Health check
```

### WebSocket

```
WS   /ws                        - Real-time updates
```

## Development

### Building

```bash
# Development build
./scripts/build.ps1

# Production build
./scripts/build.ps1 -Release

# Clean build
./scripts/build.ps1 -Clean
```

### Testing

```bash
# Run all tests
./test/run-tests.ps1 -Type all

# Unit tests only
./test/run-tests.ps1 -Type unit

# Integration tests only
./test/run-tests.ps1 -Type integration
```

### Code Quality

See [CLAUDE.md](CLAUDE.md) for:
- Agent-based development system
- Code quality standards
- Architecture patterns
- Testing requirements

## Configuration

### Priority Order

1. **CLI Flags** (highest)
2. **Environment Variables**
3. **Config File** (quaero.toml)
4. **Defaults** (lowest)

### Environment Variables

```bash
QUAERO_PORT=8080
QUAERO_HOST=localhost
QUAERO_LOG_LEVEL=info
```

### Configuration File

```toml
[server]
host = "localhost"
port = 8080

[logging]
level = "info"
format = "json"

[storage]
type = "sqlite"

[storage.sqlite]
path = "./quaero.db"
enable_fts5 = true
enable_wal = true
cache_size_mb = 100
```

## Troubleshooting

### Server won't start

```bash
# Check port availability
netstat -an | grep 8080

# Try different port
./bin/quaero serve --port 8081
```

### Extension not connecting

1. Check server is running: http://localhost:8080/health
2. Check extension permissions in Chrome
3. Reload extension
4. Check browser console for errors

### Collection fails

1. Verify authentication in extension
2. Check server logs
3. Verify Atlassian instance URL
4. Check network connectivity

## Documentation

- [Architecture](docs/architecture.md) - System architecture and design
- [Dependency Injection](docs/dependency-injection.md) - Constructor-based DI pattern
- [Requirements](docs/requirements.md) - Current requirements
- [Remaining Requirements](docs/remaining-requirements.md) - Future work
- [CLAUDE.md](CLAUDE.md) - Development standards

## Current Limitations

- ✅ Confluence and Jira only (no GitHub, Slack, etc.)
- ✅ No natural language query (coming soon)
- ✅ No vector embeddings yet (planned)
- ✅ Web UI only (no CLI collection commands)
- ✅ Single-user only (no multi-user support)

## Roadmap

See [docs/remaining-requirements.md](docs/remaining-requirements.md) for detailed roadmap.

**Near Term:**
- [ ] Vector embeddings (sqlite-vec)
- [ ] Natural language query interface
- [ ] RAG pipeline integration

**Future:**
- [ ] GitHub collector
- [ ] Additional data sources (Slack, Linear)
- [ ] Multi-user support
- [ ] Cloud deployment option

## Contributing

See [CLAUDE.md](CLAUDE.md) for development guidelines and agent-based workflow.

## License

MIT

---

**Quaero: I seek knowledge. 🔍**
