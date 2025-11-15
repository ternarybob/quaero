# Step 1: Replace page-container with container in all HTML files

**Skill:** @none
**Files:** `pages/*.html` (9 files)

---

## Iteration 1

### Agent 2 - Implementation

Replacing `class="page-container"` with `class="container"` in all HTML template files.

**Files to update:**
1. pages/settings.html
2. pages/queue.html
3. pages/chat.html
4. pages/documents.html
5. pages/job_add.html
6. pages/job.html
7. pages/search.html
8. pages/index.html
9. pages/config.html

**Implementation:**

Successfully replaced `class="page-container"` with `class="container"` in all 9 HTML template files:

1. **pages/settings.html** (line 15)
   - Changed: `<main class="page-container">` → `<main class="container">`

2. **pages/chat.html** (line 13)
   - Changed: `<main class="page-container">` → `<main class="container">`

3. **pages/documents.html** (line 13)
   - Changed: `<main class="page-container">` → `<main class="container">`

4. **pages/job_add.html** (line 70)
   - Changed: `<main class="page-container" x-data="jobAddPage()">` → `<main class="container" x-data="jobAddPage()">`
   - Note: Preserved Alpine.js data binding

5. **pages/queue.html** (line 13)
   - Changed: `<main class="page-container">` → `<main class="container">`

6. **pages/job.html** (line 13)
   - Changed: `<main class="page-container" x-data="jobDetailPage">` → `<main class="container" x-data="jobDetailPage">`
   - Note: Preserved Alpine.js data binding

7. **pages/search.html** (line 17)
   - Changed: `<main class="page-container">` → `<main class="container">`

8. **pages/index.html** (line 13)
   - Changed: `<main class="page-container">` → `<main class="container">`

9. **pages/config.html** (line 13)
   - Changed: `<main class="page-container">` → `<main class="container">`

### Agent 3 - Validation

**Validation Checks:**
✅ All HTML files maintain valid syntax
✅ All `x-data` attributes preserved correctly
✅ All Go template directives remain functional
✅ All 9 files updated consistently
✅ No unintended changes to content or structure

**Files Verified:**
- settings.html ✅
- chat.html ✅
- documents.html ✅
- job_add.html ✅
- queue.html ✅
- job.html ✅
- search.html ✅
- index.html ✅
- config.html ✅

**Status:** ✅ COMPLETE
**Quality Score:** 10/10

All instances of `class="page-container"` successfully replaced with `class="container"`. No syntax errors introduced. Ready for Step 2.
