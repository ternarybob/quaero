# Task 1: Revert UI changes in job_add.html
Depends: - | Critical: no | Model: sonnet

## Addresses User Intent
Remove the over-engineered UI additions - the existing simple UI is sufficient

## Do
- Remove CSS for .example-dropdown and .tab-nav
- Revert dropdown back to simple "Load Example" button
- Remove tab navigation from help section
- Keep single help reference (existing crawler format)
- Remove showExampleDropdown, activeTab state variables
- Remove loadCrawlerExample, loadLocalDirExample, loadMultiStepExample functions
- Keep single loadExample function with correct flat TOML format

## Accept
- [ ] Single "Load Example" button (no dropdown)
- [ ] Single help reference section (no tabs)
- [ ] Example uses flat TOML format (no [job] section)
- [ ] Page loads and functions correctly
