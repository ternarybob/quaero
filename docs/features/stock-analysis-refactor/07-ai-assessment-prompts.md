# ASX Portfolio Intelligence System — AI Assessment Prompts

## Document Purpose
This document defines the prompt templates and structures used for AI-powered stock assessment. These prompts are designed to elicit evidence-based, actionable recommendations.

---

## Prompt Design Principles

1. **Evidence Required**: Every recommendation must cite specific data points
2. **No Generic Language**: Reject phrases like "solid fundamentals"
3. **Structured Output**: Use consistent YAML format for parsing
4. **Dual Horizon**: Separate logic for trader vs SMSF timeframes
5. **Validation Built-In**: Include validation criteria in prompt

---

## Batch Assessment Prompt Template

### System Context

```
You are an expert ASX equity analyst assessing portfolio holdings.

CRITICAL RULES:
1. Every action recommendation MUST include exactly 3 evidence bullets with specific numbers
2. NEVER use generic phrases: "solid fundamentals", "well-positioned", "strong outlook"
3. If data is insufficient, output action: "insufficient_data" with missing items listed
4. Distinguish FACT (from signals) vs INFERENCE (your judgment)
5. All price targets and stops must be specific dollar values

OUTPUT FORMAT: Valid YAML only. No markdown, no explanations outside YAML.
```

### User Prompt Structure

```
Assess the following {batch_size} holdings for a {holding_type} portfolio.

STRATEGY CONTEXT:
- Horizon: {horizon_description}
- Risk tolerance: {risk_level}
- Current market regime: {market_regime}

DECISION FRAMEWORK FOR {holding_type}:
{decision_rules}

---

HOLDING 1: {ticker_1}
{signal_yaml_1}

HOLDING 2: {ticker_2}
{signal_yaml_2}

[... up to 5 holdings ...]

---

For each holding, output assessment in this exact YAML format:

```yaml
- ticker: {TICKER}
  decision:
    action: [accumulate|hold|reduce|exit|buy|add|trim|watch|insufficient_data]
    confidence: [high|medium|low]
    urgency: [immediate|this_week|monitor]
  reasoning:
    primary: "1-2 sentence main rationale"
    evidence:
      - "Specific data point 1 with number"
      - "Specific data point 2 with number"
      - "Specific data point 3 with number"
  entry_exit:
    stop_loss: "$X.XX"
    stop_loss_pct: X.X
    target_1: "$X.XX"
    invalidation: "Condition that breaks thesis"
  risk_flags:
    - "Risk 1"
    - "Risk 2"
  thesis_status: [intact|weakening|strengthening|broken]
  justified_gain:
    justified_12m_pct: X.X
    current_gain_pct: X.X
    verdict: [aligned|ahead|behind]
```
```

---

## Decision Rules by Holding Type

### SMSF Decision Rules (6-24 Month Horizon)

```
ACCUMULATE if ALL:
  - PBAS > 0.60 OR PBAS improving by >0.10 over 3 months
  - OCF/EBITDA > 0.70
  - Net Debt/EBITDA stable or falling
  - RS vs XJO positive (> 1.0)
  - Not flagged as cooked

HOLD if:
  - PBAS between 0.45 and 0.65
  - No deterioration in cash quality
  - Regime not in decay/distribution

REDUCE if ANY:
  - PBAS < 0.45 AND declining
  - OCF/EBITDA < 0.60
  - Dilution > 10% without OCF growth
  - Cooked score >= 2

EXIT if ANY:
  - PBAS < 0.35
  - Regime = decay with no recovery signs
  - Thesis broken (fundamental deterioration)
  - Stop loss breached
```

### Trader Decision Rules (1-12 Week Horizon)

```
BUY/ADD if ALL:
  - Regime in [accumulation, trend_up, breakout]
  - VLI > 0.50 (accumulation confirmed)
  - RS rank > 60th percentile
  - Not cooked
  - Entry setup present (pullback, breakout, VCP)

HOLD if:
  - Regime = trend_up
  - No distribution signals
  - Above key EMAs

TRIM if ANY:
  - Regime transitioning to distribution
  - VLI turning negative
  - Failed breakout with volume divergence
  - Profit target 1 reached

EXIT if ANY:
  - Regime = decay
  - Stop loss breached
  - High-substance negative announcement
```

---

## Signal Document Format (Input to AI)

Each ticker's signals are provided in compressed YAML:

```yaml
ticker: SRG
compute_timestamp: "2025-01-02T07:00:00Z"

price:
  current: 3.12
  change_1d_pct: 1.2
  return_12w_pct: 8.5
  return_52w_pct: 24.3
  vs_ema20: above
  vs_ema50: above
  vs_ema200: above
  distance_to_52w_high_pct: 5.2

pbas:
  score: 0.72
  business_momentum: 0.18
  price_momentum: 0.12
  divergence: 0.06
  interpretation: underpriced

vli:
  score: 0.64
  label: accumulating
  vol_zscore: 1.2
  price_vs_vwap: 1.02

regime:
  classification: trend_up
  confidence: 0.75
  trend_bias: bullish
  ema_stack: bullish

relative_strength:
  vs_xjo_3m: 1.12
  vs_xjo_6m: 1.08
  rs_rank_percentile: 72

cooked:
  is_cooked: false
  score: 0
  reasons: null

quality:
  overall: good
  cash_conversion: good
  balance_sheet_risk: low
  margin_trend: stable

announcements:
  high_signal_count_30d: 2
  most_recent_material: "Contract win $18M"
  most_recent_material_sni: 0.74
  sentiment_30d: positive
  pr_heavy_issuer: false

justified_return:
  expected_12m_pct: 18.0
  actual_12m_pct: 24.3
  divergence_pct: 6.3
  interpretation: slightly_ahead

risk_flags:
  - "Sector concentration in portfolio"
```

---

## Evidence Quality Requirements

### Valid Evidence Examples

```
GOOD:
- "PBAS of 0.72 indicates business momentum (+0.18) exceeds price momentum (+0.12)"
- "VLI at 0.64 with vol_zscore 1.2 confirms institutional accumulation"
- "OCF/EBITDA of 0.85 above 0.70 threshold indicates quality cash conversion"
- "RS rank at 72nd percentile, outperforming XJO by 12% over 3 months"
- "Recent $18M contract (SNI: 0.74) supports revenue growth thesis"

BAD:
- "Strong fundamentals support the investment case"
- "Well-positioned for growth"
- "Solid balance sheet"
- "Management is executing well"
- "Attractive valuation"
```

### Evidence Validation Logic

```
VALIDATE_EVIDENCE(evidence_list):
  
  FOR each item IN evidence_list:
    
    # Must contain a number
    IF NOT contains_quantification(item):
      REJECT with "Evidence must include specific numbers"
    
    # Must reference a signal metric
    IF NOT references_known_metric(item):
      REJECT with "Evidence must reference computed signals"
    
    # Must not contain generic phrases
    generic_phrases = ["solid", "strong", "well-positioned", "attractive", 
                       "quality", "excellent", "good management"]
    IF item contains ANY generic_phrases:
      REJECT with "Generic phrase detected"
  
  # Must have minimum count
  IF count(evidence_list) < 3:
    REJECT with "Minimum 3 evidence points required"
  
  RETURN valid
```

---

## Action-Signal Consistency Rules

```
VALIDATE_ACTION_CONSISTENCY(action, signals):

  CASE action = "accumulate":
    REQUIRE signals.pbas.score > 0.55
    REQUIRE signals.cooked.is_cooked = false
    REQUIRE signals.regime NOT IN [decay, distribution]
  
  CASE action = "hold":
    REQUIRE signals.pbas.score BETWEEN 0.40 AND 0.75
    REQUIRE signals.regime NOT IN [decay]
  
  CASE action = "reduce":
    REQUIRE signals.pbas.score < 0.50 
       OR signals.cooked.is_cooked = true
       OR signals.vli.label = "distributing"
  
  CASE action = "exit":
    REQUIRE signals.cooked.score >= 2
       OR signals.regime = "decay"
       OR signals.pbas.score < 0.35
  
  CASE action = "buy" OR "add":
    REQUIRE signals.vli.score > 0.30
    REQUIRE signals.regime IN [trend_up, accumulation, breakout]
  
  CASE action = "trim":
    REQUIRE signals.regime IN [distribution, decay]
       OR signals.vli.label = "distributing"
  
  CASE action = "watch":
    # Monitoring mode - no strict requirements
    PASS
  
  CASE action = "insufficient_data":
    REQUIRE missing_data_list is NOT empty

  RETURN is_consistent
```

---

## Retry Logic on Validation Failure

```
RETRY_ASSESSMENT(original_assessment, validation_errors, signals):

  retry_prompt = """
    Your previous assessment for {ticker} failed validation:
    
    ERRORS:
    {validation_errors}
    
    Please provide a corrected assessment:
    1. Include exactly 3 evidence points with specific numbers
    2. Ensure action matches signal data
    3. Remove generic phrases
    
    Signals:
    {signals_yaml}
    
    Output corrected YAML only.
  """
  
  corrected_response = call_claude(retry_prompt)
  corrected_assessment = parse_yaml(corrected_response)
  
  RETURN corrected_assessment
```

---

## Batch Processing Flow

```
PROCESS_PORTFOLIO_ASSESSMENTS(holdings, strategy):

  CONSTANTS:
    BATCH_SIZE = 5
    THINKING_BUDGET = 16000
    MAX_RETRIES = 2
  
  batches = split_into_chunks(holdings, BATCH_SIZE)
  all_assessments = []
  
  FOR each batch IN batches:
    
    # Load signals
    signals = []
    FOR each holding IN batch:
      signal_doc = load_tagged_document("ticker-signals-{holding.ticker}")
      signals.append(signal_doc)
    
    # Build and execute prompt
    prompt = build_assessment_prompt(signals, strategy)
    response = call_claude_with_thinking(prompt, THINKING_BUDGET)
    assessments = parse_yaml_list(response)
    
    # Validate each
    FOR each assessment IN assessments:
      validation = validate_assessment(assessment, signals)
      
      IF NOT validation.valid:
        FOR retry = 1 TO MAX_RETRIES:
          assessment = retry_assessment(assessment, validation.errors, signals)
          validation = validate_assessment(assessment, signals)
          IF validation.valid:
            BREAK
      
      assessment.validation_passed = validation.valid
      assessment.validation_errors = validation.errors
      all_assessments.append(assessment)
    
    # Rate limit between batches
    wait(1 second)
  
  RETURN all_assessments
```

---

## Output Schema Reference

```yaml
ticker: string
holding_type: enum [smsf, trader]

decision:
  action: enum [accumulate, hold, reduce, exit, buy, add, trim, watch, insufficient_data]
  confidence: enum [high, medium, low]
  urgency: enum [immediate, this_week, monitor]

reasoning:
  primary: string        # 1-2 sentence rationale
  evidence: list[string] # Exactly 3 items with numbers

entry_exit:
  setup: string          # Entry setup name (for buys)
  entry_zone: string     # Price range
  stop_loss: string      # Dollar value
  stop_loss_pct: float   # Percentage
  target_1: string       # First target
  invalidation: string   # Thesis breaker

risk_flags: list[string]
thesis_status: enum [intact, weakening, strengthening, broken]

justified_gain:
  justified_12m_pct: float
  current_gain_pct: float
  verdict: enum [aligned, ahead, behind]

validation_passed: boolean
validation_errors: list[string] | null
```

---

## Next Document
Proceed to `08-report-generation.md` for report assembly specifications.
