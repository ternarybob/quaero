# Step 1: Remove queue stats displays from queue.html

Model: sonnet | Skill: frontend | Status: ✅

## Done
1. Removed step-level progress stats template (lines 191-198)
2. Removed job-level "Progress:" display block (lines 471-485)
3. Removed unused stepProgress computation code (lines 2545-2568)
4. Removed `progress: stepProgress` property from step item

## Files Changed
- `pages/queue.html` - Removed 50 lines of inaccurate queue stats display

## Details
- **Step-level**: Removed template showing "X pending, Y running, Z completed, W failed" on each step card
- **Job-level**: Removed "Progress:" section that showed global queue counts on parent job cards
- **Dead code**: Removed stepProgress variable computation that was no longer used after display removal

## Skill Compliance (frontend)
- [x] Alpine.js template modification
- [x] Clean removal without breaking surrounding structure
- [x] Comments explain rationale
- [x] Removed dead code (stepProgress computation)

## Build Check
Build: ✅ | Tests: ⏭️ (HTML only)
