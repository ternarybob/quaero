# Queue UI Architecture

> **For AI Agents:** This document describes the queue management UI.
> Read this before modifying `pages/queue.html` or related frontend code.

## Overview

The Queue UI provides real-time visibility into job execution using Alpine.js for state management and WebSocket for live updates.

## Component Structure

```
pages/queue.html
├── Alpine.js x-data="jobList"
│   ├── State: jobTreeData, jobTreeExpandedSteps, jobLogs
│   ├── WebSocket: subscriptions for job_update, job_status_change
│   └── Methods: toggleTreeStep, fetchStepLogs, etc.
└── Tree View Rendering
    ├── Manager Jobs (parent level)
    ├── Step Jobs (expandable children)
    └── Log Display (per-step)
```

## State Management

### jobList Component

```javascript
Alpine.data('jobList', () => ({
    // Job tree data (manager → steps hierarchy)
    jobTreeData: {},           // { managerId: { job, steps: [...] } }
    jobTreeExpandedSteps: {},  // { stepId: true/false }
    
    // Logs per step
    jobLogs: {},               // { stepId: [LogEntry, ...] }
    jobLogsLoading: {},        // { stepId: true/false }
    
    // WebSocket connection
    ws: null,
    wsConnected: false,
    
    // Methods
    init() { ... },
    toggleTreeStep(stepId) { ... },
    fetchStepLogs(stepId) { ... },
    handleWebSocketMessage(event) { ... }
}))
```

## WebSocket Events

### Subscribed Events

| Event | Purpose | Handler Action |
|-------|---------|----------------|
| `job_update` | Job metadata changed | Update jobTreeData |
| `job_status_change` | Status changed | Update status, icons |
| `refresh_logs` | Logs should be refetched | Call fetchStepLogs() |
| `queue_stats` | Queue statistics | Update stats display |

### Event Payload Examples

```javascript
// job_status_change
{
    type: "job_status_change",
    payload: {
        job_id: "step123",
        status: "completed",
        manager_id: "mgr456"
    }
}

// refresh_logs
{
    type: "refresh_logs",
    payload: {
        job_id: "step123",
        manager_id: "mgr456"
    }
}
```

## Tree View Hierarchy

```
┌─────────────────────────────────────────────────────────────────┐
│ Manager Job: "Codebase Classify"                    [▶] [🗑️]   │
│   Status: running  |  Progress: 2/3 steps                       │
├─────────────────────────────────────────────────────────────────┤
│   ├── Step: import_files                            [✓]         │
│   │   └── [Logs collapsed - click to expand]                    │
│   ├── Step: code_map                                [⟳]         │
│   │   └── [Logs expanded]                                       │
│   │       1: Starting code map analysis...                      │
│   │       2: Processing file: main.go                           │
│   │       3: Processing file: utils.go                          │
│   └── Step: classify_files                          [○]         │
│       └── [Pending - no logs yet]                               │
└─────────────────────────────────────────────────────────────────┘
```

## Icon Standards

**CRITICAL:** Step icons MUST match parent job icon standards.

### Status Icons

| Status | Icon Class | Description |
|--------|------------|-------------|
| pending | `fa-clock` | Waiting to start |
| running | `fa-spinner fa-spin` | Currently executing |
| completed | `fa-check-circle` | Successfully finished |
| failed | `fa-times-circle` | Error occurred |
| cancelled | `fa-ban` | User cancelled |

### Icon Rendering

```html
<!-- Step status icon -->
<span class="tree-step-status">
    <i :class="getStepIconClass(step.status)"></i>
</span>
```

```javascript
getStepIconClass(status) {
    switch(status) {
        case 'pending': return 'fas fa-clock';
        case 'running': return 'fas fa-spinner fa-spin';
        case 'completed': return 'fas fa-check-circle has-text-success';
        case 'failed': return 'fas fa-times-circle has-text-danger';
        case 'cancelled': return 'fas fa-ban has-text-warning';
        default: return 'fas fa-question-circle';
    }
}
```

## Step Expansion

### Auto-Expand Behavior

**CRITICAL:** ALL steps should auto-expand when they start running.

```javascript
// When step status changes to "running", auto-expand
handleStepStatusChange(stepId, newStatus) {
    if (newStatus === 'running') {
        this.jobTreeExpandedSteps[stepId] = true;
        this.fetchStepLogs(stepId);
    }
}
```

### Manual Toggle

```javascript
toggleTreeStep(stepId) {
    this.jobTreeExpandedSteps[stepId] = !this.jobTreeExpandedSteps[stepId];
    if (this.jobTreeExpandedSteps[stepId]) {
        this.fetchStepLogs(stepId);
    }
}
```

## Log Display

### Fetching Logs

```javascript
async fetchStepLogs(stepId) {
    this.jobLogsLoading[stepId] = true;
    try {
        const response = await fetch(`/api/jobs/${stepId}/logs?limit=100`);
        const data = await response.json();
        this.jobLogs[stepId] = data.logs;
    } finally {
        this.jobLogsLoading[stepId] = false;
    }
}
```

### Log Line Numbering

**CRITICAL:** Log lines MUST start at 1 and increment sequentially.

```html
<template x-for="(log, idx) in jobLogs[stepId]" :key="log.index">
    <div class="log-line">
        <span class="log-line-number" x-text="idx + 1"></span>
        <span class="log-timestamp" x-text="formatTimestamp(log.timestamp)"></span>
        <span class="log-level" :class="'log-' + log.level" x-text="log.level"></span>
        <span class="log-message" x-text="log.message"></span>
    </div>
</template>
```

## API Calls

### Minimize API Calls

**CRITICAL:** Step log API calls should be < 10 per job execution.

**Best Practices:**
1. Fetch logs only when step is expanded
2. Use WebSocket events for incremental updates
3. Batch log fetches when possible
4. Cache logs in `jobLogs` state

### API Endpoints Used

| Endpoint | Purpose |
|----------|---------|
| `GET /api/jobs` | List jobs for tree view |
| `GET /api/jobs/{id}` | Get single job details |
| `GET /api/jobs/{id}/logs` | Get logs for a step |
| `GET /api/jobs/{id}/tree` | Get job tree structure |

## Known Issues

1. **Too Many API Calls:** UI makes excessive API calls for logs
2. **Icon Mismatch:** Step icons don't match parent job icon standard
3. **Auto-Expand:** Not all steps auto-expand when running
4. **Log Numbering:** Some steps don't follow 1 → N sequential pattern

## Related Documents

- **Manager/Worker Architecture:** `docs/architecture/manager_worker_architecture.md`
- **Logging Architecture:** `docs/architecture/QUEUE_LOGGING.md`
- **Workers Reference:** `docs/architecture/workers.md`

