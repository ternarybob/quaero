# Task 3: Update queue.html UI to format logs with text level tags

Depends: 2 | Critical: no | Model: sonnet

## Addresses User Intent

Update log format to `[time] [level] [context] message` with `[INF]`, `[DBG]` etc. tags - User Intent #3, #4, #5

## Do

1. Update `pages/queue.html` step events panel log rendering:
   - Change level display from emoji to text tags: `[INF]`, `[WRN]`, `[ERR]`, `[DBG]`
   - Ensure format is: `HH:MM:SS [LVL] [originator] message`
   - Apply Service Logs color scheme for level tags
   - Maintain white/transparent background
2. Update CSS styling if needed for level tag colors:
   - INFO: blue or default
   - WARN: yellow/orange
   - ERROR: red
   - DEBUG: gray
3. Ensure `[step]` and `[worker]` context shows correctly

## Accept

- [ ] Log entries show format: `HH:MM:SS [INF] [step] message`
- [ ] Level tags are text-based, not emoji
- [ ] Colors match Service Logs panel styling
- [ ] Background remains white/transparent
- [ ] Context tags `[step]` and `[worker]` display correctly
