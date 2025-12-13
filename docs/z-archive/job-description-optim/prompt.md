1. With tyhe refactor of jobs, and introduction of jobs within jobs (old steps), all jobs no longer need a number of config items.

eg. test\config\job-definitions\nearby-restaurants-places.toml

# Maintain / Keep
id = "places-nearby-restaurants" 
name = "Nearby Restaurants (Wheelers Hill)"
description = "Search for restaurants near Wheelers Hill using Google Places Nearby Search API"
schedule = ""
timeout = "5m"
enabled = true
auto_start = false

# No longer required
type = "places" 
job_type = "user" # No longer required

# Optional, update to apply to all jobs, however jobs can also have tags, to which are appended to the document tags
tags = ["Restaurants", "Wheelers Hill", "google-search"]

2. All jobs must comply to the standard configuration

[parent]
# Describes the parent job, and this will be visible in the UI, is the monitor

[step.name1]
# At least one step is required

[step.name2]
depends = "name1" # Optional, and is a comma separated list of step names

eg. test\config\job-definitions\news-crawler.toml is a fail,. as no steps.

work thorugh all configurations and edit.
- test\config\job-definitions
- deployments\local\job-definitions
- bin\job-definitions

3. Run queue tests and iterate to pass