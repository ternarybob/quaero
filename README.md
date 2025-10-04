# Quaero

**Quaero** (Latin: "I seek, I search") - A local knowledge base system with natural language query capabilities.

## Overview

Quaero is a self-contained knowledge base system that:
- Collects documentation from multiple sources (Confluence, Jira, GitHub, Slack, Linear, etc.)
- Processes and stores content with full-text and vector search
- Provides natural language query interface using local LLMs (Ollama)
- Runs completely offline on a single machine
- Uses browser extension for seamless authentication

## Technology Stack

- **Language:** Go 1.21+
- **Storage:** RavenDB (document store with vector search)
- **LLM:** Ollama (Qwen2.5-32B for text, Llama3.2-Vision-11B for images)
- **Browser Automation:** rod (for web scraping)
- **Authentication:** Chrome extension → HTTP service
- **Testing:** Go testing + testify

## Installation

```bash
# Clone the repository
git clone https://github.com/ternarybob/quaero.git
cd quaero

# Build
make build

# Or install directly
go install ./cmd/quaero
```

## Usage

### Start the server

```bash
quaero serve
```

This starts the HTTP server that receives authentication from the browser extension.

### Collect data

```bash
# Collect from all sources
quaero collect --all

# Collect from specific source
quaero collect --source confluence
quaero collect --source jira
```

### Query

```bash
quaero query "How to onboard a new user?"
quaero query "Show me the data architecture" --images
```

## Configuration

Copy `config.yaml.example` to `config.yaml` and configure your sources:

```yaml
sources:
  confluence:
    enabled: true
    spaces: ["TEAM", "DOCS"]

  jira:
    enabled: true
    projects: ["DATA", "ENG"]

  github:
    enabled: true
    token: "${GITHUB_TOKEN}"
    repos:
      - "your-org/repo1"
```

## Authentication

Quaero uses a browser extension to capture authentication credentials automatically. Install the [Quaero Authentication Extension](https://github.com/ternarybob/quaero-auth-extension) to seamlessly authenticate with Confluence, Jira, and other sources.

## Architecture

See [docs/architecture.md](docs/architecture.md) for detailed architecture documentation.

## Development

```bash
# Run tests
make test

# Run integration tests
make test-integration

# Build
make build
```

## Related Repositories

- [quaero-auth-extension](https://github.com/ternarybob/quaero-auth-extension) - Chrome extension for authentication
- [quaero-docs](https://github.com/ternarybob/quaero-docs) - Documentation site

## License

MIT
