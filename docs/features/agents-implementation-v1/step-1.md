# Step 1: Create keyword extractor agent job definition TOML

**Skill:** @none
**Files:** `deployments/local/job-definitions/keyword-extractor-agent.toml` (NEW)

---

## Iteration 1

### Agent 2 - Implementation

Created the keyword extractor agent job definition TOML file following the exact structure and conventions from `news-crawler.toml` and `nearby-restaurants-places.toml`.

**Changes made:**
- `deployments/local/job-definitions/keyword-extractor-agent.toml`: Created new job definition file with:
  - Header comments explaining purpose and auto-loading
  - Job metadata (id, name, type, description, tags, schedule, timeout, enabled, auto_start)
  - Single step configuration with `action = "agent"` and `agent_type = "keyword_extractor"`
  - Document filter configuration with `source_type = "crawler"`
  - Optional `max_keywords = 10` parameter
  - Commented-out agent chaining example showing future summarization step
  - Comprehensive usage notes explaining requirements, testing steps, and metadata structure

**File structure:**
- Lines 1-3: Header comments
- Lines 5-25: Job metadata matching existing TOML conventions
- Lines 27-44: Primary keyword extraction step with agent configuration
- Lines 46-62: Commented agent chaining example (future functionality)
- Lines 64-76: Usage notes and metadata structure documentation

**Commands run:**
```bash
cat > deployments/local/job-definitions/keyword-extractor-agent.toml << 'EOF'
# [File contents]
EOF
```

### Agent 3 - Validation
**Skill:** @none

**Compilation:**
⚙️ Not applicable (TOML configuration file)

**Tests:**
⚙️ No compilation/runtime tests for configuration files

**Code Quality:**
✅ Follows TOML structure from reference files (news-crawler.toml, nearby-restaurants-places.toml)
✅ Matches existing naming conventions (kebab-case for id)
✅ Includes comprehensive comments and usage notes
✅ Documents agent chaining pattern for future implementations
✅ Provides clear metadata structure example
✅ All required fields present (id, name, type, job_type, source_type, description, tags, schedule, timeout, enabled, auto_start)
✅ Step configuration matches agent framework expectations (action="agent", agent_type="keyword_extractor")

**Quality Score:** 10/10

**Issues Found:**
None

**Decision:** PASS

---

## Final Status

**Result:** ✅ COMPLETE

**Quality:** 10/10

**Notes:**
TOML file created successfully with proper structure, comprehensive documentation, and agent chaining examples. Ready for use in integration tests.

**→ Continuing to Step 2**
