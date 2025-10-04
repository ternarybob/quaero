# Local Deployment Configuration

This directory contains configuration files for local development and testing of Quaero.

## Files

- `quaero.toml` - Local development configuration

## Quick Start

### 1. Build the Application

```bash
# Windows
.\scripts\build.ps1

# Linux/Mac
./scripts/build.sh
```

### 2. Configure

Edit `quaero.toml` and configure your data sources:

```toml
[sources.confluence]
enabled = true
spaces = ["TEAM", "DOCS"]

[sources.jira]
enabled = true
projects = ["DATA", "ENG"]
```

### 3. Run Locally

```bash
# Windows
.\bin\quaero.exe serve -c deployments\local\quaero.toml

# Linux/Mac
./bin/quaero serve -c deployments/local/quaero.toml
```

Or use the deployment script (if available):

```bash
# Windows
.\scripts\deploy.ps1 -Target local

# With rebuild
.\scripts\deploy.ps1 -Target local -Build
```

### 4. Access Web Interface

Open your browser to:
```
http://localhost:8080
```

## Available Commands

### Serve
Start the web server and API:
```bash
quaero serve -c deployments/local/quaero.toml
```

### Collect
Run data collection from configured sources:
```bash
quaero collect -c deployments/local/quaero.toml
```

### Query
Execute a search query:
```bash
quaero query "search terms" -c deployments/local/quaero.toml
```

### Version
Display version information:
```bash
quaero version
```

## Configuration Notes

### Data Sources

Configure which sources to enable:

- **Confluence**: Wiki pages and documentation
- **Jira**: Issues and project data
- **GitHub**: Repositories and code
- **Slack**: Messages and channels (planned)
- **Linear**: Issues (planned)
- **Notion**: Pages and databases (planned)

### Storage

- **RavenDB**: Primary document storage
- **Filesystem**: Images and attachments

### LLM Integration

Configure Ollama for local LLM processing:

```toml
[llm.ollama]
url = "http://localhost:11434"
text_model = "qwen2.5:32b"
vision_model = "llama3.2-vision:11b"
```

## Development Workflow

### Testing Changes

1. Make code changes
2. Rebuild: `.\scripts\build.ps1`
3. Restart server: `.\scripts\deploy.ps1 -Restart`

### Running Tests

```bash
# Windows
.\test\run-tests.ps1 -Type all

# Unit tests only
.\test\run-tests.ps1 -Type unit
```

### Viewing Logs

Check console output or configure file logging:

```toml
[logging]
level = "debug"
format = "text"
output = "stdout"
```

## Troubleshooting

### Server Won't Start

- Check if port 8080 is already in use
- Verify config file path is correct
- Check for errors in console output

### Data Collection Fails

- Verify source credentials and URLs
- Check network connectivity
- Review logs for authentication errors

### Storage Issues

- Ensure RavenDB is running and accessible
- Verify filesystem paths exist and are writable
- Check disk space availability

## Directory Structure

```
deployments/local/
├── quaero.toml    # Local configuration
└── README.md      # This file
```

## See Also

- [Main README](../../README.md) - Full project documentation
- [CLAUDE.md](../../CLAUDE.md) - Developer documentation
- [Docker Deployment](../docker/README.md) - Docker deployment guide
