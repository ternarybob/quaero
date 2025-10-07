# Quaero MCP Client

This directory contains the MCP (Model Context Protocol) client proxy for Quaero.

## Overview

Quaero provides an HTTP-based MCP server at `http://localhost:8085/mcp`. Most MCP clients (like LM Studio and Claude Desktop) expect stdio-based communication. This proxy bridges the gap.

## Quick Setup

### Automatic Configuration (Recommended)

Build Quaero - the MCP configurations are automatically generated:

```bash
# Build and deploy
./scripts/build.ps1
```

The build process creates ready-to-use configuration files in `bin/mcp-client/`:
- `lmstudio-config.json` - For LM Studio
- `claude-desktop-config.json` - For Claude Desktop

Simply copy the contents of the appropriate file to your MCP client configuration.

### Manual Configuration

#### LM Studio

Add to LM Studio's MCP configuration (Developer → Model Context Protocol):

```json
{
  "mcpServers": {
    "quaero": {
      "command": "node",
      "args": ["<PATH_TO_QUAERO>/bin/mcp-client/proxy.js"],
      "env": {
        "QUAERO_URL": "http://localhost:8085"
      }
    }
  }
}
```

Replace `<PATH_TO_QUAERO>` with your actual path, e.g.:
- Windows: `C:/development/quaero`
- Linux: `/home/user/quaero`
- Mac: `/Users/username/quaero`

#### Claude Desktop

Add to `%APPDATA%\Claude\claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "quaero": {
      "command": "node",
      "args": ["<PATH_TO_QUAERO>/bin/mcp-client/proxy.js"],
      "env": {
        "QUAERO_URL": "http://localhost:8085"
      }
    }
  }
}
```

## Available Resources

- `quaero://documents/all` - All documents (Jira + Confluence + GitHub)
- `quaero://documents/jira` - Jira issues only
- `quaero://documents/confluence` - Confluence pages only
- `quaero://documents/github` - GitHub documents only
- `quaero://documents/stats` - Document statistics

## Available Tools

### search_documents
Search documents using full-text search.

**Parameters:**
- `query` (string, required): Search query
- `limit` (number, optional): Maximum results (default: 10)

**Example:**
```
Use the search_documents tool to find all documents mentioning "authentication"
```

### get_document
Retrieve a specific document by ID.

**Parameters:**
- `id` (string, required): Document ID

**Example:**
```
Get document doc_46d98640-0b0f-4c39-b88c-7d7ed5b005da
```

### list_documents
List documents with pagination and filtering.

**Parameters:**
- `source` (string, optional): Filter by source type (jira, confluence, github)
- `limit` (number, optional): Number of results (default: 50)
- `offset` (number, optional): Pagination offset (default: 0)

**Example:**
```
List the first 10 Confluence pages
```

## Testing

### Test the proxy directly:
```bash
# Send a test request via stdin
echo '{"jsonrpc":"2.0","id":1,"method":"resources/list","params":{}}' | node proxy.js
```

### Test with debug output:
```bash
DEBUG=true echo '{"jsonrpc":"2.0","id":1,"method":"resources/list","params":{}}' | node proxy.js
```

### Test the HTTP endpoint directly:
```bash
curl -X POST http://localhost:8085/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"resources/list","params":{}}'
```

## Prerequisites

1. **Quaero server must be running**: `./bin/quaero.exe serve`
2. **Node.js installed**: The proxy requires Node.js
3. **Port 8085 accessible**: Default port for Quaero server

## Troubleshooting

### "Cannot find module" error
Make sure the path in your MCP configuration matches the actual location of `proxy.js`.

### "Connection closed" error
Ensure the Quaero server is running:
```bash
cd C:/development/quaero/bin
./quaero.exe serve
```

### "ECONNREFUSED" error
Check that:
- Quaero is running on port 8085
- `QUAERO_URL` environment variable is correct
- No firewall blocking localhost connections

### Enable debug logging
Set `DEBUG=true` in the environment variables:
```json
{
  "env": {
    "QUAERO_URL": "http://localhost:8085",
    "DEBUG": "true"
  }
}
```

Check LM Studio Developer Logs for debug output.

## Architecture

```
┌─────────────┐         stdio (JSON-RPC)         ┌──────────┐
│  LM Studio  │ ◄──────────────────────────────► │ proxy.js │
│   / Claude  │                                   └──────────┘
└─────────────┘                                        │
                                                       │ HTTP POST
                                                       │ /mcp
                                                       ▼
                                               ┌───────────────┐
                                               │    Quaero     │
                                               │  MCP Server   │
                                               │ (port 8085)   │
                                               └───────────────┘
                                                       │
                                                       ▼
                                               ┌───────────────┐
                                               │   Documents   │
                                               │   (SQLite)    │
                                               └───────────────┘
```

## Document Statistics

Current knowledge base contains:
- 256 total documents
- 200 Jira issues
- 56 Confluence pages
- 127 documents with embeddings
