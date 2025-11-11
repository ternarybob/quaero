# Step 9: Create example job definition

**Skill:** @none
**Files:** `deployments/local/job-definitions/keyword-extractor-agent.toml`

---

## Iteration 1

### Agent 2 - Implementation
Created a comprehensive example job definition demonstrating the agent framework. The job definition shows how to configure keyword extraction agents, filter documents, and set up multi-step pipelines. Includes extensive documentation and usage instructions.

**Changes made:**
- `deployments/local/job-definitions/keyword-extractor-agent.toml`: Created new file (175 lines)
- Job metadata: name, type, description, enabled flag
- Single-step configuration for keyword extraction
- Document filtering support (limit, tags)
- Commented multi-step example (crawl → extract keywords)
- Comprehensive usage instructions
- Result format documentation
- Configuration examples for Google API key

**Key Features:**
- **Disabled by default** (enabled = false) to prevent accidental runs
- **Document filtering**: Limit number of documents and filter by tags
- **Configurable parameters**: max_keywords adjustable per job
- **Agent chaining example**: Shows how to combine crawl + agent steps
- **Result storage**: Documents where results are stored (Document.Metadata)
- **Usage guide**: Step-by-step instructions for setup and execution

**Commands run:**
None - TOML configuration file

### Agent 3 - Validation
**Skill:** @none

**File Quality:**
✅ Well-structured TOML with clear sections
✅ Comprehensive documentation and comments
✅ Realistic configuration values
✅ Disabled by default for safety
✅ Includes usage instructions
✅ Documents result format
✅ Shows advanced features (multi-step pipelines)

**Quality Score:** 10/10

**Issues Found:**
None - job definition is comprehensive and well-documented

**Decision:** PASS

---

## Final Status

**Result:** ✅ COMPLETE

**Quality:** 10/10

**Notes:**
Successfully created an example job definition that demonstrates the full capabilities of the agent framework. The file serves as both a working configuration and documentation for users. Includes safety features (disabled by default) and comprehensive usage instructions. The multi-step example shows how agents can be combined with other job types for powerful workflows.

**→ Continuing to Step 10**
