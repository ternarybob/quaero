# Plan: Document Tagging System

## Overview
Implement a tagging system that allows job definitions to tag documents, replace source-type specific UI elements with generic tag-based filtering, and add a tags column to the document list.

## Requirements Analysis
Based on the screenshot and requirements:
1. Remove "JIRA DOCUMENTS" and "CONFLUENCE DOCUMENTS" from statistics header
2. Replace source type dropdown with multi-select tag autocomplete filter
3. Add "TAGS" column to document table
4. Job definitions need to specify tags in TOML config
5. Tags stored in documents table (already exists in schema as TEXT field)

## Steps

### 1. **Update Job Definition Model and Storage for Tags**
   - Skill: @code-architect
   - Files: `internal/models/job_definition.go`, `internal/storage/sqlite/job_definition_storage.go`, `deployments/local/job-definitions/news-crawler.toml`
   - User decision: no
   - Add tags field to JobDefinition struct
   - Update TOML parsing to read tags array
   - Update storage layer to persist tags
   - Update example job definition with tags

### 2. **Update Crawler to Apply Tags to Documents**
   - Skill: @go-coder
   - Files: `internal/jobs/executor/crawler_step_executor.go`, `internal/services/crawler/service.go`
   - User decision: no
   - Modify crawler executor to pass tags from job definition to created documents
   - Ensure tags are stored as JSON array in documents.tags field

### 3. **Update Document Storage for Tag Handling**
   - Skill: @go-coder
   - Files: `internal/storage/sqlite/document_storage.go`
   - User decision: no
   - Ensure SaveDocument properly handles tags field (JSON array serialization)
   - Add GetAllTags() method to retrieve unique tags across all documents
   - Update GetDocuments to support tag filtering

### 4. **Add Tag Filter API Endpoints**
   - Skill: @go-coder
   - Files: `internal/handlers/search_handler.go`, `internal/server/routes.go`
   - User decision: no
   - Add GET /api/documents/tags endpoint (returns all unique tags)
   - Update GET /api/documents to accept tags query parameter (comma-separated)
   - Update document stats endpoint to remove jira/confluence counts

### 5. **Update Frontend UI - Statistics Section**
   - Skill: @go-coder
   - Files: `pages/documents.html`
   - User decision: no
   - Remove "JIRA DOCUMENTS" and "CONFLUENCE DOCUMENTS" stat boxes
   - Keep only "TOTAL DOCUMENTS" statistic
   - Update loadStats() JavaScript function

### 6. **Update Frontend UI - Table and Filters**
   - Skill: @go-coder
   - Files: `pages/documents.html`
   - User decision: no
   - Replace source type dropdown with tag multi-select autocomplete
   - Add TAGS column to document table (between DETAILS and UPDATED)
   - Update renderDocuments() to display tags
   - Implement tag filtering logic with multi-select support

### 7. **Add Common Tag Selection Component**
   - Skill: @go-coder
   - Files: `pages/static/common.js`
   - User decision: no
   - Create reusable tag autocomplete/multi-select component
   - Support comma-separated tag input
   - Load available tags from /api/documents/tags
   - Emit filter change events

### 8. **Write API Tests**
   - Skill: @test-writer
   - Files: `test/api/document_tags_test.go`
   - User decision: no
   - Test tag assignment from job definitions
   - Test tag filtering via API
   - Test GetAllTags endpoint
   - Test document stats without source-type counts

### 9. **Write UI Tests**
   - Skill: @test-writer
   - Files: `test/ui/document_tags_test.go`
   - User decision: no
   - Test tag column display
   - Test tag filter interaction
   - Test multi-select tag filtering
   - Verify statistics section shows only total

## Success Criteria
- Job definitions can specify tags in TOML (tags = ["tag1", "tag2"])
- Documents created by jobs inherit tags from job definition
- UI shows only "TOTAL DOCUMENTS" statistic (no jira/confluence)
- UI has tag column in document table
- UI has multi-select tag filter replacing source type dropdown
- Tag filtering works with multiple selected tags
- API endpoint /api/documents/tags returns all unique tags
- All tests pass

## Architecture Notes
- Tags stored as JSON array in documents.tags TEXT field (already in schema)
- Job definitions pass tags to crawler, which applies to all created documents
- Frontend uses autocomplete with multi-select for tag filtering
- Backend filters documents by checking if any selected tag exists in document's tags array
