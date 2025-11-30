problem:
- the name stepworker and step manager is previlent in much o the queue and interface code 
- this is consusing and not correct.

actions:
1. the concept of steps, is not needed. The job manager (internal\queue\manager.go) accepts a job confirgutation, reviews and then executes the required workers (jobs). the workers are a queued job and are independant units of work. workers, can if requires, execute a child worker (eg. crawlers). The queue and spawned chilredn are observed by the monitor (internal\queue\state\monitor.go). The workers send events to the monitor, whci enables logging and UI reporting
2. Workers should implement the a worker interface, hwih cenables the manager to execute the corrtect type of work, however execute the same functions in the worker.

