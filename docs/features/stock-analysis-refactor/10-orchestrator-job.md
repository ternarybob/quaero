# ASX Portfolio Intelligence System — Orchestrator Job Configuration

## Document Purpose
This document defines the job configuration for the daily portfolio analysis pipeline. It specifies scheduling, step dependencies, and execution parameters.

---

## Job Overview

```
JOB: smsf-portfolio-daily
PURPOSE: Generate daily portfolio analysis report
SCHEDULE: 7:00 AM AEDT, Monday-Friday
TIMEOUT: 2 hours
OUTPUT: Email report to configured recipients
```

---

## Job Configuration Schema

```toml
# =============================================================================
# DAILY PORTFOLIO ANALYSIS JOB
# =============================================================================

[job]
id = "smsf-portfolio-daily"
description = "Daily portfolio intelligence report generation"
schedule = "0 7 * * 1-5"  # Cron: 7am weekdays
timezone = "Australia/Melbourne"
timeout = "2h"
retry_policy = "fail_fast"  # or "retry_once"

# =============================================================================
# PORTFOLIO CONFIGURATION
# =============================================================================

[portfolio]
source = "manual"  # or "navexa"
config_path = "configs/portfolio.toml"

# Holdings configuration (if source = manual)
[[portfolio.holdings]]
ticker = "APA"
name = "APA Group"
sector = "infrastructure"
units = 2500
avg_price = 9.30
target_weight_pct = 6.0
holding_type = "smsf"

[[portfolio.holdings]]
ticker = "DOW"
name = "Downer EDI"
sector = "infrastructure"
units = 4000
avg_price = 7.98
target_weight_pct = 5.0
holding_type = "smsf"

# ... additional holdings ...

# =============================================================================
# PIPELINE STEPS
# =============================================================================

[steps]

# -----------------------------------------------------------------------------
# STAGE 0: Load Portfolio
# -----------------------------------------------------------------------------
[steps.load_portfolio]
tool = "portfolio_load"
inputs = { source = "manual", config_path = "${portfolio.config_path}" }
outputs = ["portfolio-state"]
timeout = "30s"
on_failure = "abort"

# -----------------------------------------------------------------------------
# STAGE 1: Fetch Data (Parallel)
# -----------------------------------------------------------------------------
[steps.fetch_market_data]
tool = "eodhd_fetch"
type = "parallel"
for_each = "${portfolio-state.holdings[*].ticker}"
inputs = { 
  ticker = "${item}.AU",
  lookback_days = 252,
  include_fundamentals = true
}
outputs = ["ticker-raw-${item}"]
timeout = "60s"
on_failure = "skip_item"  # Continue with other tickers

[steps.fetch_announcements]
tool = "asx_announcements_fetch"
type = "parallel"
for_each = "${portfolio-state.holdings[*].ticker}"
inputs = {
  ticker = "${item}",
  days_back = 30,
  include_body_summary = true
}
outputs = ["ticker-announcements-${item}"]
timeout = "60s"
on_failure = "skip_item"
depends_on = []  # Can run parallel with market data

# -----------------------------------------------------------------------------
# STAGE 2: Compute Signals
# -----------------------------------------------------------------------------
[steps.compute_signals]
tool = "compute_signals"
type = "parallel"
for_each = "${portfolio-state.holdings[*].ticker}"
inputs = {
  ticker = "${item}",
  raw_data_tag = "ticker-raw-${item}",
  announcements_tag = "ticker-announcements-${item}"
}
outputs = ["ticker-signals-${item}"]
depends_on = ["fetch_market_data", "fetch_announcements"]
timeout = "30s"
on_failure = "skip_item"

# -----------------------------------------------------------------------------
# STAGE 3: AI Assessment (Batched)
# -----------------------------------------------------------------------------
[steps.ai_assessment]
tool = "ai_assess_batch"
type = "batched"
batch_size = 5
for_each = "${portfolio-state.holdings[*].ticker}"
inputs = {
  tickers = "${batch}",
  signal_tags = ["ticker-signals-${item}" for item in batch],
  holding_types = "${portfolio-state.holding_types_map}",
  thinking_budget = 16000
}
outputs = ["ticker-assessment-${item}" for item in batch]
depends_on = ["compute_signals"]
timeout = "3m"  # Per batch
on_failure = "retry_once"

# -----------------------------------------------------------------------------
# STAGE 4: Portfolio Rollup
# -----------------------------------------------------------------------------
[steps.portfolio_rollup]
tool = "portfolio_rollup"
inputs = {
  portfolio_state_tag = "portfolio-state",
  assessment_tags = ["ticker-assessment-${ticker}" for ticker in holdings]
}
outputs = ["portfolio-rollup"]
depends_on = ["ai_assessment"]
timeout = "30s"
on_failure = "abort"

# -----------------------------------------------------------------------------
# STAGE 5: Assemble Report
# -----------------------------------------------------------------------------
[steps.assemble_report]
tool = "assemble_report"
inputs = {
  portfolio_state_tag = "portfolio-state",
  portfolio_rollup_tag = "portfolio-rollup",
  assessment_tags = "${all_assessment_tags}",
  signal_tags = "${all_signal_tags}",
  include_screening = false
}
outputs = ["daily-report"]
depends_on = ["portfolio_rollup"]
timeout = "60s"
on_failure = "abort"

# -----------------------------------------------------------------------------
# STAGE 6: Email Delivery
# -----------------------------------------------------------------------------
[steps.send_email]
tool = "send_email"
inputs = {
  to = "${config.email_recipients}",
  subject = "SMSF Portfolio Report - ${date}",
  body_from_tag = "daily-report",
  format = "text"
}
depends_on = ["assemble_report"]
timeout = "30s"
on_failure = "retry_once"

# =============================================================================
# CONFIGURATION
# =============================================================================

[config]
email_recipients = ["bobmcallan@gmail.com"]
strategy_path = "configs/strategy.toml"

[config.data_sources]
eodhd_api_key = "${env.EODHD_API_KEY}"

[config.ai]
model = "claude-sonnet-4-20250514"
thinking_budget = 16000
temperature = 0.3

[config.cache]
enabled = true
ttl_hours = 24

# =============================================================================
# NOTIFICATIONS
# =============================================================================

[notifications]
on_success = ["email"]
on_failure = ["email", "log"]

[notifications.email]
to = ["bobmcallan@gmail.com"]
subject_prefix = "[SMSF Pipeline]"
```

---

## Pipeline Execution Flow

```
EXECUTE_PIPELINE(job_config):

  # Initialize
  start_time = now()
  tag_store = create_tag_store()
  
  # Stage 0: Load Portfolio
  log_info("Stage 0: Loading portfolio")
  portfolio_result = execute_step(job_config.steps.load_portfolio)
  IF portfolio_result.failed:
    abort_pipeline("Portfolio load failed")
  tag_store.set("portfolio-state", portfolio_result.output)
  
  # Extract ticker list
  tickers = portfolio_result.output.holdings.map(h => h.ticker)
  
  # Stage 1: Fetch Data (Parallel)
  log_info("Stage 1: Fetching market data for {count(tickers)} tickers")
  
  fetch_tasks = []
  FOR each ticker IN tickers:
    fetch_tasks.append(async execute_step(fetch_market_data, ticker))
    fetch_tasks.append(async execute_step(fetch_announcements, ticker))
  
  fetch_results = await_all(fetch_tasks)
  
  successful_tickers = []
  FOR each result IN fetch_results:
    IF result.success:
      tag_store.set(result.tag, result.output)
      successful_tickers.append(result.ticker)
    ELSE:
      log_warning("Data fetch failed for {ticker}: {error}")
  
  # Stage 2: Compute Signals
  log_info("Stage 2: Computing signals for {count(successful_tickers)} tickers")
  
  signal_tasks = []
  FOR each ticker IN successful_tickers:
    signal_tasks.append(async execute_step(compute_signals, ticker))
  
  signal_results = await_all(signal_tasks)
  
  signal_tickers = []
  FOR each result IN signal_results:
    IF result.success:
      tag_store.set(result.tag, result.output)
      signal_tickers.append(result.ticker)
  
  # Stage 3: AI Assessment (Batched)
  log_info("Stage 3: AI assessment for {count(signal_tickers)} tickers")
  
  batches = chunk(signal_tickers, batch_size=5)
  all_assessments = []
  
  FOR batch_num, batch IN enumerate(batches):
    log_info("Processing batch {batch_num + 1}/{count(batches)}")
    
    batch_result = execute_step(ai_assessment, batch)
    
    IF batch_result.success:
      FOR each assessment IN batch_result.outputs:
        tag_store.set(assessment.tag, assessment.output)
        all_assessments.append(assessment)
    ELSE:
      log_error("Batch {batch_num} failed: {error}")
      # Retry once
      retry_result = execute_step(ai_assessment, batch, retry=true)
      IF retry_result.success:
        all_assessments.extend(retry_result.outputs)
    
    # Rate limiting between batches
    wait(1 second)
  
  # Stage 4: Portfolio Rollup
  log_info("Stage 4: Computing portfolio rollup")
  
  rollup_result = execute_step(portfolio_rollup, {
    portfolio_state: tag_store.get("portfolio-state"),
    assessments: all_assessments
  })
  
  IF rollup_result.failed:
    abort_pipeline("Rollup failed: {error}")
  
  tag_store.set("portfolio-rollup", rollup_result.output)
  
  # Stage 5: Assemble Report
  log_info("Stage 5: Assembling report")
  
  report_result = execute_step(assemble_report, {
    all_tags: tag_store.list("*")
  })
  
  IF report_result.failed:
    abort_pipeline("Report assembly failed: {error}")
  
  tag_store.set("daily-report", report_result.output)
  
  # Stage 6: Send Email
  log_info("Stage 6: Sending email")
  
  email_result = execute_step(send_email, {
    report: tag_store.get("daily-report")
  })
  
  IF email_result.failed:
    log_error("Email failed: {error}")
    # Retry once
    retry_result = execute_step(send_email, retry=true)
    IF retry_result.failed:
      log_error("Email retry failed - report saved but not sent")
  
  # Complete
  duration = now() - start_time
  log_info("Pipeline complete in {duration}")
  
  RETURN PipelineResult(
    success = true,
    duration = duration,
    tickers_processed = count(signal_tickers),
    tickers_failed = count(tickers) - count(signal_tickers),
    report_tag = "daily-report"
  )
```

---

## Step Types

### Sequential Step
```
SEQUENTIAL:
  - Executes one at a time
  - Waits for completion before next step
  - Default type
```

### Parallel Step
```
PARALLEL:
  - Executes multiple instances concurrently
  - for_each defines iteration variable
  - Collects all results before proceeding
  - Respects rate limits
```

### Batched Step
```
BATCHED:
  - Groups items into batches
  - Processes batches sequentially
  - Items within batch may be parallel
  - Used for AI assessment to manage context
```

---

## Failure Handling

```
ON_FAILURE OPTIONS:

  "abort":
    - Stop entire pipeline
    - Use for critical steps (portfolio load, rollup)
  
  "skip_item":
    - Continue without failed item
    - Use for per-ticker operations
    - Log warning and proceed
  
  "retry_once":
    - Retry the step once
    - If retry fails, treat as skip_item or abort based on step
  
  "fallback_cache":
    - Use cached data if available
    - Only for data fetch steps
```

---

## Scheduling

```
SCHEDULE SYNTAX: Cron format

  ┌───────────── minute (0 - 59)
  │ ┌───────────── hour (0 - 23)
  │ │ ┌───────────── day of month (1 - 31)
  │ │ │ ┌───────────── month (1 - 12)
  │ │ │ │ ┌───────────── day of week (0 - 6, Sun-Sat)
  │ │ │ │ │
  * * * * *

EXAMPLES:
  "0 7 * * 1-5"    # 7am weekdays
  "0 7 * * *"      # 7am every day
  "0 */4 * * *"    # Every 4 hours
  "0 7,19 * * *"   # 7am and 7pm
```

---

## Environment Variables

```
REQUIRED:
  EODHD_API_KEY      # API key for market data
  SMTP_HOST          # Email server
  SMTP_USER          # Email username
  SMTP_PASSWORD      # Email password
  
OPTIONAL:
  NAVEXA_API_KEY     # If using Navexa integration
  CLAUDE_API_KEY     # If not using default
  LOG_LEVEL          # DEBUG, INFO, WARNING, ERROR
  CACHE_DIR          # Override cache location
```

---

## Manual Execution

```
COMMAND LINE INTERFACE:

  # Run full pipeline
  ./portfolio-system run --job smsf-portfolio-daily
  
  # Run specific stages only
  ./portfolio-system run --job smsf-portfolio-daily --stages 0,1,2
  
  # Run for specific tickers
  ./portfolio-system run --job smsf-portfolio-daily --tickers SRG,ASB,PNC
  
  # Dry run (no email)
  ./portfolio-system run --job smsf-portfolio-daily --dry-run
  
  # Force re-fetch (ignore cache)
  ./portfolio-system run --job smsf-portfolio-daily --no-cache
  
  # Output report to file instead of email
  ./portfolio-system run --job smsf-portfolio-daily --output report.md
```

---

## Monitoring

```
HEALTH CHECKS:

  /health          # Basic health
  /health/data     # Data source connectivity
  /health/email    # SMTP connectivity
  
METRICS ENDPOINT:

  /metrics         # Prometheus format
  
METRICS EXPORTED:
  - pipeline_runs_total
  - pipeline_duration_seconds
  - pipeline_success_rate
  - tickers_processed_total
  - tickers_failed_total
  - data_fetch_duration_seconds
  - ai_assessment_duration_seconds
  - cache_hit_rate
```

---

## Next Document
Proceed to `11-example-output.md` for a complete example email report.
