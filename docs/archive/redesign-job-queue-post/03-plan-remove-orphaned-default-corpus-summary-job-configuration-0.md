I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

The warning `"Job setting found but job not registered, skipping"` occurs because `default-corpus-summary` has an orphaned entry in the `job_settings` table, but it's actually a **job definition** (stored in `job_definitions` table), not a hardcoded scheduler job.

**Root Cause Analysis:**

Quaero has two distinct job management systems:

1. **Hardcoded Scheduler Jobs** (legacy system)
   - Registered directly in code via `RegisterJob()` method
   - Settings persisted in `job_settings` table
   - Loaded via `LoadJobSettings()` method (line 723 in scheduler_service.go)
   - Examples: None currently active (legacy scheduled tasks disabled when job definitions exist)

2. **Job Definitions** (database-driven system)
   - Stored in `job_definitions` table with full configuration
   - Loaded via `LoadJobDefinitions()` method (line 809 in scheduler_service.go)
   - Examples: `default-database-maintenance`, `default-corpus-summary`
   - Created by `CreateDefaultJobDefinitions()` in job_definition_storage.go

**The Problem:**
- `default-corpus-summary` is defined as a job definition (lines 533-638 in job_definition_storage.go)
- But an orphaned entry exists in `job_settings` table (likely from a previous implementation or testing)
- `LoadJobSettings()` finds this entry but the job is not registered as a hardcoded job, triggering the warning

**Why This Happened:**
The `default-corpus-summary` job was likely created as a hardcoded job initially, then migrated to the job definitions system, but the `job_settings` entry was never cleaned up.

**Impact:**
- Non-critical: Just a warning, doesn't affect functionality
- Job definitions work correctly via their own persistence layer
- The orphaned entry serves no purpose and causes confusion

### Approach

Implement a **database migration** to clean up the orphaned `job_settings` entry for `default-corpus-summary`. This is the cleanest solution because:

1. **Automatic cleanup** - Runs once during schema initialization
2. **Idempotent** - Safe to run multiple times (checks before deleting)
3. **Documented** - Migration name explains the purpose
4. **No manual intervention** - Users don't need to run SQL commands

Additionally, enhance documentation in the TOML configuration file to clarify the distinction between the two job systems and prevent future confusion.

### Reasoning

Analyzed the log file showing the warning at line 94, explored the scheduler service code to understand `LoadJobSettings()` and `LoadJobDefinitions()` methods, searched for references to `default-corpus-summary` and found it defined in job_definition_storage.go, examined the database schema to understand the two separate tables (`job_settings` vs `job_definitions`), and reviewed the app initialization flow to confirm how jobs are registered and loaded.

## Proposed File Changes

### internal\storage\sqlite\schema.go(MODIFY)

References: 

- internal\storage\sqlite\job_definition_storage.go
- internal\services\scheduler\scheduler_service.go(MODIFY)

**Add a new migration to clean up orphaned job_settings entries:**

1. **Create migration method** `migrateCleanupOrphanedJobSettings()` after the existing migration methods (after line 405, before the helper functions section):
   - Check if `job_settings` table exists using `PRAGMA table_info(job_settings)`
   - If table doesn't exist, return nil (migration not needed)
   - Query `job_settings` table for entries that match job definition IDs: `SELECT job_name FROM job_settings WHERE job_name IN ('default-corpus-summary', 'default-database-maintenance')`
   - For each matching entry, delete it: `DELETE FROM job_settings WHERE job_name = ?`
   - Log the cleanup action with count of deleted entries
   - Return nil on success

2. **Register migration in `runMigrations()` method** (after line 404, before the final `return nil`):
   - Add comment: `// MIGRATION 23: Cleanup orphaned job_settings entries for job definitions`
   - Add comment: `// Job definitions (default-corpus-summary, default-database-maintenance) should not have entries in job_settings`
   - Add comment: `// The job_settings table is only for hardcoded scheduler jobs, not database-driven job definitions`
   - Call: `if err := s.migrateCleanupOrphanedJobSettings(); err != nil { return err }`

**Rationale:** This migration ensures clean separation between the two job systems. Job definitions have their own persistence layer in `job_definitions` table and should never have entries in `job_settings`. The migration is idempotent and safe to run multiple times.

### deployments\local\quaero.toml(MODIFY)

References: 

- internal\storage\sqlite\schema.go(MODIFY)
- internal\services\scheduler\scheduler_service.go(MODIFY)

**Enhance documentation to clarify the two job systems:**

1. **Update the "Default Jobs Configuration" section header** (lines 120-136):
   - Change section title to: `# Default Jobs Configuration (Hardcoded Scheduler Jobs)`
   - Add clarification comment after line 121:
     ```
     # NOTE: This section is for HARDCODED scheduler jobs only.
     # Database-driven job definitions (like default-corpus-summary, default-database-maintenance)
     # are managed via the Job Definitions UI and stored in the job_definitions table.
     # They should NOT be configured in this TOML file.
     ```
   - Add explanation of the two systems:
     ```
     # Quaero has two job management systems:
     # 1. Hardcoded Jobs (this section) - Defined in code, settings persisted in job_settings table
     # 2. Job Definitions (UI-managed) - Stored in job_definitions table, fully configurable via UI
     ```

2. **Add a new section** after the Default Jobs Configuration section (after line 148):
   - Title: `# =============================================================================`
   - Title: `# Job Definitions (Database-Driven Jobs)`
   - Title: `# =============================================================================`
   - Add explanation:
     ```
     # Job definitions are managed via the Job Definitions UI (not this config file).
     # They are stored in the job_definitions database table and fully configurable.
     #
     # Default job definitions shipped with Quaero:
     # - default-database-maintenance: Weekly FTS5 index rebuild (Sundays at 2:00 AM)
     # - default-corpus-summary: Hourly corpus statistics generation
     #
     # These jobs can be enabled/disabled, scheduled, and configured via the UI.
     # Do NOT add job_settings entries for job definitions - they have their own persistence.
     ```

**Rationale:** Clear documentation prevents users from manually adding job definition entries to `job_settings` table or trying to configure job definitions in the TOML file. This reduces confusion and prevents future orphaned entries.

### internal\services\scheduler\scheduler_service.go(MODIFY)

References: 

- internal\storage\sqlite\schema.go(MODIFY)

**Enhance the warning message to be more informative:**

1. **Update the warning log at line 754** in the `LoadJobSettings()` method:
   - Change from: `s.logger.Warn().Str("job_name", name).Msg("Job setting found but job not registered, skipping")`
   - Change to: `s.logger.Warn().Str("job_name", name).Msg("Job setting found but job not registered (may be a job definition, not a hardcoded job) - skipping")`
   - Add additional debug logging after the warning:
     ```
     s.logger.Debug().
         Str("job_name", name).
         Msg("Hint: Job definitions should not have entries in job_settings table. Check if this is a job definition that needs cleanup.")
     ```

**Rationale:** The enhanced warning message helps developers understand the distinction between hardcoded jobs and job definitions. The debug hint provides actionable guidance if this warning appears again in the future. This is a defensive improvement that makes the system more maintainable.