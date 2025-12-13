# Manifest

## User Request

The defaults for crawler need to be created in a TOML. This must include depth and max_pages etc, however as the job is ondemand, when the extension triggers a crawl, the job should search through "crawler" jobs and find a match based upon a list of URLs.

i.e. if a capture request was for ABC news (abc.com.au), it would find bin\job-definitions\news-crawler.toml and use the configuration/rules.

The authentication process must remain.

Actions:
1. Create a TOML config for a Confluence site, which lists include_patterns = [] and exclude_patterns, include in tests, deployments\local\job-definitions, bin\job-definitions\
2. When the job is created/requested, the definition is searched for and found, based upon a list of URLs (wildcard) in the job definition
3. Authentication from the browser (which includes the extension) must be refreshed and used
4. Keep the structure and code as clean and non-contextual i.e. no specific site context in code, this should be in the TOML. Maintain easy/user focused TOML, and apply defaults in code.

## User Intent

Enable Chrome extension quick crawl to reuse existing job definitions based on URL pattern matching, rather than creating ad-hoc configurations. This allows:
- Site-specific crawl rules (include/exclude patterns, depth, max_pages) to be pre-defined in TOML
- URL matching via wildcards to automatically select the right configuration
- Authentication to be refreshed and applied regardless of which job definition is matched
- Clean separation: site-specific config in TOML, defaults in code

## Success Criteria

- [ ] **Confluence TOML**: Create job definition for Confluence sites with appropriate patterns
  - Include `url_patterns` field for URL matching (e.g., `["*.atlassian.net/*"]`)
  - Include sensible `include_patterns`, `exclude_patterns` for Confluence
  - Deploy to bin/job-definitions, deployments/local/job-definitions, test config
- [ ] **URL Matching**: When quick crawl is triggered, search crawler job definitions for matching `url_patterns`
  - Support wildcards in patterns (e.g., `*.atlassian.net/*`)
  - First match wins (or most specific match)
  - If no match, use default ad-hoc job as before
- [ ] **Config Inheritance**: When a matching job definition is found
  - Use its `max_depth`, `max_pages`, `include_patterns`, `exclude_patterns`, etc.
  - Override `start_urls` with the actual URL from the extension
  - Keep the authentication flow unchanged (refresh and store credentials)
- [ ] **Authentication**: Auth must be refreshed and applied to the job
  - Credentials stored with site domain
  - `auth_id` set on job definition or passed through metadata
- [ ] **Tests**: Add tests for URL pattern matching
- [ ] **No Code Hardcoding**: No site-specific URLs or patterns in code; all in TOML
