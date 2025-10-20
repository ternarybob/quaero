I have the following verification comments after thorough review and exploration of the codebase. Implement the comments by following the instructions in the comments verbatim.

---
## Comment 1: HTML parsing now receives markdown due to result body selection order, breaking transformers.

In `internal/services/atlassian/helpers.go` update `selectResultBody()` to change priority as follows: 1) If `result.Metadata["html"]` is a non-empty string, return that as bytes. 2) Else if `result.Metadata["response_body"]` exists, return it (string or []byte). 3) Else if `result.Body` looks like HTML (trimmed string starts with “<”), return it. 4) As a last resort, only return markdown for consumers that explicitly want markdown; for current HTML parsers, do not fall back to markdown. Ensure both `jira_transformer.go` `parseJiraIssue()` and `confluence_transformer.go` `parseConfluencePage()` continue to use `selectResultBody()` without further changes.

### Referred Files
- c:\development\quaero\internal\services\atlassian\helpers.go
- c:\development\quaero\internal\services\atlassian\jira_transformer.go
- c:\development\quaero\internal\services\atlassian\confluence_transformer.go
- c:\development\quaero\internal\services\crawler\types.go
---
## Comment 2: Parser files left empty will cause Go build errors; remove them completely.

Delete `internal/services/crawler/jira_parser.go` and `internal/services/crawler/confluence_parser.go` from the repository. Confirm they are not referenced anywhere (search references) and verify the project builds. Do not leave zero-byte `.go` files without a `package` clause.

### Referred Files
- c:\development\quaero\internal\services\crawler\jira_parser.go
- c:\development\quaero\internal\services\crawler\confluence_parser.go
---
## Comment 3: CrawlResult.Body now holds markdown by default, which may violate downstream expectations of HTML.

Either update `ScrapeResult.ToCrawlResult()` to set `Body` to HTML/RawHTML instead of markdown, keeping markdown in `metadata["markdown"]`, or document the contract clearly and ensure any consumers of `CrawlResult.Body` expecting HTML switch to `metadata["html"]`/`metadata["response_body"]`. If changing behavior, update `internal/services/crawler/types.go` `ToCrawlResult()` accordingly and test transformers and link discovery.

### Referred Files
- c:\development\quaero\internal\services\crawler\types.go
- c:\development\quaero\internal\services\atlassian\helpers.go
---