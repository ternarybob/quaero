# Iteration 1

**Goal:** Fix test to create documents and verify keyword extraction produces results

---

## Agent 1 - Implementation

### Failures to Address

1. **No documents for keyword extraction to process**
   - Current: Keyword job runs with 0 documents
   - Expected: Test documents should exist in database
   - Fix: Insert test documents via POST /api/documents

2. **Test doesn't verify document_count > 0**
   - Current: Test checks status=completed but not result_count
   - Expected: Test should fail if result_count == 0
   - Fix: Add verification for result_count > 0

### Analysis

The test currently passes superficially but doesn't test the actual requirement:
- Keyword extraction agent needs documents to process
- Job completes with `result_count: 0` because no documents exist
- Test should insert markdown documents and verify keywords are extracted

Based on document_handler.go:253-319, the API expects:
```json
POST /api/documents
{
  "id": "string",
  "source_type": "string",
  "title": "string",
  "content_markdown": "string",
  "url": "string (optional)",
  "source_id": "string (optional)",
  "metadata": {}
}
```

### Proposed Fixes

**File: `test/ui/keyword_job_test.go`**

1. **Add helper function to insert test documents** (after line 617, before pollForParentJobCreation)
   - Function: `insertTestDocument(t, h, env, id, title, content) error`
   - Insert via POST /api/documents
   - Log success/failure

2. **Insert 3 test documents before Phase 2** (replace lines 127-128)
   - Remove skip message
   - Add "=== PHASE 1: Create Test Documents ===" section
   - Insert 3 documents with markdown content about different topics
   - Topics: "AI and Machine Learning", "Web Development", "Cloud Computing"

3. **Update Phase 2 verification** (lines 558-617)
   - Check `result_count` field in API response
   - Add assertion: result_count > 0
   - Fail test if result_count == 0

### Changes Made

**`test/ui/keyword_job_test.go`:**

#### Change 1: Add helper function to insert test documents

**Location:** After line 617 (after TestKeywordJob function, before pollForParentJobCreation)

```go
// insertTestDocument creates a test document via POST /api/documents
func insertTestDocument(t *testing.T, h *common.HTTPTestHelper, env *common.TestEnvironment, id, title, content string) error {
	doc := map[string]interface{}{
		"id":               id,
		"source_type":      "test",
		"title":            title,
		"content_markdown": content,
		"url":              "",
		"source_id":        "test-source",
		"metadata": map[string]interface{}{
			"test": true,
		},
	}

	env.LogTest(t, "Creating test document: %s (%s)", id, title)
	resp, err := h.POST("/api/documents", doc)
	if err != nil {
		return fmt.Errorf("failed to create document: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	env.LogTest(t, "✓ Test document created: %s", id)
	return nil
}
```

#### Change 2: Replace Phase 1 skip with document creation

**Location:** Lines 114-128 (replace entire Phase 1 section)

**Before:**
```go
// ============================================================
// PHASE 1: Run "places-nearby-restaurants" job (SKIPPED)
// ============================================================
// NOTE: Phase 1 is skipped because it requires Google Places API (Legacy)
// ...
env.LogTest(t, "=== PHASE 1: Places Job - SKIPPED (requires Places API Legacy) ===")
env.LogTest(t, "⚠️  Skipping Phase 1 - requires Places API (Legacy) enablement")
```

**After:**
```go
// ============================================================
// PHASE 1: Create Test Documents for Keyword Extraction
// ============================================================

env.LogTest(t, "=== PHASE 1: Creating Test Documents ===")

// Insert 3 test documents with markdown content
testDocs := []struct {
	id      string
	title   string
	content string
}{
	{
		id:    "test-doc-ai-ml",
		title: "Introduction to AI and Machine Learning",
		content: `# AI and Machine Learning

Artificial Intelligence (AI) and Machine Learning (ML) are transforming technology.

## Key Concepts
- **Neural Networks**: Inspired by the human brain
- **Deep Learning**: Multiple layers of neural networks
- **Natural Language Processing**: Understanding human language
- **Computer Vision**: Teaching computers to see

## Applications
AI is used in healthcare, finance, transportation, and entertainment.

### Keywords
artificial intelligence, machine learning, neural networks, deep learning, NLP, computer vision, algorithms, data science`,
	},
	{
		id:    "test-doc-web-dev",
		title: "Modern Web Development Practices",
		content: `# Modern Web Development

Web development has evolved significantly with modern frameworks and tools.

## Frontend Technologies
- **React**: Component-based UI library
- **Vue.js**: Progressive JavaScript framework
- **TypeScript**: Typed superset of JavaScript

## Backend Technologies
- **Node.js**: JavaScript runtime
- **Go**: Fast compiled language
- **Python**: Versatile and popular

## Best Practices
- Responsive design
- Progressive web apps
- Performance optimization
- Security best practices

### Keywords
web development, frontend, backend, React, Vue, TypeScript, Node.js, Go, Python, JavaScript, frameworks, responsive design`,
	},
	{
		id:    "test-doc-cloud",
		title: "Cloud Computing and DevOps",
		content: `# Cloud Computing

Cloud computing provides on-demand computing resources over the internet.

## Major Cloud Providers
- **AWS**: Amazon Web Services
- **Azure**: Microsoft Azure
- **GCP**: Google Cloud Platform

## DevOps Practices
- Continuous Integration/Continuous Deployment (CI/CD)
- Infrastructure as Code (IaC)
- Container orchestration with Kubernetes
- Monitoring and observability

## Benefits
- Scalability
- Cost efficiency
- Global availability
- Disaster recovery

### Keywords
cloud computing, AWS, Azure, GCP, DevOps, CI/CD, Kubernetes, Docker, infrastructure, microservices, containers, automation`,
	},
}

for _, doc := range testDocs {
	if err := insertTestDocument(t, h, env, doc.id, doc.title, doc.content); err != nil {
		env.LogTest(t, "ERROR: Failed to create test document: %v", err)
		t.Fatalf("Failed to create test document: %v", err)
	}
}

env.LogTest(t, "✓ Created %d test documents for keyword extraction", len(testDocs))
env.LogTest(t, "✅ PHASE 1 PASS: Test documents created")
```

#### Change 3: Update Phase 2 verification to check result_count

**Location:** Lines 558-617 (update pollForJobStatus verification)

Add after line 564 (after getting job status):

```go
// Get result_count from API response
keywordResultCount := 0
jobResp, err := h.GET("/api/jobs/" + keywordJobID)
if err == nil {
	var jobData map[string]interface{}
	if err := h.ParseJSONResponse(jobResp, &jobData); err == nil {
		if rc, ok := jobData["result_count"].(float64); ok {
			keywordResultCount = int(rc)
		}
	}
}

env.LogTest(t, "✓ Keyword job result_count: %d", keywordResultCount)

// VERIFY: Test should fail if no documents were processed
if keywordResultCount == 0 {
	env.LogTest(t, "ERROR: Keyword job processed 0 documents - test fails")
	env.LogTest(t, "  This means no keywords were extracted from the test documents")
	t.Fatalf("Keyword job must process documents and extract keywords (result_count > 0), got: %d", keywordResultCount)
}
```

Replace existing verification (lines 571-616) with:

```go
// Verify job completed successfully WITH results
if keywordJobStatus == "failed" {
	env.LogTest(t, "ERROR: Keyword job failed: %s", keywordJobError)
	t.Fatalf("Keyword job failed: %s", keywordJobError)
}

if keywordJobStatus == "completed" && keywordResultCount > 0 {
	env.LogTest(t, "✓ Keyword job completed successfully")
	env.LogTest(t, "✓ Processed %d documents and extracted keywords", keywordResultCount)
	env.LogTest(t, "✅ PHASE 2 PASS: Keywords extracted from %d documents", keywordResultCount)
} else {
	env.LogTest(t, "ERROR: Job completed but processed 0 documents")
	t.Fatalf("Expected result_count > 0, got: %d", keywordResultCount)
}

env.LogTest(t, "✓ Test completed successfully")
```

### Compilation Check

```bash
cd test/ui && go build keyword_job_test.go
```

**Result:** ✅ Code compiles successfully

---

## Bug Discovery and Fix

### Issue: Database Constraint Violation

**Error:** Second document creation fails with 500 error: "Failed to save document"

**Root Cause:**
- Database has a UNIQUE constraint on `(source_type, source_id)` combination
- All test documents used same `source_id: "test-source"`
- Second document violated constraint: `UNIQUE constraint failed: documents.source_type, documents.source_id`

**Source:** `internal/storage/sqlite/document_storage.go:64-88`
```sql
ON CONFLICT(source_type, source_id) DO UPDATE SET ...
```

**Fix:** Changed `insertTestDocument()` to use unique `source_id` per document:
```go
// Before:
"source_id": "test-source",  // Same for all documents - WRONG!

// After:
"source_id": id,  // Use unique document ID - CORRECT!
```

This ensures each document has a unique `(source_type, source_id)` combination.

---

## Summary of Changes

1. **Added** `insertTestDocument()` helper function to create documents via API
2. **Replaced** Phase 1 skip with document creation (3 test documents with markdown)
3. **Updated** Phase 2 verification to check `result_count > 0`
4. **Added** test failure if `result_count == 0`
5. **Fixed** database constraint issue by using unique `source_id` per document

**Expected Result:**
- Test creates 3 documents with markdown content
- Keyword extraction job processes documents
- Test verifies result_count > 0
- Test fails if no keywords extracted
