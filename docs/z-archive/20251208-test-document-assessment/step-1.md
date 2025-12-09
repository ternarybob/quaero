# Step 1: Add assessPerFileEnrichment() Function

## What Was Done

Added a new function `assessPerFileEnrichment()` to `test/ui/devops_enrichment_test.go` that:

1. **Fetches all enriched documents** via `/api/documents?tags=devops-enriched`
2. **Assesses each document** for DevOps metadata fields:
   - `includes` - extracted include directives
   - `defines` - extracted preprocessor defines
   - `platforms` - detected platform-specific code
   - `component` - classified component name
   - `file_role` - classified file role (header, source, test, etc.)
3. **Tracks pass/fail** for each document based on enrichment fields populated
4. **Saves JSON report** to `per_file_assessment.json` in the test results directory

## Types Added

```go
type PerFileAssessment struct {
    DocumentID   string   `json:"document_id"`
    Title        string   `json:"title"`
    HasDevOps    bool     `json:"has_devops"`
    HasIncludes  bool     `json:"has_includes"`
    HasDefines   bool     `json:"has_defines"`
    HasPlatforms bool     `json:"has_platforms"`
    HasComponent bool     `json:"has_component"`
    HasFileRole  bool     `json:"has_file_role"`
    PassCount    int      `json:"pass_count"`
    Issues       []string `json:"issues,omitempty"`
}

type PerFileAssessmentReport struct {
    GeneratedAt     string             `json:"generated_at"`
    TotalDocuments  int                `json:"total_documents"`
    PassedDocuments int                `json:"passed_documents"`
    FailedDocuments int                `json:"failed_documents"`
    Assessments     []PerFileAssessment `json:"assessments"`
}
```

## Files Modified

- `test/ui/devops_enrichment_test.go` - Added lines 1854-2003
