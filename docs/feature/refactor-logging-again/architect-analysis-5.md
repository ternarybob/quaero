# ARCHITECT Analysis - Real-Time Log Updates Not Working

## Issue

User reports: "Logs NEVER update in real time without hard page refresh."

## Full Data Flow Traced

### 1. Worker Logs → Event Publication

```
test_job_generator_worker.Execute()
  → w.jobMgr.AddJobLog(ctx, job.ID, level, message)
    → AddJobLogFull(ctx, jobID, level, message, ...)
      → resolveJobHierarchy() - gets managerID, stepName, stepID from job metadata
      → eventService.Publish(EventJobLog, payload{
          job_id: workerJobID,
          manager_id: managerJobID,  ← Critical for routing
          step_name: "fast_generator",
          ...
        })
```

**Status**: ✅ Events are published with correct payload including `manager_id`.

### 2. Event → SSE Handler

```
SSELogsHandler.handleJobLogEvent()
  → Extract managerID from payload
  → matchingJobIDs = [workerJobID, managerID]
  → For each matchJobID:
      → subs = h.jobSubs[matchJobID]  ← Should find manager subscriber
      → For each subscriber:
          → sub.logs <- entry
```

**Status**: ✅ Handler routes to subscribers matching `managerID`.

### 3. SSE Handler → Browser

```
streamJobLogs() event loop:
  → case log := <-sub.logs:
      → logBatch = append(logBatch, log)
  → case <-batchTicker.C:
      → sendJobLogBatch(logs) → SSE "logs" event
```

**Status**: ✅ Event loop sends logs via SSE.

### 4. Browser → UI Update

```
QueueSSEManager.connectJob(managerJobId, {
  onLogs: (data) => self.handleSSELogs(jobId, data)
})

handleSSELogs():
  → Group logs by step_name
  → Find step in treeData
  → Merge logs into step.logs
  → Trigger Alpine reactivity: this.jobTreeData = {...}
```

**Status**: ⚠️ Need to verify if events reach browser.

## Hypothesis

The code paths look correct. The issue is likely one of:

1. **SSE events not being sent** - Backend receives event but doesn't route to subscriber
2. **SSE connection not established** - Frontend thinks it's connected but isn't
3. **SSE events not reaching handler** - EventSource configured incorrectly
4. **Step name mismatch** - Logs have step_name that doesn't match tree steps

## Debug Strategy

Add server-side logging to trace:
1. When events are received by `handleJobLogEvent`
2. Whether subscribers are found for `managerID`
3. Whether logs are sent via `sendJobLogBatch`

## Key Files

- `internal/handlers/sse_logs_handler.go` - SSE streaming
- `internal/queue/job_manager.go` - Event publishing
- `pages/static/js/log-stream.js` - Frontend SSE client
- `pages/queue.html` - UI update logic
