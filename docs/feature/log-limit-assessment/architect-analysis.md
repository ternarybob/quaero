# Architect Analysis: Log Limit Removal Assessment

## Current Implementation

**Location:** `pages/queue.html`
- Line 5046: `defaultLogsPerStep: 100`
- Line 5086-5090: Limits logs to `stepLogLimit` during running jobs
- Line 5153-5156: "Show earlier logs" increases limit by 100 per click

**Behavior:**
- Running jobs: Show last 100 logs (most recent)
- Completed jobs: Show ALL logs (no limit)
- "Show earlier logs" button: Increases limit by 100

## Log Volume from test_job_generator.toml

| Step | Workers | Logs/Worker | Total |
|------|---------|-------------|-------|
| fast_generator | 5 | 50 | 250 |
| high_volume_generator | 3 | 1200 | 3,600 |
| slow_generator | 2 | 300 | 600 |
| recursive_generator | 3 | 20 | 60 (+children) |
| **TOTAL** | | | **~4,510+** |

## DOM Impact Analysis

Each log line renders 4 elements:
```html
<div class="tree-log-line">           <!-- 1 -->
    <span class="tree-log-num">       <!-- 2 -->
    <span class="level-badge">        <!-- 3 -->
    <span class="tree-log-text">      <!-- 4 -->
</div>
```

**DOM Elements per Step:**
| Logs | DOM Elements | Memory (est.) |
|------|--------------|---------------|
| 100 | 400 | ~40KB |
| 1,000 | 4,000 | ~400KB |
| 3,600 | 14,400 | ~1.4MB |
| 4,510 | 18,040 | ~1.8MB |

**With 4 steps expanded:** ~72,000 DOM elements, ~7MB

## Browser Performance Considerations

1. **DOM Render Time:**
   - 100 elements: <10ms
   - 1,000 elements: ~100ms
   - 10,000+ elements: 500ms-2s (janky scroll)

2. **Memory:**
   - Modern browsers handle 7MB easily
   - But Alpine.js reactivity overhead adds ~2x

3. **Scroll Performance:**
   - 10,000+ items: Stuttering possible
   - Chrome/Firefox: OK up to ~20,000 elements
   - Safari: Struggles above ~10,000

4. **Real-time Updates:**
   - Adding 1 log to 10,000-item list: Full diff = slow
   - Current batching helps but still costly

## Current Test Coverage

**File:** `test/ui/job_definition_test_generator_test.go`
- Uses `TriggerJob("Test Job Generator")`
- Runs `test/config/job-definitions/test_job_generator.toml`
- Monitors job completion
- **Does NOT** measure DOM performance/memory

## Recommendation

**DO NOT remove the log limit entirely.** Instead:

### Option A: Increase Default Limit (RECOMMENDED)
- Increase `defaultLogsPerStep` from 100 to 500
- High-volume step (3,600 logs) still needs "Show earlier" but only 7 clicks
- DOM stays manageable (~8,000 elements with 4 steps)

### Option B: Virtual Scrolling
- Render only visible logs (~50 at a time)
- Requires significant Alpine.js changes
- Best for 10,000+ logs but complex

### Option C: Remove Limit (NOT RECOMMENDED)
- Page will become unresponsive with high_volume_generator (3,600 logs)
- Scroll performance will degrade
- Real-time updates will lag

## Test Enhancement Needed

The test should verify:
1. Page remains responsive with high log volume
2. Scroll performance is acceptable
3. Memory doesn't exceed threshold

## Files to Modify

| File | Change |
|------|--------|
| `pages/queue.html` | Increase `defaultLogsPerStep` to 500 |
| `test/ui/job_definition_test_generator_test.go` | Add DOM performance assertions |
