# Task 2: Refactor TOML Parsing for New Format

- Group: 2 | Mode: sequential | Model: sonnet
- Skill: @golang-pro | Critical: no | Depends: 1
- Sandbox: /tmp/3agents/task-2/ | Source: ./ | Output: ./docs/feature/job-steps-refactor/

## Files
- `internal/jobs/service.go` - Refactor TOML parsing

## Requirements

### New TOML Structure
Change from `[[steps]]` array to `[step.{name}]` tables:

```toml
# OLD
[[steps]]
name = "search_step"
action = "web_search"
on_error = "fail"
[steps.config]
query = "search"
[steps.config.document_filter]
limit = 100

# NEW
[step.search_step]
action = "web_search"
on_error = "fail"
depends = ""
query = "search"
filter_limit = 100
```

### Changes Required

1. Update `JobDefinitionFile` struct:
   - Change `Steps []JobStepFile` to `Step map[string]JobStepFile`
   - The map key becomes the step name

2. Update `JobStepFile` struct:
   - Remove `Name` field (now in map key)
   - Remove `Config` map (fields are flat)
   - Add `Depends` field
   - Add direct fields: `Query`, `APIKey`, `Depth`, `Breadth`, etc.
   - Add `FilterLimit`, `FilterTags`, `FilterSourceType`, etc. for document_filter
   - Use `toml:",remain"` for extra fields into a map

3. Update `ParseTOML` function:
   - No changes needed if struct tags are correct

4. Update `ToJobDefinition` conversion:
   - Iterate map entries to create JobStep slice
   - Copy flat fields into Config map
   - Set step.Name from map key
   - Set step.Depends from parsed field

5. Maintain backward compatibility:
   - Keep support for legacy `[[steps]]` format during transition
   - Detect which format is used and parse accordingly

## Acceptance
- [ ] New [step.name] format parses correctly
- [ ] Flat config fields (query, api_key, filter_*) work
- [ ] Depends field is parsed and populated
- [ ] Legacy [[steps]] format still works (optional)
- [ ] Compiles: `go build ./...`
