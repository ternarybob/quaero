---
task: "Remove redundant data source-specific code (Jira/Confluence/GitHub API integrations)"
folder: remove-redundant-data-sources
complexity: high
estimated_steps: 12
---

# Implementation Plan: Remove Redundant Data Source Code

## Executive Summary

The codebase contains Atlassian (Jira/Confluence) and GitHub specific API integration code that is now redundant. The generic ChromeDP-based crawler has replaced these direct API connections. This plan outlines the safe removal of:

1. **Configuration**: Jira/Confluence/GitHub sections in config files
2. **Database Schema**: Jira/Confluence tables and columns
3. **Models**: Atlassian-specific data structures
4. **Interfaces**: Service interfaces for scrapers
5. **Storage**: Atlassian-specific storage code
6. **Auth**: Atlassian-specific authentication (Chrome extension remains for generic auth)
7. **Documentation**: References to direct API integrations

## Current State Analysis

### Components to Remove

**Configuration (internal/common/config.go):**
- `SourcesConfig` struct with Jira/Confluence/GitHub sub-configs
- Lines 46-66: Configuration structures for data sources

**Database Schema (internal/storage/sqlite/schema.go):**
- `jira_projects` table (lines 36-43)
- `jira_issues` table (lines 45-53)
- `confluence_spaces` table (lines 56-64)
- `confluence_pages` table (lines 66-74)

**Models (internal/models/atlassian.go):**
- `JiraProject` struct
- `JiraIssue` struct
- `ConfluenceSpace` struct
- `ConfluencePage` struct
- `AuthCredentials` struct (KEEP - used by generic auth)

**Interfaces (internal/interfaces/atlassian.go):**
- `AtlassianAuthService` interface (used by auth service)
- `JiraScraperService` interface (REMOVE)
- `ConfluenceScraperService` interface (REMOVE)
- `AtlassianExtensionCookie` (KEEP - used by generic auth)
- `AtlassianAuthData` (KEEP - used by generic auth)

**Auth Service (internal/services/auth/service.go):**
- Service constants: `ServiceNameAtlassian`, `ServiceNameGitHub` (lines 16-19)
- Atlassian-specific methods can be generified

**Storage:**
- No Jira/Confluence specific storage implementations exist (already generic)

**Chrome Extension:**
- KEEP - The extension is for generic authentication, not Atlassian-specific
- References to "Jira/Confluence" in docs are just examples
- Core functionality is generic cookie/token capture

**Documentation:**
- CLAUDE.md: References to Jira/Confluence API integrations
- README.md: Configuration examples
- Chrome extension README: Example references

### Components to Keep

**Generic Auth Infrastructure:**
- `auth_credentials` table
- `AuthStorage` interface and implementation
- Chrome extension (generic auth capture)
- HTTP client configuration from auth data

**Generic Crawler:**
- Crawler service (generic HTML crawling)
- Document storage (source-agnostic)
- Job definitions and processing

**Metadata Extractors:**
- `internal/services/identifiers/extractor.go` - Generic identifier extraction (has Jira/Confluence patterns)
- `internal/services/metadata/extractor.go` - Generic metadata extraction (has Jira/Confluence patterns)
- These extractors work with any URL pattern and should be kept

## Dependencies and Risks

### High Risk Areas

1. **App Initialization** (`internal/app/app.go`):
   - Lines 183-191: References enabled sources in logging
   - Lines 328-335: Auth service initialization
   - No actual Jira/Confluence service initialization (already removed)

2. **Configuration Loading**:
   - Breaking change to config file format
   - Existing deployments will have invalid configs

3. **Database Schema**:
   - Existing databases will have orphaned tables
   - Need migration to clean up

### Low Risk Areas

1. **Chrome Extension**: No changes needed (already generic)
2. **Auth Storage**: Already generic, no changes needed
3. **Crawler Service**: Already using generic approach

## Implementation Steps

---

## Step 1: Remove Database Tables via Migration

**Why:** Clean up orphaned Jira/Confluence tables from existing databases safely

**Depends on:** none

**Validation:** database_schema_clean

**Creates/Modifies:**
- `C:\development\quaero\internal\storage\sqlite\schema.go`

**Actions:**
1. Add new migration function `migrateRemoveAtlassianTables()`
2. Drop tables: `jira_projects`, `jira_issues`, `confluence_spaces`, `confluence_pages`
3. Add to `runMigrations()` sequence
4. Test migration on existing database

**Risk:** low (tables are unused, migration is idempotent)

---

## Step 2: Remove Jira/Confluence Table Definitions from Schema

**Why:** Remove table creation SQL for deprecated tables

**Depends on:** Step 1

**Validation:** code_compiles

**Creates/Modifies:**
- `C:\development\quaero\internal\storage\sqlite\schema.go`

**Actions:**
1. Remove Jira table definitions (lines 36-53)
2. Remove Confluence table definitions (lines 56-74)
3. Keep `auth_credentials` table (used by generic auth)

**Risk:** low (migration handles existing databases)

---

## Step 3: Remove Source-Specific Configuration Structures

**Why:** Remove unused configuration sections

**Depends on:** Step 2

**Validation:** code_compiles

**Creates/Modifies:**
- `C:\development\quaero\internal\common\config.go`

**Actions:**
1. Remove `SourcesConfig` struct (lines 46-50)
2. Remove `ConfluenceConfig` struct (lines 52-55)
3. Remove `JiraConfig` struct (lines 57-60)
4. Remove `GitHubConfig` struct (lines 62-66)
5. Remove `Sources` field from `Config` struct (line 19)
6. Remove environment variable overrides for sources (if any)
7. Update `NewDefaultConfig()` to remove sources initialization

**Risk:** medium (breaking change to config file format)

---

## Step 4: Remove Source-Specific Models

**Why:** Clean up unused data structures

**Depends on:** Step 3

**Validation:** code_compiles

**Creates/Modifies:**
- `C:\development\quaero\internal\models\atlassian.go`

**Actions:**
1. Remove `JiraProject` struct (lines 4-9)
2. Remove `JiraIssue` struct (lines 12-16)
3. Remove `ConfluenceSpace` struct (lines 19-24)
4. Remove `ConfluencePage` struct (lines 27-32)
5. Keep `AuthCredentials` struct (used by generic auth)
6. Rename file to `auth.go` for clarity

**Risk:** low (unused structs)

---

## Step 5: Clean Up Atlassian Interfaces

**Why:** Remove scraper service interfaces, keep auth interfaces

**Depends on:** Step 4

**Validation:** code_compiles

**Creates/Modifies:**
- `C:\development\quaero\internal\interfaces\atlassian.go`

**Actions:**
1. Remove `JiraScraperService` interface (lines 26-39)
2. Remove `ConfluenceScraperService` interface (lines 42-55)
3. Keep `AtlassianAuthService` interface (used by generic auth)
4. Keep `AtlassianExtensionCookie` struct (used by Chrome extension)
5. Keep `AtlassianAuthData` struct (used by generic auth)
6. Remove compatibility aliases `JiraScraper`, `ConfluenceScraper` (lines 118-119)
7. Rename file to `auth.go` for clarity

**Risk:** low (unused interfaces)

---

## Step 6: Update Auth Service Constants

**Why:** Generalize service name constants

**Depends on:** Step 5

**Validation:** code_compiles

**Creates/Modifies:**
- `C:\development\quaero\internal\services\auth\service.go`

**Actions:**
1. Keep `ServiceNameAtlassian` constant (line 17) - still used for Atlassian sites
2. Remove `ServiceNameGitHub` constant (line 18) if unused
3. Update comments to reflect generic auth capability
4. No functional changes to auth service (already generic)

**Risk:** low (constants still used by auth service)

---

## Step 7: Remove Source References from App Initialization

**Why:** Clean up logging of enabled sources

**Depends on:** Step 6

**Validation:** code_compiles

**Creates/Modifies:**
- `C:\development\quaero\internal\app\app.go`

**Actions:**
1. Remove enabled sources logging (lines 182-191)
2. Simplify initialization summary to focus on LLM mode and crawler status
3. Remove references to `cfg.Sources` field

**Risk:** low (cosmetic logging change)

---

## Step 8: Update Example Configuration Files

**Why:** Remove deprecated configuration sections

**Depends on:** Step 7

**Validation:** manual_review

**Creates/Modifies:**
- `C:\development\quaero\deployments\local\quaero.toml`
- `C:\development\quaero\test\config\test-config.toml`
- `C:\development\quaero\deployments\docker\config.offline.example.toml`

**Actions:**
1. Remove `[sources.confluence]` sections
2. Remove `[sources.jira]` sections
3. Remove `[sources.github]` sections
4. Add comments explaining that crawler jobs replace these configurations
5. Update comments to reference job definitions instead

**Risk:** low (example files only)

---

## Step 9: Update Documentation (CLAUDE.md)

**Why:** Remove references to direct API integrations

**Depends on:** Step 8

**Validation:** manual_review

**Creates/Modifies:**
- `C:\development\quaero\CLAUDE.md`
- `C:\development\quaero\AGENTS.md`

**Actions:**
1. Remove "Collectors (ONLY These)" section mentioning Jira/Confluence/GitHub services
2. Update architecture sections to reflect crawler-only approach
3. Update "Adding a New Data Source" to focus on crawler patterns/extractors
4. Update Chrome extension section to emphasize generic auth (not just Atlassian)
5. Update configuration examples
6. Remove references to `internal/services/atlassian/`

**Risk:** low (documentation only)

---

## Step 10: Update README.md

**Why:** Remove configuration examples for deprecated sources

**Depends on:** Step 9

**Validation:** manual_review

**Creates/Modifies:**
- `C:\development\quaero\README.md`

**Actions:**
1. Remove Jira/Confluence/GitHub configuration examples
2. Update "Data Sources" section to focus on crawler jobs
3. Emphasize job definitions as the primary configuration method
4. Update Chrome extension description to be generic (not Atlassian-specific)

**Risk:** low (documentation only)

---

## Step 11: Update Chrome Extension Documentation

**Why:** Clarify that extension is for generic auth, not just Atlassian

**Depends on:** Step 10

**Validation:** manual_review

**Creates/Modifies:**
- `C:\development\quaero\cmd\quaero-chrome-extension\README.md`

**Actions:**
1. Update title/description to emphasize generic auth capability
2. Change "Jira/Confluence" references to "authenticated sites" (generic)
3. Keep examples but clarify they're just examples
4. Update "Domain Validation" section to reflect configurable domains

**Risk:** low (documentation only)

---

## Step 12: Final Integration Test

**Why:** Ensure system works without source-specific code

**Depends on:** Steps 1-11

**Validation:** integration_test_pass

**Creates/Modifies:** none

**Actions:**
1. Delete existing database file
2. Build application: `.\scripts\build.ps1`
3. Start server: `.\scripts\build.ps1 -Run`
4. Verify schema migration creates clean database (no Jira/Confluence tables)
5. Test auth capture via Chrome extension
6. Test crawler job execution
7. Verify document storage and search
8. Check logs for any references to removed components

**Risk:** medium (full system integration test)

---

## Breaking Changes

**Configuration Files:**
- `[sources.confluence]`, `[sources.jira]`, `[sources.github]` sections removed
- Users must migrate to job definitions in `job-definitions/` directory

**Database Schema:**
- Jira/Confluence tables removed (migration handles cleanup)
- No data loss (tables were unused by current crawler)

**No Migration Path:**
- This is acceptable per requirements ("Breaking changes are acceptable", "Do not consider migration")

## Success Criteria

1. ✅ Application compiles without errors
2. ✅ Schema migration removes Jira/Confluence tables
3. ✅ No references to removed interfaces/models in codebase
4. ✅ Chrome extension continues to work for generic auth
5. ✅ Crawler service operates normally
6. ✅ Example configs updated
7. ✅ Documentation reflects crawler-only architecture
8. ✅ Integration tests pass

## Constraints

- Breaking changes acceptable
- No migration of old API-based data needed
- Generic crawler infrastructure remains intact
- Chrome extension remains (generic auth capability)
- Auth storage and service remain (generic)

## Notes

**Metadata Extractors:**
The identifier and metadata extractors in `internal/services/identifiers/` and `internal/services/metadata/` contain Jira/Confluence URL patterns. These are **generic extractors** that work with any URL pattern and should be **kept**. They demonstrate how future page-specific metadata extraction can be implemented as plugins.

**Auth Service:**
The auth service and Chrome extension are **generic** and work with any authenticated site. The "Atlassian" naming is historical but the functionality is not limited to Atlassian products. Consider renaming for clarity in future refactoring.

**Job Definitions:**
The removal of source-specific configuration shifts all data source management to job definitions stored in the `job-definitions/` directory. This is the intended architecture moving forward.
