# Step 2: Event batching for WebSocket broadcasts

Model: sonnet | Skill: go | Status: ⏭️ SKIPPED

## Reason
After investigation, the event service `Publish()` method is already async (fires goroutines per handler). The bottleneck may be elsewhere. Adding debug logging first will help identify the actual issue before implementing batching.

## Recommendation
If logs show event publishing is actually blocking workers, implement:
1. Buffered channel in WebSocket handler (capacity ~100 events)
2. Batch flush every 100ms or when buffer reaches 10 events
3. Graceful shutdown to drain buffer
