# Fix: Local Dir Config Refactor
- Slug: local-dir-config-refactor | Type: fix | Date: 2025-12-05
- Request: "1. remove [job] from the config, this is redundant as there is only a single job definition in any file. 2. the UI does not require additional multi-step screens, these should be removed and any use case, for multi step editing, should be refactored into existing UI, and kept as simple and technical (i.e. json editing) as possible. Do not build forms/screen/etc for specific use cases, use json/technical/basic editing/saving. 3. Rewrite the tests (API and UI) to match the refactor 4. Execute the tests and iterate to success."
- Prior: none

## User Intent
The user wants to:
1. Remove the redundant `[job]` section from local_dir TOML config - job definitions should use flat top-level fields like existing jobs (id, name, description at top, then [step.xxx] sections)
2. Remove the over-engineered UI additions (dropdown with 3 examples, tab navigation in help section) - the existing simple UI is sufficient
3. Update API and UI tests to use the correct TOML format matching existing job definitions
4. Run tests and fix any failures

## Success Criteria
- [ ] TOML examples in UI use flat format (no [job] section) matching existing job definitions like github-git-collector.toml
- [ ] UI reverted to simple single "Load Example" button (no dropdown, no tabs)
- [ ] API tests use correct TOML format and pass
- [ ] UI tests use correct TOML format and pass
- [ ] All local_dir tests execute successfully
