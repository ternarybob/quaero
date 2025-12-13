# Quaero MCP Server

Model Context Protocol (MCP) server that exposes Quaero's search capabilities to AI assistants like Claude Desktop.

## Quick Start

This directory contains the MCP server binary and documentation for integrating Quaero with Claude Desktop and other MCP-compatible clients.

### Configuration

Add to your Claude Desktop config (`%APPDATA%\Claude\claude_desktop_config.json`):

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

**Configuration Files:**
- `quaero-mcp.toml` - Minimal config (database + logging only)
- Alternative: Use `../quaero.toml` to share config with main app

**Note:** Adjust paths to match your installation directory.

## Available Tools

The MCP server provides 4 tools for searching and retrieving documents:

### 1. search_documents
Full-text search using SQLite FTS5.

**Parameters:**
- `query` (string, required) - Search query (supports FTS5 syntax)
- `limit` (integer, optional) - Maximum results to return (default: 10)
- `source_types` (array, optional) - Filter by source types (jira, confluence, github, etc.)

**Example:**
```
"Find all documents about authentication in Jira"
→ Uses: search_documents(query="authentication", source_types=["jira"])
```

### 2. get_document
Retrieve complete document by ID.

**Parameters:**
- `document_id` (string, required) - Document ID (format: doc_{uuid})

**Example:**
```
"Show me the full content of document doc_abc123"
→ Uses: get_document(document_id="doc_abc123")
```

### 3. list_recent_documents
List recently updated documents.

**Parameters:**
- `limit` (integer, optional) - Maximum results to return (default: 10)
- `source_type` (string, optional) - Filter by single source type

**Example:**
```
"What are the 5 most recent Confluence pages?"
→ Uses: list_recent_documents(limit=5, source_type="confluence")
```

### 4. get_related_documents
Find documents by cross-reference (issue keys, project IDs, etc.).

**Parameters:**
- `reference` (string, required) - Reference identifier (e.g., BUG-123, PROJ-456)
- `limit` (integer, optional) - Maximum results to return (default: 10)

**Example:**
```
"Find all documents related to PROJ-456"
→ Uses: get_related_documents(reference="PROJ-456")
```

## Response Format

All tools return Markdown-formatted results with:
- Document metadata (title, source type, URL)
- Content preview or full content
- Timestamps (created, updated)
- Relevance scores (for search results)

## Documentation

For detailed setup and usage:
- **Setup Guide:** `../../docs/implement-mcp-server/mcp-configuration.md`
- **Usage Examples:** `../../docs/implement-mcp-server/usage-examples.md`
- **Architecture:** See `../../CLAUDE.md` - MCP Server Architecture section
- **Project README:** `../README.md` (in bin directory)

## Version

This MCP server uses the same version as the main Quaero application. Check `../../.version` file in the project root for current version and build information.

## Troubleshooting

**MCP server not appearing in Claude Desktop:**
1. Check config path is correct (absolute paths required)
2. Verify `quaero-mcp.exe` exists in this directory
3. Restart Claude Desktop after config changes
4. Check Claude Desktop logs for errors

**Search returns no results:**
1. Verify Quaero database has documents (run main app first)
2. Check database path in `quaero.toml`
3. Ensure documents are embedded (scheduler runs every 5 minutes)

**Connection errors:**
1. Check `QUAERO_CONFIG` environment variable points to valid config
2. Verify database file is accessible
3. Check file permissions

## Development

Built automatically with main Quaero application:

```powershell
# From project root directory
cd ../..

# Build both quaero.exe and quaero-mcp.exe
.\scripts\build.ps1

# Build and deploy to bin/quaero-mcp/
.\scripts\build.ps1 -Deploy
```

For development guidelines, see `../../CLAUDE.md` - "Working with MCP Server" section.
