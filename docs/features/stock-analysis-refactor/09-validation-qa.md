# ASX Portfolio Intelligence System — Validation & Quality Assurance

## Document Purpose
This document defines the validation checkpoints, quality gates, and error handling procedures that ensure data integrity and output correctness throughout the pipeline.

---

## Validation Philosophy

```
CORE PRINCIPLE:
  "It is better to take 15 minutes and be correct than 2 minutes and be wrong.
   Wrong decisions cost money; slow decisions cost time.
   Time is recoverable; capital is not."
```

**Priorities (in order)**:
1. Correctness - Data must be accurate
2. Completeness - All required fields present
3. Consistency - Outputs match inputs logically
4. Speed - Only after above are satisfied

---

## Validation Checkpoints by Stage

### Stage 0: Portfolio Load Validation

```
VALIDATE_PORTFOLIO_LOAD(portfolio_state):

  errors = []
  warnings = []
  
  # Structure validation
  IF portfolio_state.holdings IS empty:
    errors.append("Portfolio has no holdings")
  
  # Holdings validation
  FOR each holding IN portfolio_state.holdings:
    
    # Required fields
    IF holding.ticker IS empty:
      errors.append("Holding missing ticker")
    IF holding.units <= 0:
      errors.append("{ticker}: units must be positive")
    IF holding.avg_price <= 0:
      errors.append("{ticker}: avg_price must be positive")
    
    # Ticker format (ASX)
    IF NOT matches_pattern(holding.ticker, "^[A-Z0-9]{3,4}$"):
      errors.append("{ticker}: invalid ASX ticker format")
    
    # Reasonable values
    IF holding.units > 10_000_000:
      warnings.append("{ticker}: unusually large position")
    IF holding.avg_price > 1000:
      warnings.append("{ticker}: unusually high avg price")
  
  # Duplicate check
  tickers = extract_tickers(portfolio_state.holdings)
  IF has_duplicates(tickers):
    errors.append("Duplicate tickers found: {duplicates}")
  
  RETURN ValidationResult(errors, warnings)
```

---

### Stage 1: Data Fetch Validation

```
VALIDATE_TICKER_RAW(ticker_raw):

  errors = []
  warnings = []
  
  # Price data completeness
  IF ticker_raw.price.current <= 0:
    errors.append("{ticker}: missing current price")
  
  IF ticker_raw.price.ema_200 <= 0:
    warnings.append("{ticker}: missing 200 EMA - insufficient history")
  
  # Price sanity checks
  IF ticker_raw.price.current > ticker_raw.price.high_52w * 1.1:
    warnings.append("{ticker}: current price above 52w high (data lag?)")
  
  IF ticker_raw.price.current < ticker_raw.price.low_52w * 0.9:
    warnings.append("{ticker}: current price below 52w low (data lag?)")
  
  # Volume data
  IF ticker_raw.volume.sma_20 <= 0:
    warnings.append("{ticker}: missing volume data")
  
  # Fundamentals (non-ETF only)
  IF NOT is_etf(ticker_raw.ticker):
    IF ticker_raw.fundamentals IS null:
      warnings.append("{ticker}: missing fundamentals")
    ELSE:
      IF ticker_raw.fundamentals.market_cap_m <= 0:
        warnings.append("{ticker}: missing market cap")
  
  # Data freshness
  IF ticker_raw.fetch_timestamp < today() - 1 day:
    warnings.append("{ticker}: data is stale")
  
  RETURN ValidationResult(errors, warnings)
```

---

### Stage 2: Signal Computation Validation

```
VALIDATE_TICKER_SIGNALS(signals):

  errors = []
  warnings = []
  
  # PBAS validation
  IF signals.pbas.score < 0 OR signals.pbas.score > 1:
    errors.append("{ticker}: PBAS score out of range [0,1]")
  
  # VLI validation  
  IF signals.vli.score < -1 OR signals.vli.score > 1:
    errors.append("{ticker}: VLI score out of range [-1,1]")
  
  IF signals.vli.label NOT IN ["accumulating", "distributing", "neutral"]:
    errors.append("{ticker}: invalid VLI label")
  
  # Regime validation
  valid_regimes = ["breakout", "trend_up", "trend_down", "accumulation", 
                   "distribution", "range", "decay", "undefined"]
  IF signals.regime.classification NOT IN valid_regimes:
    errors.append("{ticker}: invalid regime classification")
  
  IF signals.regime.confidence < 0 OR signals.regime.confidence > 1:
    errors.append("{ticker}: regime confidence out of range")
  
  # Cooked validation
  IF signals.cooked.score < 0 OR signals.cooked.score > 5:
    errors.append("{ticker}: cooked score out of range [0,5]")
  
  IF signals.cooked.is_cooked AND signals.cooked.score < 2:
    errors.append("{ticker}: cooked flag inconsistent with score")
  
  # Cross-signal consistency
  IF signals.regime.classification = "breakout" AND signals.vli.label = "distributing":
    warnings.append("{ticker}: breakout regime with distribution volume - verify")
  
  # NaN check
  FOR each field IN signals.all_numeric_fields():
    IF is_nan(field.value):
      errors.append("{ticker}: NaN value in {field.name}")
  
  RETURN ValidationResult(errors, warnings)
```

---

### Stage 3: AI Assessment Validation

```
VALIDATE_ASSESSMENT(assessment, signals):

  errors = []
  warnings = []
  
  # Evidence count
  IF count(assessment.reasoning.evidence) < 3:
    errors.append("{ticker}: requires exactly 3 evidence points, got {count}")
  
  # Evidence quality
  FOR each evidence IN assessment.reasoning.evidence:
    
    # Must contain numbers
    IF NOT contains_number(evidence):
      errors.append("{ticker}: evidence lacks quantification: '{evidence}'")
    
    # Must not contain generic phrases
    generic = ["solid fundamentals", "well-positioned", "strong outlook",
               "good management", "attractive valuation", "quality company"]
    FOR phrase IN generic:
      IF evidence.lower() CONTAINS phrase:
        errors.append("{ticker}: generic phrase in evidence: '{phrase}'")
  
  # Action-signal consistency
  consistency_result = check_action_signal_consistency(
    assessment.decision.action, 
    signals
  )
  IF NOT consistency_result.valid:
    errors.append("{ticker}: action inconsistent with signals - {reason}")
  
  # Required fields
  IF assessment.entry_exit.stop_loss IS empty:
    errors.append("{ticker}: missing stop loss")
  
  IF assessment.thesis_status NOT IN ["intact", "weakening", "strengthening", "broken"]:
    errors.append("{ticker}: invalid thesis_status")
  
  # Confidence-evidence alignment
  IF assessment.decision.confidence = "high" AND count(evidence) < 4:
    warnings.append("{ticker}: high confidence should have 4+ evidence points")
  
  RETURN ValidationResult(errors, warnings)
```

---

### Stage 4: Portfolio Rollup Validation

```
VALIDATE_ROLLUP(rollup, portfolio_state, assessments):

  errors = []
  warnings = []
  
  # All holdings accounted for
  assessed_count = count(assessments)
  expected_count = count(portfolio_state.holdings)
  IF assessed_count != expected_count:
    errors.append("Holdings mismatch: assessed {assessed_count}, expected {expected_count}")
  
  # Portfolio value reconciliation
  calculated_value = sum(holding.current_value FOR holding IN assessed_holdings)
  IF abs(calculated_value - rollup.performance.total_value) > 0.01:
    errors.append("Portfolio value mismatch: calculated {calc}, reported {report}")
  
  # Allocation sums to ~100%
  sector_total = sum(rollup.allocation.by_sector.values())
  IF abs(sector_total - 100) > 1:
    errors.append("Sector allocation sums to {total}%, expected ~100%")
  
  # Action summary matches assessments
  urgent_count = count(a FOR a IN assessments WHERE a.urgency = "immediate")
  IF urgent_count != rollup.action_summary.immediate_actions:
    errors.append("Action summary mismatch")
  
  RETURN ValidationResult(errors, warnings)
```

---

### Stage 5: Report Validation

```
VALIDATE_REPORT(report):

  errors = []
  warnings = []
  
  # Required sections present
  required_sections = [
    "executive_summary",
    "actions_required",
    "holdings_summary",
    "portfolio_health"
  ]
  FOR section IN required_sections:
    IF report[section] IS null OR empty:
      errors.append("Missing required section: {section}")
  
  # Content checks
  IF length(report.markdown_content) < 1000:
    warnings.append("Report seems too short")
  
  IF length(report.markdown_content) > 50000:
    warnings.append("Report seems too long")
  
  # Date consistency
  IF report.report_date != format_date(today()):
    warnings.append("Report date mismatch")
  
  RETURN ValidationResult(errors, warnings)
```

---

## Quality Gates

Each stage has a quality gate that must pass before proceeding:

```
QUALITY_GATE(stage, validation_result):

  # Critical errors block pipeline
  IF validation_result.has_errors():
    IF stage.allows_partial_failure:
      # Log error, continue with valid data only
      log_error("Stage {stage} partial failure: {errors}")
      RETURN proceed_with_valid_only
    ELSE:
      # Block pipeline
      raise PipelineError("Stage {stage} failed: {errors}")
  
  # Warnings are logged but don't block
  IF validation_result.has_warnings():
    log_warning("Stage {stage} warnings: {warnings}")
  
  RETURN proceed
```

### Gate Configuration

| Stage | Allows Partial Failure | Error Threshold |
|-------|------------------------|-----------------|
| Portfolio Load | No | 0 errors |
| Data Fetch | Yes | Per-ticker |
| Signal Compute | Yes | Per-ticker |
| AI Assessment | Yes | Per-ticker |
| Rollup | No | 0 errors |
| Report | No | 0 errors |

---

## Error Recovery Strategies

### Per-Ticker Failures

```
HANDLE_TICKER_FAILURE(ticker, stage, error):

  CASE stage = "data_fetch":
    # Try cached data if available
    cached = cache.get("ticker-raw-{ticker}")
    IF cached AND cached.age < 24 hours:
      log_warning("Using cached data for {ticker}")
      RETURN cached
    ELSE:
      # Mark as failed, continue without
      log_error("Skipping {ticker}: {error}")
      RETURN null
  
  CASE stage = "signal_compute":
    # Cannot proceed without raw data
    log_error("Cannot compute signals for {ticker}: {error}")
    RETURN null
  
  CASE stage = "ai_assessment":
    # Retry once
    retry_result = retry_assessment(ticker, signals)
    IF retry_result.valid:
      RETURN retry_result
    ELSE:
      # Mark as insufficient data
      RETURN create_insufficient_data_assessment(ticker)
```

### AI Retry Logic

```
RETRY_ASSESSMENT(assessment, errors, signals):

  MAX_RETRIES = 2
  
  FOR attempt = 1 TO MAX_RETRIES:
    
    retry_prompt = build_retry_prompt(assessment, errors, signals)
    corrected = call_claude(retry_prompt)
    
    validation = validate_assessment(corrected, signals)
    
    IF validation.valid:
      RETURN corrected
    
    errors = validation.errors
    log_warning("Retry {attempt} failed for {ticker}")
  
  # All retries failed
  log_error("Assessment failed after {MAX_RETRIES} retries for {ticker}")
  RETURN create_insufficient_data_assessment(assessment.ticker, errors)
```

---

## Data Integrity Checks

### Cross-Stage Consistency

```
VERIFY_CROSS_STAGE_CONSISTENCY(all_tags):

  errors = []
  
  # Signals reference valid raw data
  FOR each signal_tag IN tags_matching("ticker-signals-*"):
    ticker = extract_ticker(signal_tag)
    raw_tag = "ticker-raw-{ticker}"
    
    IF NOT exists(raw_tag):
      errors.append("Signal {signal_tag} has no corresponding raw data")
    
    raw = load_tag(raw_tag)
    signal = load_tag(signal_tag)
    
    IF signal.price.current != raw.price.current:
      errors.append("{ticker}: price mismatch between raw and signals")
  
  # Assessments reference valid signals
  FOR each assessment_tag IN tags_matching("ticker-assessment-*"):
    ticker = extract_ticker(assessment_tag)
    signal_tag = "ticker-signals-{ticker}"
    
    IF NOT exists(signal_tag):
      errors.append("Assessment {assessment_tag} has no corresponding signals")
  
  RETURN errors
```

---

## Logging and Monitoring

```
LOG_LEVELS:
  DEBUG:   Detailed computation steps (dev only)
  INFO:    Stage progress, timing
  WARNING: Non-fatal issues, data quality concerns
  ERROR:   Failures requiring attention
  
LOG_FORMAT:
  [{timestamp}] [{level}] [{stage}] [{ticker}?] {message}

METRICS_TO_TRACK:
  - pipeline_duration_seconds
  - stage_duration_seconds (per stage)
  - tickers_processed
  - tickers_failed
  - validation_errors_count
  - validation_warnings_count
  - ai_retries_count
  - cache_hits / cache_misses
```

---

## Testing Requirements

### Unit Test Coverage

```
REQUIRED_UNIT_TESTS:

  # Computation algorithms
  - test_pbas_computation_known_inputs
  - test_pbas_edge_cases (zero revenue, negative, etc)
  - test_vli_accumulation_detection
  - test_vli_distribution_detection
  - test_regime_classification_all_types
  - test_cooked_detector_triggers
  
  # Validation
  - test_evidence_validation_rejects_generic
  - test_evidence_validation_requires_numbers
  - test_action_signal_consistency_all_actions
  
  # Announcement processing
  - test_announcement_type_classification
  - test_substance_score_computation
  - test_pr_entropy_detection
  - test_sni_calculation
```

### Integration Tests

```
REQUIRED_INTEGRATION_TESTS:

  - test_single_ticker_end_to_end
  - test_batch_assessment_processing
  - test_full_pipeline_mock_data
  - test_retry_logic_on_failure
  - test_cache_fallback_behavior
```

### Validation Test Vectors

```
TEST_VECTORS:

  # PBAS test cases
  pbas_tests = [
    {
      input: {rev_growth: 20%, ocf_growth: 15%, margin_delta: 2%, price_return: 10%},
      expected_range: [0.65, 0.80],
      expected_interpretation: "underpriced"
    },
    {
      input: {rev_growth: 5%, ocf_growth: -5%, margin_delta: -2%, price_return: 50%},
      expected_range: [0.15, 0.35],
      expected_interpretation: "overpriced"
    }
  ]
  
  # Evidence validation test cases
  evidence_tests = [
    {
      input: "PBAS of 0.72 exceeds threshold",
      expected: valid
    },
    {
      input: "Strong fundamentals",
      expected: invalid,
      reason: "generic phrase"
    },
    {
      input: "The company is well-positioned",
      expected: invalid,
      reason: "generic phrase, no number"
    }
  ]
```

---

## Processing Time Budgets

```
TIME_BUDGETS:
  
  Stage 0 (Portfolio Load):    < 5 seconds
  Stage 1 (Data Fetch):        < 60 seconds (parallel)
  Stage 2 (Signal Compute):    < 30 seconds
  Stage 3 (AI Assessment):     < 10 minutes (batched)
  Stage 4 (Rollup):            < 10 seconds
  Stage 5 (Report):            < 30 seconds
  
  TOTAL PIPELINE:              < 15 minutes

TIMEOUT_HANDLING:
  IF stage exceeds 2x budget:
    log_warning("Stage {stage} exceeding time budget")
  
  IF stage exceeds 3x budget:
    abort_stage()
    IF stage.allows_partial:
      continue_with_completed()
    ELSE:
      fail_pipeline()
```

---

## Next Document
Proceed to `10-orchestrator-job.md` for pipeline orchestration configuration.
