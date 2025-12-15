Refactor the internal\queue process and workers to use https://github.com/reugn/go-quartz

- Simplify code and structures were possible, however, the core structure of toml configurations, steps (jobs) and dependancies are contolled by toml and orchestration, step initialisation (i.e. preparing work and workers) is separate to workers, workers only execute structured tasks, and can add additional workers to the queue, within configuration params (eg. crawler woeker). Layers are job -> steps -> workers. i.e. workers cannot contain steps. Wokers are unaware of other workers, and have no dependancies. A worker configuration is contained within the worker.
- VERY IMPORTANT. Job/Steps/Worker logging is CRITICAL, it is how the UI is updated. However, the logging is context limited to the service, and the asseibling of the information, as inserted into the database, or websockets or api is context aware to the request, NOT to the log entry. Maintain logging interfaces, preserving log context (Badger), and api requests/UI triggers
- Ensure job cancelation also cancels all steps & workers
- READ docs\architecture, for a view on the current architrecture, and adjust as needed
- Update test\api\jobs_test.go (and other associated api tests)
- Remove redudant code, and breaking changes are ok, there are NO dependancies on api's, or code
