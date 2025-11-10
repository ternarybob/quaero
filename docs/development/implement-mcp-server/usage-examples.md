# MCP Server Usage Examples

This document provides real-world examples of using the Quaero MCP server with Claude Desktop. All examples assume you have configured Claude Desktop to use the Quaero MCP server (see [mcp-configuration.md](mcp-configuration.md)).

## Quick Reference

| Tool | Purpose | Primary Use Case |
|------|---------|------------------|
| `search_documents` | Full-text search | Finding documents by keywords |
| `get_document` | Retrieve by ID | Getting complete document content |
| `list_recent_documents` | List by recency | Seeing recent activity |
| `get_related_documents` | Find cross-references | Tracking issue relationships |

## Tool 1: search_documents

### Example 1: Basic Keyword Search

**User Query:**
```
Search my knowledge base for documents about authentication
```

**Claude's Tool Call:**
```json
{
  "tool": "search_documents",
  "parameters": {
    "query": "authentication",
    "limit": 10
  }
}
```

**Response Format:**
```markdown
## Search Results for "authentication" (7 results)

### 1. Implement JWT Authentication
**Source:** jira (SEC-401)
**URL:** https://jira.company.com/browse/SEC-401
**Updated:** 2025-11-08T14:23:00Z

#### Content:
We need to implement JWT-based authentication for the API. Current session-based auth
doesn't work well with mobile clients. Requirements: RS256 signing, 15-minute expiry,
refresh token support...

**Metadata:** {"priority": "high", "status": "in_progress", "assignee": "alice@company.com"}

---

### 2. Authentication Service Documentation
**Source:** confluence (WIKI-789)
**URL:** https://confluence.company.com/display/DOCS/Auth
**Updated:** 2025-11-05T09:15:00Z

#### Content:
# Authentication Service

Our authentication service provides OAuth2 and JWT token management...
```

**Use Cases:**
- Finding all documents related to a topic
- Discovering relevant context before starting work
- Building understanding of a feature area

---

### Example 2: Advanced Search with Filters

**User Query:**
```
Find Jira issues about "database migration" that are high priority
```

**Claude's Tool Call:**
```json
{
  "tool": "search_documents",
  "parameters": {
    "query": "\"database migration\" +high",
    "source_types": ["jira"],
    "limit": 5
  }
}
```

**FTS5 Query Features Used:**
- `"database migration"` - Exact phrase match
- `+high` - Required term (must contain "high")
- `source_types: ["jira"]` - Only search Jira issues

**Response:**
Returns only Jira issues containing the exact phrase "database migration" AND the word "high".

---

### Example 3: Multi-Source Search

**User Query:**
```
Show me everything about the payment gateway in Jira and Confluence
```

**Claude's Tool Call:**
```json
{
  "tool": "search_documents",
  "parameters": {
    "query": "payment gateway",
    "source_types": ["jira", "confluence"],
    "limit": 20
  }
}
```

**Result:**
Returns issues, documentation, and discussion from both Jira and Confluence about the payment gateway.

**Use Cases:**
- Getting comprehensive context across multiple tools
- Finding both issues and documentation
- Cross-referencing work items with docs

---

## Tool 2: get_document

### Example 4: Retrieve Specific Document

**User Query:**
```
Get the full content of document doc_a1b2c3d4e5f6g7h8
```

**Claude's Tool Call:**
```json
{
  "tool": "get_document",
  "parameters": {
    "document_id": "doc_a1b2c3d4e5f6g7h8"
  }
}
```

**Response Format:**
```markdown
# API Authentication Implementation Guide

**ID:** doc_a1b2c3d4e5f6g7h8
**Source:** confluence (DOCS-401)
**URL:** https://confluence.company.com/display/API/Auth
**Created:** 2025-10-15T10:00:00Z
**Updated:** 2025-11-09T08:30:00Z

## Content

### Overview
This guide covers the implementation of API authentication using JWT tokens...

### Prerequisites
- Node.js 18 or higher
- Redis for token storage
- Access to the auth service repository

### Implementation Steps

1. Install dependencies:
   ```bash
   npm install jsonwebtoken redis
   ```

2. Configure JWT settings...
[Full document content continues...]

## Metadata

```json
{
  "space": "API",
  "author": "alice@company.com",
  "labels": ["authentication", "api", "security"],
  "version": 5
}
```
```

**Use Cases:**
- Reading complete documentation pages
- Getting full context of a specific issue
- Following up on search results with full content

---

### Example 5: Multi-Step Workflow

**User Query:**
```
Search for "login bug" and show me the full content of the top result
```

**Claude's Response:**
```
Let me search for "login bug" first...

[Calls search_documents with query="login bug"]

I found 3 results. The top result is "Login fails on Safari 17" (BUG-523).
Let me get the full details...

[Calls get_document with document_id="doc_x9y8z7..."]

Here's the complete issue:
[Full document content displayed]
```

**Use Cases:**
- Automated research workflows
- Drilling down from search to details
- Building comprehensive answers from multiple sources

---

## Tool 3: list_recent_documents

### Example 6: See Recent Activity

**User Query:**
```
What are the 5 most recently updated documents?
```

**Claude's Tool Call:**
```json
{
  "tool": "list_recent_documents",
  "parameters": {
    "limit": 5
  }
}
```

**Response Format:**
```markdown
## Recently Updated Documents (5 results)

### 1. Q4 Performance Review
**Source:** confluence (HR-101)
**URL:** https://confluence.company.com/display/HR/Q4-Review
**Updated:** 2025-11-09T15:42:00Z

#### Content:
Q4 performance metrics and team achievements...

---

### 2. Fix critical security vulnerability
**Source:** jira (SEC-789)
**URL:** https://jira.company.com/browse/SEC-789
**Updated:** 2025-11-09T15:30:00Z

#### Content:
SQL injection vulnerability discovered in user profile endpoint...

[Additional 3 documents...]
```

**Use Cases:**
- Daily standup preparation (what changed overnight?)
- Monitoring project activity
- Staying updated on team work

---

### Example 7: Recent Activity by Source Type

**User Query:**
```
Show me the 10 most recent Confluence pages
```

**Claude's Tool Call:**
```json
{
  "tool": "list_recent_documents",
  "parameters": {
    "limit": 10,
    "source_type": "confluence"
  }
}
```

**Result:**
Returns only Confluence pages, sorted by updated_at descending.

**Use Cases:**
- Checking recent documentation updates
- Finding newly created pages
- Reviewing knowledge base changes

---

## Tool 4: get_related_documents

### Example 8: Find Cross-References

**User Query:**
```
What documents reference issue BUG-401?
```

**Claude's Tool Call:**
```json
{
  "tool": "get_related_documents",
  "parameters": {
    "reference": "BUG-401"
  }
}
```

**Response Format:**
```markdown
## Documents Referencing "BUG-401" (4 results)

### 1. Release Notes v2.3.0
**Source:** confluence (RELEASE-230)
**URL:** https://confluence.company.com/display/RELEASES/v2.3.0
**Updated:** 2025-11-08T12:00:00Z

#### Content:
# Release v2.3.0

Bug Fixes:
- Fixed login timeout issue (BUG-401)
- Resolved payment gateway retry logic (BUG-387)
...

---

### 2. Authentication Refactoring Epic
**Source:** jira (EPIC-55)
**URL:** https://jira.company.com/browse/EPIC-55
**Updated:** 2025-11-07T16:30:00Z

#### Content:
Related issues: BUG-401, BUG-412, TASK-789
This epic addresses authentication issues reported in Q3...
```

**Use Cases:**
- Finding related work items
- Tracking issue dependencies
- Understanding context and impact

---

### Example 9: Project Cross-References

**User Query:**
```
Show me all documents that mention PROJECT-123
```

**Claude's Tool Call:**
```json
{
  "tool": "get_related_documents",
  "parameters": {
    "reference": "PROJECT-123"
  }
}
```

**Result:**
Returns all documents (Jira issues, Confluence pages, GitHub PRs) that reference PROJECT-123.

**Use Cases:**
- Tracking epic/project references
- Finding related documentation
- Discovering cross-team dependencies

---

## Complex Workflows

### Workflow 1: Comprehensive Research

**User Query:**
```
I need to understand our authentication system. Search for auth docs,
show me recent changes, and find related Jira issues.
```

**Claude's Multi-Step Response:**

1. **Search for documentation:**
   ```
   [Calls search_documents with query="authentication system", source_types=["confluence"]]
   ```

2. **Get recent updates:**
   ```
   [Calls list_recent_documents with source_type="confluence", limit=5]
   ```

3. **Find related issues:**
   ```
   [Calls search_documents with query="authentication", source_types=["jira"], limit=10]
   ```

4. **Synthesize findings:**
   ```
   Based on the search results, here's what I found about your authentication system:

   Documentation:
   - Main auth guide (WIKI-401) covers JWT implementation
   - OAuth2 integration documented in WIKI-523

   Recent Changes:
   - Auth service docs updated 2 days ago (security improvements)
   - New SSO guide published yesterday

   Related Issues:
   - 3 high-priority auth bugs currently in progress
   - 2 enhancement requests for multi-factor auth

   Would you like me to get the full content of any specific document?
   ```

---

### Workflow 2: Bug Investigation

**User Query:**
```
Find the bug report for login failures, get its full details,
and show me what other documents reference it.
```

**Claude's Multi-Step Response:**

1. **Search for bug:**
   ```
   [Calls search_documents with query="login failure", source_types=["jira"]]
   ```
   Found: BUG-789 "Login timeout on Safari 17"

2. **Get full bug details:**
   ```
   [Calls get_document with document_id="doc_..."]
   ```

3. **Find related documents:**
   ```
   [Calls get_related_documents with reference="BUG-789"]
   ```

4. **Provide comprehensive analysis:**
   ```
   Here's what I found about the login failure issue:

   Bug Report (BUG-789):
   - Status: In Progress
   - Priority: High
   - Affects: Safari 17 users
   - Root cause: Session cookie timeout mismatch

   Related Documents:
   - EPIC-55: Authentication refactoring (includes this fix)
   - Release v2.3.0: Scheduled to include the fix
   - WIKI-401: Auth documentation updated to reflect changes

   The issue is being addressed as part of a larger auth refactor
   and is scheduled for release v2.3.0.
   ```

---

### Workflow 3: New Developer Onboarding

**User Query:**
```
I'm new to the team. Show me recent Confluence pages and
the most important Jira epics.
```

**Claude's Multi-Step Response:**

1. **Get recent documentation:**
   ```
   [Calls list_recent_documents with source_type="confluence", limit=10]
   ```

2. **Search for epics:**
   ```
   [Calls search_documents with query="epic", source_types=["jira"], limit=5]
   ```

3. **Provide onboarding guide:**
   ```
   Welcome to the team! Here's what you should read:

   Recent Documentation Updates:
   1. API Authentication Guide (updated yesterday)
   2. Developer Setup Instructions (updated 3 days ago)
   3. Architecture Overview (updated last week)

   Current Major Epics:
   1. EPIC-55: Authentication Refactoring (in progress)
   2. EPIC-61: Payment Gateway v2 (planning)
   3. EPIC-49: Mobile App Rewrite (completed)

   I recommend starting with the Developer Setup Instructions.
   Would you like me to get the full content of any of these documents?
   ```

---

## Tips for Effective Queries

### 1. Use Specific Keywords
- ❌ "Show me stuff about bugs"
- ✅ "Search for authentication bugs in Jira"

### 2. Combine Tools for Better Results
- Search first to discover documents
- Use get_document for full details
- Use get_related_documents to find connections

### 3. Filter by Source Type
- Use `source_types` to narrow searches
- Jira for bugs/tasks, Confluence for docs, GitHub for code

### 4. Use FTS5 Query Syntax
- `"exact phrase"` - Match exact phrases
- `+required` - Must contain term
- `term1 OR term2` - Boolean operators
- `-excluded` - Exclude terms

### 5. Adjust Result Limits
- Default limit is usually sufficient (10-20)
- Increase for comprehensive research
- Decrease for quick checks

---

## Common Use Cases

### Daily Standups
```
Show me the 15 most recent Jira issues
```

### Code Review Context
```
Find documentation about the payment API and show me related bugs
```

### Bug Triage
```
Search for "critical security" issues and get the full details of the top 3
```

### Documentation Discovery
```
List recent Confluence pages about the authentication service
```

### Dependency Tracking
```
Show me all documents that reference EPIC-42
```

### Knowledge Transfer
```
Search for all documents about the reporting system and summarize key points
```

---

## Troubleshooting Query Issues

### Empty Results
**Problem:** Query returns no results

**Solutions:**
- Remove source type filters
- Use broader search terms
- Check if database is populated
- Try listing recent documents (no query needed)

### Too Many Results
**Problem:** Query returns hundreds of results

**Solutions:**
- Use more specific keywords
- Add source type filters
- Use FTS5 syntax for precision (+required, "exact phrase")
- Reduce limit parameter

### Wrong Document Types
**Problem:** Getting Jira issues when you want docs

**Solutions:**
- Add `source_types: ["confluence"]` filter
- Use source-specific keywords ("documentation", "guide")
- Use list_recent_documents with source_type filter

---

## Next Steps

- **Configure Claude Desktop:** See [mcp-configuration.md](mcp-configuration.md)
- **Populate Your Knowledge Base:** Use Quaero's crawler to collect documents
- **Experiment:** Try different query patterns and filters
- **Build Workflows:** Combine tools for comprehensive research
- **Report Issues:** File bugs or feature requests in the Quaero repository

## See Also

- [MCP Configuration Guide](mcp-configuration.md) - Setup instructions
- [Quaero README](../../README.md) - Main documentation
- [MCP Protocol Specification](https://github.com/mark3labs/mcp-go) - Technical details
