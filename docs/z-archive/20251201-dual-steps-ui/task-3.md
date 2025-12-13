# Task 3: Add document_filter_tags to Job Definition Model

- Group: 3 | Mode: sequential | Model: sonnet
- Skill: @golang-pro | Critical: no | Depends: none
- Sandbox: /tmp/3agents/task-3/ | Source: ./ | Output: ./docs/fix/20251201-dual-steps-ui/

## Files
- `internal/models/job_definition.go` - Add filter_tags validation to JobStep

## Requirements

From the prompt:
> Add a config option to the job definition to enable the document filtering
> - 'document_filter_tags' lists the documents filter, by 'tags' that a step should be executed against. If not provided, the step should be executed against all documents.

The JobStep struct already supports a Config field (map[string]interface{}) where filter fields are stored.
Looking at existing code (lines 329-363), there's already validation for:
- `filter_tags` ([]interface{})
- `filter_created_after` (RFC3339 string)
- `filter_updated_after` (RFC3339 string)
- `filter_limit` (int/float64)

The `filter_tags` validation already exists at line 330-336:
```go
// Validate filter_tags if provided
if tags, ok := step.Config["filter_tags"].([]interface{}); ok {
    for i, tag := range tags {
        if _, ok := tag.(string); !ok {
            return fmt.Errorf("filter_tags[%d] must be a string", i)
        }
    }
}
```

This task is to:
1. Verify the validation is working correctly
2. Document the `filter_tags` field usage in the code comments
3. Ensure the field is passed through to workers when executing steps

## Acceptance
- [ ] filter_tags validation is present and working
- [ ] Documentation comments added explaining filter_tags usage
- [ ] Compiles
- [ ] Tests pass
