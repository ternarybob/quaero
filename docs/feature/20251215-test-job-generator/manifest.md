# Feature: Rename error_generator to test_job_generator
Date: 2025-12-15
Request: "1. Change the name of the internal\queue\workers\error_generator_worker.go (both filename and function) to test_job_generator. It is does more the just errors. 2. update all job definitions bin\job-definitions, deployments\local\job-definitions, test\config\job-definitions (and file names, if required) 3. In the test (test\config\job-definitions) definition add multiple generators, increase 1 generator logging to random 1000+, and slow down the 1 generator, to take +2 minutes."

## User Intent
Rename the error_generator worker to test_job_generator since it does more than just error generation. Update all references and job definition files. Enhance the test job definition with multiple generators including a high-volume logger.

## Success Criteria
- [x] Rename internal/queue/workers/error_generator_worker.go to test_job_generator_worker.go
- [x] Rename worker type from "error_generator" to "test_job_generator"
- [x] Update job definitions in bin/job-definitions/
- [x] Update job definitions in test/config/job-definitions/
- [x] Update job definitions in test/bin/job-definitions/
- [x] Update any test files referencing error_generator
- [x] Add multiple generator steps to test job definition
- [x] Add a generator with 1000+ random logs
- [x] Add a slow generator that takes 2+ minutes
- [x] Build passes
- [x] Tests pass

## Applicable Architecture Requirements
| Doc | Section | Requirement |
|-----|---------|-------------|
| workers.md | Worker Interfaces | Workers must implement GetType() returning worker type identifier |
| workers.md | Worker Interfaces | Workers implement DefinitionWorker and/or JobWorker interfaces |
| manager_worker_architecture.md | Workers | Workers implement JobWorker for queue execution and DefinitionWorker for step creation |
| QUEUE_LOGGING.md | Logging Methods | Use AddJobLog variants for job logging |
| QUEUE_SERVICES.md | Worker Registration | Workers must be registered with StepManager and JobProcessor |
