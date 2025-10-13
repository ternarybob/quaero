# Pointer RAG Implementation Plan for AI Assistant (CLI)

## Overview
This document provides a comprehensive step-by-step guide to implement the Pointer RAG system using a service-oriented architecture with interfaces and methods for enhanced modularity and testability. Follow each stage sequentially.

---

## Stage 1: Enhance Document Schema with Rich Metadata

**Goal:** Add source-specific metadata fields to the Document model to enable cross-source linking.

### Step 1.1: Modify the Document Model

**Action:** Update the Go structs in the models file.

**Instruction:** Read `internal/models/document.go` and update it to match the following code, which adds the `Metadata` field to the `Document` struct and defines the new `DocumentMetadata` struct.

```go
package models

import "time"

// Document represents a chunk of content from a data source.
type Document struct {
    ID         string
    SourceID   string
    SourceType string // "jira", "confluence", "github"
    Title      string
    Content    string
    URL        string
    Embedding  []float32
    LastSynced time.Time
    CreatedAt  time.Time
    UpdatedAt  time.Time

    // NEW: Rich metadata (stored as JSON)
    Metadata DocumentMetadata
}

// DocumentMetadata contains source-specific and cross-source extracted data.
type DocumentMetadata struct {
    // Jira-specific
    IssueKey       string    `json:"issue_key,omitempty"`
    IssueType      string    `json:"issue_type,omitempty"`
    Status         string    `json:"status,omitempty"`
    ResolutionDate time.Time `json:"resolution_date,omitempty"`

    // Confluence-specific
    PageTitle    string    `json:"page_title,omitempty"`
    SpaceKey     string    `json:"space_key,omitempty"`
    Author       string    `json:"author,omitempty"`
    LastModified time.Time `json:"last_modified,omitempty"`

    // GitHub-specific
    RepoName     string `json:"repo_name,omitempty"`
    FilePath     string `json:"file_path,omitempty"`
    CommitSHA    string `json:"commit_sha,omitempty"`
    FunctionName string `json:"function_name,omitempty"`

    // Cross-source identifiers
    ReferencedIssues []string `json:"referenced_issues,omitempty"`
}
```

### Step 1.2: Create the Database Migration

**Action:** Create a new SQL migration file to alter the database schema.

**Instruction:** Create `internal/storage/sqlite/migrations/006_add_document_metadata.sql` with the following content:

```sql
-- Add metadata column (JSON type in SQLite)
ALTER TABLE documents ADD COLUMN metadata TEXT;

-- Create indexes for common metadata queries
CREATE INDEX idx_documents_issue_key ON documents(json_extract(metadata, '$.issue_key'));
CREATE INDEX idx_documents_repo_name ON documents(json_extract(metadata, '$.repo_name'));
CREATE INDEX idx_documents_space_key ON documents(json_extract(metadata, '$.space_key'));
```

### Step 1.3: Update the Storage Layer

**Action:** Modify the document storage logic to handle the new JSON metadata.

**Instruction:** Read `internal/storage/sqlite/document_storage.go`. Update the `Insert` and `Update` methods (or their equivalents) to handle the `Metadata` field. You will need to:
- Use `json.Marshal` to serialize the struct before saving to the metadata column
- Use `json.Unmarshal` when retrieving documents

### Step 1.4: Update Data Ingestion Services

**Action:** Update the Jira and Confluence services to populate the new metadata fields during data ingestion.

**Instruction 1 (Jira):** Read `internal/services/atlassian/jira_service.go`. In the function that transforms Jira API data into a Document model, populate the `Metadata` field with `IssueKey`, `IssueType`, and `Status`.

**Instruction 2 (Confluence):** Read `internal/services/atlassian/confluence_service.go`. In the function that transforms Confluence API data into a Document model, populate the `Metadata` field with `PageTitle`, `SpaceKey`, and `Author`.

---

## Stage 2: Implement Identifier Extraction and Linking

**Goal:** Create a dedicated service to find identifiers like BUG-123 in content and a new search method to find documents that reference them.

### Step 2.1: Create the Identifier Extractor Service

**Action:** Create a new service with an interface for identifier extraction.

**Instruction:** Create the file `internal/services/identifiers/extractor.go` with the following content. This establishes a clear contract for the service.

```go
package identifiers

import (
    "regexp"
    "github.com/your-username/quaero/internal/models" // Adjust this import path
)

// Service defines the interface for identifier extraction operations.
type Service interface {
    ExtractFromText(content string) []string
    ExtractFromDocuments(docs []*models.Document) []string
}

type service struct {
    patterns map[string]*regexp.Regexp
}

// NewService creates a new identifier extraction service.
func NewService() Service {
    return &service{
        patterns: map[string]*regexp.Regexp{
            "jira_issue": regexp.MustCompile(`\b([A-Z]+-\d+)\b`),
        },
    }
}

func (s *service) ExtractFromText(content string) []string {
    var identifiers []string
    for _, pattern := range s.patterns {
        matches := pattern.FindAllString(content, -1)
        if matches != nil {
            identifiers = append(identifiers, matches...)
        }
    }
    return unique(identifiers)
}

func (s *service) ExtractFromDocuments(docs []*models.Document) []string {
    var allIdentifiers []string
    for _, doc := range docs {
        if doc.Metadata.IssueKey != "" {
            allIdentifiers = append(allIdentifiers, doc.Metadata.IssueKey)
        }
        allIdentifiers = append(allIdentifiers, doc.Metadata.ReferencedIssues...)
        allIdentifiers = append(allIdentifiers, s.ExtractFromText(doc.Content)...)
    }
    return unique(allIdentifiers)
}

func unique(items []string) []string {
    seen := make(map[string]struct{})
    var result []string
    for _, item := range items {
        if _, ok := seen[item]; !ok {
            seen[item] = struct{}{}
            result = append(result, item)
        }
    }
    return result
}
```

### Step 2.2: Update the Document Service Interface

**Action:** Add a new search method to the external document service interface.

**Instruction:** Read `internal/interfaces/document_service.go`. Add the `SearchByIdentifierQuery` struct and the `SearchByIdentifier` method signature to the `DocumentService` interface.

```go
// Add this struct to the interfaces file
type SearchByIdentifierQuery struct {
    Identifier     string
    ExcludeSources []string
    Limit          int
}

// Add this method to the DocumentService interface
SearchByIdentifier(ctx context.Context, query *SearchByIdentifierQuery) ([]*models.Document, error)
```

### Step 2.3: Implement SearchByIdentifier in Storage Layer

**Action:** Implement the new search method in the SQLite storage layer.

**Instruction:** Read `internal/storage/sqlite/document_storage.go`. Implement the `SearchByIdentifier` method on the `DocumentStorage` struct. Use the following SQL as a reference, ensuring you correctly handle parameter binding and dynamic filtering.

```sql
-- Reference SQL for SearchByIdentifier implementation
SELECT * FROM documents
WHERE
    (
        json_extract(metadata, '$.issue_key') = ? OR
        content LIKE ? OR
        json_extract(metadata, '$.referenced_issues') LIKE ?
    )
-- Your Go code will need to dynamically add 'AND source_type NOT IN (...)'
LIMIT ?;
```

---

## Stage 3: Implement Augmented RAG Retrieval

**Goal:** Enhance the chat service to perform a multi-phase retrieval using the newly created services.

### Step 3.1: Implement the Augmented Retrieval Logic

**Action:** Modify the ChatService to orchestrate the new retrieval flow and manage its dependencies.

**Instruction:** Read `internal/services/chat/chat_service.go`.

1. Update the `ChatService` struct to include the `documents.Service` and `identifiers.Service` as dependencies
2. Update the `NewService` constructor to accept and store these service interfaces
3. Implement a new private method, `(s *service) retrieveContextAugmented(...)`. This method will use the injected services (`s.documentService`, `s.identifierService`) to perform the multi-phase search

### Step 3.2: Create Retrieval Helper Functions

**Action:** Create a new file for helper functions to keep the service logic clean.

**Instruction:** Create `internal/services/chat/augmented_retrieval.go`. Add the `deduplicateDocuments` and `rankByCrossSourceConnections` functions from the design document into this file.

### Step 3.3: Integrate Augmented Retrieval into the Chat Flow

**Action:** Update the main Chat method to use the new retrieval logic.

**Instruction:** In `internal/services/chat/chat_service.go`, modify the `(s *service) Chat(...)` method. It should now call `s.retrieveContextAugmented(...)` instead of the previous simple vector search.

---

## Stage 4: Implement Pointer RAG Prompt Engineering

**Goal:** Create a structured prompt that instructs the LLM to act as a context analyst.

### Step 4.1: Create Prompt Templates

**Action:** Create a new file to store the LLM prompt templates.

**Instruction:** Create `internal/services/chat/prompt_templates.go` and add the `PointerRAGSystemPrompt` constant exactly as defined in the design document.

### Step 4.2: Create Document Formatting Helpers

**Action:** Create a new file for formatting documents into a text-based context block.

**Instruction:** Create `internal/services/chat/document_formatter.go`. Add the `formatDocument` and `formatMetadata` helper functions into this file.

### Step 4.3: Implement the Prompt Builder

**Action:** Add prompt construction logic as methods on the ChatService.

**Instruction:** In `internal/services/chat/chat_service.go`, create the private methods:
- `(s *service) buildPointerRAGMessages(...)`
- `(s *service) buildRetrievedDocumentsSection(...)`

These methods will use the helper functions from the previous step to construct the final messages for the LLM.

### Step 4.4: Finalize Chat Method Integration

**Action:** Integrate the new prompt builder into the main chat method.

**Instruction:** In `internal/services/chat/chat_service.go`, update the `(s *service) Chat(...)` method again. It should now:
- Call `s.buildPointerRAGMessages(...)` to prepare the request for the LLM service
- Update the `ChatResponse` to include metadata about the retrieval

---

## Stage 5: Add Testing and Validation

**Goal:** Create high-level integration tests to validate the end-to-end Pointer RAG flow.

### Step 5.1: Create an API Integration Test

**Action:** Create a new test file for API-level integration testing.

**Instruction:** Create `test/api/chat_rag_pointer_test.go`. In this file, set up a test suite that can interact with an in-memory or test instance of your application.

### Step 5.2: Implement the "Bug Resolution Tracing" Test Case

**Action:** Add a specific test function to validate a common use case.

**Instruction:** In `test/api/chat_rag_pointer_test.go`, implement the `TestPointerRAG_BugResolutionTracing` test function. This test should:

**Arrange:** Set up mock services and data, including:
- A mock Jira ticket
- A Confluence page
- A GitHub commit

**Act:** Make a test chat request to the API with a query like "What caused BUG-456 and how was it fixed?"

**Assert:** Verify that:
- The response message contains the required sections: ANALYSIS SUMMARY, PRIMARY SOURCES, etc.
- The response text includes the URLs for the mock documents
- The response metadata indicates that linked documents were found and that "BUG-456" was an extracted identifier

---

## Implementation Checklist

- [ ] **Stage 1:** Document schema enhanced with metadata
- [ ] **Stage 2:** Identifier extraction service implemented
- [ ] **Stage 3:** Augmented RAG retrieval integrated
- [ ] **Stage 4:** Pointer RAG prompts implemented
- [ ] **Stage 5:** Integration tests completed

---

## Notes

- Execute each step in the specified order to maintain system integrity
- Test each stage before moving to the next
- Adjust import paths according to your project structure
- Ensure all dependencies are properly injected using the service-oriented architecture pattern