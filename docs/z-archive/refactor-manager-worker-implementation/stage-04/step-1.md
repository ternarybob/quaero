# Step 1: Create Merged Crawler Worker File

**Skill:** @code-architect
**Files:** `internal/jobs/worker/crawler_worker.go` (NEW)

---

## Iteration 1

### Agent 2 - Implementation

Created merged crawler_worker.go file by combining crawler_executor.go (1034 lines) and crawler_executor_auth.go (495 lines) into single file (~1529 lines).

**Changes made:**
- `internal/jobs/worker/crawler_worker.go` (NEW) - Merged file with all transformations applied

**Transformations Applied:**
1. **Package**: Changed from `processor` to `worker`
2. **Struct**: Renamed `CrawlerExecutor` to `CrawlerWorker`
3. **Constructor**: Renamed `NewCrawlerExecutor()` to `NewCrawlerWorker()`
4. **Receiver**: Changed from `func (e *CrawlerExecutor)` to `func (w *CrawlerWorker)` for all methods
5. **Method References**: Updated all internal references from `e.` to `w.` throughout all method bodies
6. **Auth Method**: Moved `injectAuthCookies()` from separate file into merged file as private method

**File Organization (5 sections):**
1. **Interface Methods** (lines ~70-478):
   - GetWorkerType() - Returns "crawler_url"
   - Validate() - Validates job config
   - Execute() - Main crawler workflow

2. **Configuration and Rendering Helpers** (lines ~480-810):
   - extractCrawlConfig() - Parse crawl configuration
   - renderPageWithChromeDp() - ChromeDP page rendering with network/log domain enabling

3. **Authentication Helpers** (lines ~812-1109):
   - injectAuthCookies() - Load and inject auth cookies (moved from crawler_executor_auth.go)
   - Conditional logic preserved: checks `if w.authStorage == nil` and returns early
   - Three-phase injection: pre-injection diagnostics, network enablement, post-injection verification
   - All 🔐 emoji logging preserved

4. **Child Job Management** (lines ~1111-1220):
   - spawnChildJob() - Create and enqueue child jobs for discovered links

5. **Real-Time Event Publishing** (lines ~1222-1529):
   - publishCrawlerJobLog() - Job log events
   - publishCrawlerProgressUpdate() - Progress updates
   - publishLinkDiscoveryEvent() - Link discovery statistics
   - publishJobSpawnEvent() - Child job spawn events

**Import Changes:**
- Combined imports from both source files
- Added auth-specific imports: encoding/json, net/url (already aliased as neturl), cdp, network (chromedp protos)
- Removed processor package reference (this is now worker package)

**Preserved Features:**
- All 8 struct dependencies intact
- Conditional authentication logic (authStorage nil check)
- ChromeDP rendering with JavaScript execution
- Content processing and markdown conversion
- Link discovery with pattern filtering
- Child job spawning with depth/page limits
- Real-time event publishing for monitoring
- Comprehensive structured logging

**Commands run:**
```bash
# Compile check for new file
go build -o /tmp/test_crawler_worker internal/jobs/worker/crawler_worker.go
```

### Agent 3 - Validation

**Skill:** @code-architect

**Compilation:**
✅ Compiles cleanly

**Tests:**
⚙️ No tests applicable (file merge only, testing in final step)

**Code Quality:**
✅ Follows Go patterns - receiver methods, error handling, context usage
✅ Matches existing code style - structured logging, error wrapping
✅ Proper error handling - all errors checked and wrapped with context
✅ Single Responsibility - each method has clear purpose
✅ Clear section organization - 5 logical sections with comment dividers
✅ All 8 dependencies preserved - no missing fields
✅ Conditional auth logic intact - nil check at correct location
✅ All transformations applied correctly - package, struct, constructor, receiver
✅ Imports combined properly - all necessary imports present

**Quality Score:** 9/10

**Issues Found:**
None - merge completed successfully with all features preserved

**Decision:** PASS

---

## Final Status

**Result:** ✅ COMPLETE

**Quality:** 9/10

**Notes:**
File merge completed successfully. The new crawler_worker.go file combines both source files with all transformations applied correctly. Auth logic remains conditional (authStorage nil check). File is well-organized into 5 logical sections. All features preserved including ChromeDP rendering, authentication, link discovery, child job spawning, and event publishing.

**→ Continuing to Step 2**
