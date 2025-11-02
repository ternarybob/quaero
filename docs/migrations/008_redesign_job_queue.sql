-- Migration 008: Redesign Job Queue
-- Drop old tables if they exist from previous attempts
DROP TABLE IF EXISTS queue_messages;
DROP TABLE IF EXISTS queue_state;
DROP TABLE IF EXISTS job_queue;

-- Jobs metadata table
-- This is NOT the queue - goqite manages the queue
-- This table stores job metadata and relationships
CREATE TABLE IF NOT EXISTS jobs (
    id TEXT PRIMARY KEY,
    parent_id TEXT REFERENCES jobs(id) ON DELETE CASCADE,
    job_type TEXT NOT NULL,
    phase TEXT NOT NULL CHECK (phase IN ('pre', 'core', 'post')),
    status TEXT NOT NULL CHECK (status IN ('pending', 'running', 'completed', 'failed', 'cancelled')),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    payload TEXT,  -- JSON payload
    result TEXT,   -- JSON result
    error TEXT,
    progress_current INTEGER DEFAULT 0,
    progress_total INTEGER DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_jobs_parent ON jobs(parent_id);
CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status);
CREATE INDEX IF NOT EXISTS idx_jobs_created ON jobs(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_jobs_type ON jobs(job_type);

-- Job logs table
CREATE TABLE IF NOT EXISTS job_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    job_id TEXT NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    level TEXT NOT NULL CHECK (level IN ('debug', 'info', 'warn', 'error')),
    message TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_job_logs_job ON job_logs(job_id, timestamp);
CREATE INDEX IF NOT EXISTS idx_job_logs_timestamp ON job_logs(timestamp DESC);

-- goqite will create its own table (goqite_jobs) - we don't touch it
