# MCP Server Configuration Guide

This guide explains how to set up and use the Quaero MCP (Model Context Protocol) server with Claude CLI.

## Prerequisites

- **Quaero** installed and database populated with documents
- **Claude CLI** installed ([download here](https://claude.com/download))
- **Windows** operating system (current build)

## Building the MCP Server

The MCP server is built automatically with the main Quaero application:

```powershell
# Build both quaero.exe and quaero-mcp.exe
.\scripts\build.ps1

# Build and deploy to bin/quaero-mcp/ directory
.\scripts\build.ps1 -Deploy
```

This creates:
- `bin/quaero-mcp/quaero-mcp.exe` - MCP server executable
- `bin/quaero-mcp/quaero-mcp.toml` - Minimal MCP configuration
- `bin/quaero-mcp/README.md` - MCP-specific documentation
- `bin/README.md` - Project overview (in bin root)

The MCP server communicates via stdio/JSON-RPC.

## Claude CLI Configuration

### 1. Locate Your Claude Desktop Config

The configuration file is located at:

```
%APPDATA%\Claude\claude_desktop_config.json
```

Full path example: `C:\Users\YourUsername\AppData\Roaming\Claude\claude_desktop_config.json`

### 2. Add Quaero MCP Server

Edit `claude_desktop_config.json` to add the Quaero MCP server:

```json
{
  "mcpServers": {
    "quaero": {
      "command": "C:\\development\\quaero\\bin\\quaero-mcp\\quaero-mcp.exe",
      "args": [],
      "env": {
        "QUAERO_CONFIG": "C:\\development\\quaero\\bin\\quaero-mcp\\quaero-mcp.toml"
      }
    }
  }
}
```

**Important:**
- Use **double backslashes** (`\\`) in Windows paths for JSON
- MCP server uses `quaero-mcp.toml` (minimal config with only database + logging settings)
- **Alternative:** Use `bin\\quaero.toml` if you want shared config with main app
- Adjust paths to match your Quaero installation directory
- Database path in config must match the main Quaero database location

### 3. Restart Claude Desktop

After saving the configuration, restart Claude Desktop to load the MCP server.

## Available Tools

Once configured, Claude will have access to these Quaero search tools:

### 1. `search_documents`

Search the Quaero knowledge base using full-text search (SQLite FTS5).

**Parameters:**
- `query` (required): Search query using FTS5 syntax
  - Quoted phrases: `"exact phrase"`
  - Required terms: `+required`
  - Boolean operators: `OR`, `AND`
- `limit` (optional): Max results (default: 10, max: 100)
- `source_types` (optional): Filter by source types
  - Values: `["jira", "confluence", "github"]`

**Example:**
```
Search my knowledge base for "authentication bug" in jira issues
```

### 2. `get_document`

Retrieve a single document by its unique ID.

**Parameters:**
- `document_id` (required): Document ID in format `doc_{uuid}`

**Example:**
```
Get the full content of document doc_abc123def456
```

### 3. `list_recent_documents`

List recently updated documents, optionally filtered by source type.

**Parameters:**
- `limit` (optional): Max results (default: 20)
- `source_type` (optional): Filter by source type
  - Values: `"jira"`, `"confluence"`, `"github"`

**Example:**
```
List the 10 most recent confluence pages
```

### 4. `get_related_documents`

Find documents referencing a specific issue key or identifier.

**Parameters:**
- `reference` (required): Issue key (e.g., `BUG-123`) or identifier

**Example:**
```
Show me all documents that reference PROJ-456
```

## Example Queries

Here are some natural language queries you can use with Claude:

1. **Search for bugs:**
   ```
   Search for "login failure" bugs in Jira
   ```

2. **Find related documentation:**
   ```
   What documents reference issue KEY-789?
   ```

3. **Browse recent content:**
   ```
   Show me the 15 most recently updated documents
   ```

4. **Get specific document:**
   ```
   Get the full details of document doc_1234567890abcdef
   ```

## Response Format

All tools return results in **Markdown format** for easy reading in Claude Desktop:

### Search Results Format

```markdown
## Search Results for "query" (N results)

### 1. Document Title
**Source:** jira (PROJ-123)
**URL:** https://jira.example.com/browse/PROJ-123
**Updated:** 2025-11-09T15:30:00Z

#### Content:
[First 300 characters of content...]

**Metadata:** {structured JSON metadata}

---

### 2. Another Document
...
```

### Single Document Format

```markdown
# Document Title

**ID:** doc_1234567890abcdef
**Source:** confluence (SPACE-456)
**URL:** https://confluence.example.com/display/SPACE/Page
**Created:** 2025-10-01T10:00:00Z
**Updated:** 2025-11-09T15:30:00Z

## Content

[Full document content in markdown...]

## Metadata

```json
{
  "structured": "metadata"
}
```
```

## Troubleshooting

### MCP Server Not Appearing in Claude

1. **Check config file syntax:** Ensure JSON is valid (no trailing commas, proper escaping)
2. **Verify paths:** Use absolute paths with double backslashes
3. **Restart Claude Desktop:** Configuration is loaded at startup
4. **Check logs:** Look in Claude Desktop logs for MCP initialization errors

### Database Path Errors

If you see "database not found" errors:

1. **Verify QUAERO_CONFIG path:** Should point to `quaero.toml`
2. **Check database_path in quaero.toml:**
   ```toml
   [database]
   path = "./quaero.db"
   ```
3. **Ensure database exists:** Run Quaero once to create the database

### No Search Results

If searches return empty results:

1. **Populate the database:** Use Quaero's crawler to collect documents
2. **Check source type filters:** Remove filters to search all sources
3. **Verify FTS5 index:** The documents_fts table should exist in SQLite

### Performance Issues

If searches are slow:

1. **Reduce result limits:** Use smaller `limit` values (default: 10)
2. **Use specific queries:** More specific queries perform better
3. **Check database size:** Very large databases (>1GB) may be slow

## Advanced Configuration

### Custom Database Location

To use a different database:

```json
{
  "mcpServers": {
    "quaero": {
      "command": "C:\\quaero\\bin\\quaero-mcp.exe",
      "env": {
        "QUAERO_CONFIG": "C:\\path\\to\\custom\\quaero.toml"
      }
    }
  }
}
```

### Logging

The MCP server logs at **WARN level** by default to avoid cluttering stdio (JSON-RPC communication). To enable debug logging, modify `cmd/quaero-mcp/main.go`:

```go
logger := arbor.NewLogger().WithConsoleWriter(arbor_models.WriterConfiguration{
    ...
}).WithLevelFromString("debug") // Change from "warn" to "debug"
```

Rebuild with `.\scripts\build.ps1` after changing log level.

## Security Considerations

### Local-Only Access

The MCP server:
- Runs **locally** on your machine
- Communicates via **stdio** (no network exposure)
- Accesses the **local SQLite database**
- **No cloud API calls** (all data stays local)

### Sensitive Data

If your Quaero database contains sensitive information:
- Ensure proper file system permissions on `quaero.db`
- Be mindful when sharing Claude conversations (they may contain query results)
- Consider using separate databases for public/private content

## Next Steps

- **Populate your knowledge base:** Use Quaero's crawler to collect documents from Jira, Confluence, GitHub
- **Experiment with queries:** Try different search patterns and filters
- **Create workflows:** Use Claude to analyze patterns across your documents
- **Report issues:** File bugs or feature requests in the Quaero GitHub repository

## See Also

- [Quaero README](../../README.md) - Main application documentation
- [MCP Protocol Specification](https://github.com/mark3labs/mcp-go) - Model Context Protocol details
- [Claude CLI Documentation](https://claude.com/docs/cli) - Claude CLI features
