# ASX Portfolio Intelligence System — Evidence Chain Mapping

## Document Purpose
This document maps every user-facing output to its technical source. No claim in the report should exist without a traceable computation.

---

## Core Principle

```
RULE: Every assertion must trace to a computed value.

USER SEES                 →  DERIVED FROM           →  RAW DATA SOURCE
────────────────────────────────────────────────────────────────────────
"Stock is undervalued"    →  PBAS > 0.65            →  Revenue, OCF, margins
"Institutional buying"    →  VLI > 0.5              →  Volume, VWAP, ATR
"Uptrend confirmed"       →  regime = trend_up      →  EMA positions, slopes
```

---

## Complete Evidence Mapping

### Price-Based Claims

| User-Facing Statement | Required Signal | Threshold | Raw Data |
|----------------------|-----------------|-----------|----------|
| "At 52-week high" | price.distance_to_52w_high_pct | < 2% | OHLCV history |
| "Near 52-week low" | price.distance_to_52w_low_pct | < 10% | OHLCV history |
| "Above key moving averages" | price.vs_ema20, vs_ema50, vs_ema200 | "above" | OHLCV + EMA calc |
| "Below support" | price.vs_ema200 | "below" | OHLCV + EMA calc |
| "Up X% over Y period" | price.return_Xw_pct | Calculated | OHLCV history |
| "Extended from mean" | price / ema_200 | > 1.30 | OHLCV + EMA calc |

### Valuation Claims

| User-Facing Statement | Required Signal | Threshold | Raw Data |
|----------------------|-----------------|-----------|----------|
| "Underpriced / Undervalued" | pbas.score | > 0.65 | Fundamentals + price |
| "Overpriced / Overvalued" | pbas.score | < 0.35 | Fundamentals + price |
| "Fairly valued" | pbas.score | 0.35 - 0.65 | Fundamentals + price |
| "Business ahead of price" | pbas.divergence | > 0.10 | Business vs price momentum |
| "Price ahead of business" | pbas.divergence | < -0.10 | Business vs price momentum |
| "Cooked / Decoupled" | cooked.is_cooked | true | PBAS + triggers |

### Volume/Flow Claims

| User-Facing Statement | Required Signal | Threshold | Raw Data |
|----------------------|-----------------|-----------|----------|
| "Institutional buying" | vli.label | "accumulating" | Volume, VWAP |
| "Institutional selling" | vli.label | "distributing" | Volume, VWAP |
| "Accumulation pattern" | vli.score | > 0.50 | Volume, VWAP, ATR |
| "Distribution pattern" | vli.score | < -0.30 | Volume, VWAP |
| "Volume expanding" | volume.zscore_20 | > 1.0 | Volume history |
| "Volume contracting" | volume.zscore_20 | < -0.5 | Volume history |
| "Elevated volume" | volume.current / volume.sma_20 | > 1.5 | Volume history |

### Regime Claims

| User-Facing Statement | Required Signal | Threshold | Raw Data |
|----------------------|-----------------|-----------|----------|
| "In uptrend" | regime.classification | "trend_up" | EMAs, price action |
| "In downtrend" | regime.classification | "trend_down" | EMAs, price action |
| "Breaking out" | regime.classification | "breakout" | 52w high, volume |
| "Accumulating" | regime.classification | "accumulation" | Price range, VLI |
| "Distributing" | regime.classification | "distribution" | Price action, volume |
| "In decay" | regime.classification | "decay" | Below EMAs, failed rallies |
| "Range-bound" | regime.classification | "range" | Price range analysis |
| "Bullish structure" | regime.ema_stack | "bullish" | EMA 20 > 50 > 200 |
| "Bearish structure" | regime.ema_stack | "bearish" | EMA 20 < 50 < 200 |

### Relative Strength Claims

| User-Facing Statement | Required Signal | Threshold | Raw Data |
|----------------------|-----------------|-----------|----------|
| "Outperforming market" | rs.vs_xjo_3m | > 1.05 | Ticker vs XJO returns |
| "Underperforming market" | rs.vs_xjo_3m | < 0.95 | Ticker vs XJO returns |
| "Top quartile performer" | rs.rs_rank_percentile | > 75 | Universe ranking |
| "Bottom quartile" | rs.rs_rank_percentile | < 25 | Universe ranking |
| "Relative strength improving" | rs.vs_xjo_3m > rs.vs_xjo_6m | - | Return comparison |
| "Relative strength declining" | rs.vs_xjo_3m < rs.vs_xjo_6m | - | Return comparison |

### Fundamental Quality Claims

| User-Facing Statement | Required Signal | Threshold | Raw Data |
|----------------------|-----------------|-----------|----------|
| "Good cash conversion" | quality.cash_conversion | "good" | OCF/EBITDA > 0.80 |
| "Fair cash conversion" | quality.cash_conversion | "fair" | OCF/EBITDA 0.60-0.80 |
| "Poor cash conversion" | quality.cash_conversion | "poor" | OCF/EBITDA < 0.60 |
| "Low balance sheet risk" | quality.balance_sheet_risk | "low" | Debt ratios |
| "High balance sheet risk" | quality.balance_sheet_risk | "high" | Debt ratios |
| "Margins improving" | quality.margin_trend | "improving" | Margin delta |
| "Margins declining" | quality.margin_trend | "declining" | Margin delta |

### Announcement Claims

| User-Facing Statement | Required Signal | Threshold | Raw Data |
|----------------------|-----------------|-----------|----------|
| "High-signal announcement" | announcement.sni | > 0.60 | Substance + reaction |
| "Noise / PR" | announcement.sni | < 0.30 | Substance + reaction |
| "PR-heavy issuer" | announcements.pr_heavy_issuer | true | Frequency + SNI |
| "Positive news flow" | announcements.sentiment_30d | "positive" | Recent announcements |
| "Negative news flow" | announcements.sentiment_30d | "negative" | Recent announcements |
| "Material contract" | announcement.type | "quantified_contract" | Announcement class |
| "Guidance change" | announcement.type | "guidance_change" | Announcement class |

### Action Claims

| User-Facing Statement | Required Signals | Rule |
|----------------------|------------------|------|
| "Accumulate" | PBAS > 0.55, not cooked, regime not decay | SMSF |
| "Hold" | PBAS 0.40-0.75, no deterioration | SMSF |
| "Reduce" | PBAS < 0.50 OR cooked OR VLI distributing | SMSF |
| "Exit" | cooked.score >= 2 OR regime = decay OR PBAS < 0.35 | SMSF |
| "Buy/Add" | VLI > 0.30, regime in [trend_up, accumulation, breakout] | Trader |
| "Trim" | regime in [distribution, decay] OR VLI distributing | Trader |

---

## PBAS Computation Chain

```
USER SEES: "PBAS Score: 0.72 (underpriced)"

COMPUTATION PATH:
                                                     
  Revenue YoY Growth ──────────┐
  (from fundamentals)          │
                               ▼
  OCF/EBITDA Ratio ───────────┬──► Business Momentum (BM)
  (from fundamentals)          │    = 0.35×RevGrowth + 0.25×OCFGrowth
                               │      + 0.20×MarginDelta + 0.10×ROIC
  Margin Delta ───────────────┤      - 0.10×Dilution
  (from fundamentals)          │
                               │
  ROIC ───────────────────────┤
  (from fundamentals)          │
                               │
  Dilution 12M ───────────────┘
  (from fundamentals)
                               
  52-Week Return ─────────────────► Price Momentum (PM)
  (from OHLCV)                      = return_52w / 100
                               
                              
  BM - PM ────────────────────────► Divergence
  (computed)
                               
  sigmoid(5 × Divergence) ────────► PBAS Score
  (computed)                        = 0.72
                               
  IF PBAS > 0.65 ─────────────────► Interpretation
  (threshold check)                 = "underpriced"
```

---

## VLI Computation Chain

```
USER SEES: "VLI: 0.64 (accumulating)"

COMPUTATION PATH:

  Volume History (20 days) ────┐
  (from OHLCV)                 │
                               ▼
  Volume Z-Score ─────────────────► vol_zscore = 1.2
  = (current - mean) / stddev       (elevated)

  VWAP (20 days) ─────────────────► vwap_20 = $3.05
  (from OHLCV)

  Current Price ──────────────────► price_vs_vwap = 1.02
  (from OHLCV)                      (above VWAP)

  Volume Trend ───────────────────► trend = "rising"
  (5-day vs 20-day avg)

  
  SCORING:
    vol_zscore > 0.5      → +0.30
    vol_trend = rising    → +0.20
    price_vs_vwap > 1.0   → +0.15
    vol_zscore > 1.5      → +0.15
    ─────────────────────────────
    Accumulation Score    =  0.80
    
    Distribution Score    =  0.16
    ─────────────────────────────
    VLI = Acc - Dist      =  0.64


  IF VLI > 0.50 AND 
     Acc > Dist ──────────────────► Label = "accumulating"
```

---

## Regime Classification Chain

```
USER SEES: "Regime: trend_up (confidence: 0.75)"

COMPUTATION PATH:

  Current Price ─────┐
  EMA 20 ───────────┼────► Price Position
  EMA 50 ───────────┤     above_ema20 = true
  EMA 200 ──────────┘     above_ema50 = true
  (from OHLCV)            above_ema200 = true


  EMA 20 ───────────┐
  EMA 50 ───────────┼────► EMA Stack
  EMA 200 ──────────┘     ema_20 > ema_50 > ema_200
  (computed)              = "bullish"


  52W High ─────────┬────► Near High Check
  Current Price ────┘     distance = 5.2%
                          near_high = false


  4-Week Return ──────────► Momentum Direction
  12-Week Return            4w = +3.5%, 12w = +8.5%
  (from OHLCV)              positive momentum


  CLASSIFICATION RULES:
    IF bullish_stack AND above_ema20 AND return_4w > 0:
      regime = "trend_up"
      confidence = 0.70 + 0.05 (strong momentum)
                 = 0.75
```

---

## Cooked Detection Chain

```
USER SEES: "Cooked: YES (score 2 of 5)"

COMPUTATION PATH:

  TRIGGER CHECKS:
  
  1. PBAS Score ────────────────► Check PBAS < 0.30
     PBAS = 0.31                  0.31 < 0.30? NO ✗
     
  2. Price CAGR ────────────────► Check Price > 2.5× Revenue
     Rev CAGR = 5%                50% > 2.5 × 5%?
     Price Return = 50%           50% > 12.5%? YES ✓
     AND Price > 30%?             50% > 30%? YES
     
  3. Dilution ──────────────────► Check Dilution > 10%
     Dilution = 12%               12% > 10%? YES ✓
     
  4. Cash Conversion ───────────► Check OCF/EBITDA < 0.60
     OCF/EBITDA = 0.72            0.72 < 0.60? NO ✗
     
  5. Extension ─────────────────► Check Price > 1.30 × EMA200
     Price/EMA200 = 1.15          1.15 > 1.30? NO ✗
     
  
  TRIGGERS: 2 (items 2 and 3)
  THRESHOLD: >= 2
  
  RESULT: is_cooked = true
          score = 2
          reasons = ["price_revenue_divergence", "dilution_above_10pct"]
```

---

## Evidence Requirement Matrix

| Decision | Min Evidence | Required Signal Types |
|----------|--------------|----------------------|
| ACCUMULATE | 3 | PBAS, VLI/Regime, Quality |
| HOLD | 3 | PBAS, Regime, 1 other |
| REDUCE | 3 | PBAS or Cooked, VLI, Quality |
| EXIT | 3 | Cooked or PBAS < 0.35, Regime, Risk |
| BUY/ADD | 3 | VLI, Regime, RS |
| TRIM | 3 | Regime/VLI, Announcement, 1 other |

---

## Forbidden Assertions

These statements are NEVER allowed without the specified evidence:

| Statement | Required Evidence |
|-----------|-------------------|
| "Strong fundamentals" | Must specify: ROE, margins, OCF/EBITDA with numbers |
| "Well-positioned" | Must specify: market share, competitive advantage metric |
| "Quality company" | Must specify: ROIC, cash conversion, margin trend |
| "Attractive valuation" | Must specify: PBAS score, P/E vs sector |
| "Good management" | NEVER use - not measurable |
| "Solid balance sheet" | Must specify: Net Debt/EBITDA, current ratio |
| "Growth potential" | Must specify: Revenue CAGR, market size data |

---

## Validation Pseudocode

```
VALIDATE_EVIDENCE_CHAIN(user_statement):

  # Find required signal for this statement type
  statement_type = classify_statement(user_statement)
  required_signal = EVIDENCE_MAPPING[statement_type]
  
  # Check if signal exists and meets threshold
  IF NOT signals.has(required_signal.field):
    REJECT "Missing signal data for: {required_signal.field}"
  
  signal_value = signals.get(required_signal.field)
  
  IF required_signal.has_threshold:
    IF NOT meets_threshold(signal_value, required_signal.threshold):
      REJECT "Signal {signal_value} does not support claim (threshold: {threshold})"
  
  # Check for forbidden generic phrases
  FOR phrase IN FORBIDDEN_PHRASES:
    IF user_statement CONTAINS phrase:
      IF NOT has_specific_evidence(phrase):
        REJECT "Generic phrase '{phrase}' requires specific metrics"
  
  RETURN valid
```

---

## Example: Complete Evidence Chain for Reduce Action

```
ACTION: REDUCE PNC by 50%
CONFIDENCE: HIGH

EVIDENCE CHAIN:

┌─────────────────────────────────────────────────────────────────────────────┐
│ Evidence Point 1: "PBAS collapsed from 0.52 to 0.31 over 3 months"         │
├─────────────────────────────────────────────────────────────────────────────┤
│ Signal: signals.pbas.score = 0.31                                          │
│ Historical: historical_signals[3_months_ago].pbas.score = 0.52             │
│ Threshold: PBAS < 0.35 triggers concern                                    │
│ Computation: Business momentum (-0.08) - Price momentum (+0.05) = -0.13    │
│ Raw Data: Revenue YoY (-2%), OCF YoY (-15%), Price YoY (+5%)              │
└─────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────┐
│ Evidence Point 2: "OCF/EBITDA ratio fell from 0.82 to 0.58"                │
├─────────────────────────────────────────────────────────────────────────────┤
│ Signal: signals.fundamentals.ocf_to_ebitda = 0.58                          │
│ Historical: FY23 annual report OCF/EBITDA = 0.82                           │
│ Threshold: OCF/EBITDA < 0.70 = poor cash conversion                        │
│ Raw Data: Operating CF $8.2M, EBITDA $14.1M                                │
└─────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────┐
│ Evidence Point 3: "VLI = -0.42 indicates institutional distribution"       │
├─────────────────────────────────────────────────────────────────────────────┤
│ Signal: signals.vli.score = -0.42                                          │
│ Signal: signals.vli.label = "distributing"                                 │
│ Threshold: VLI < -0.30 = distribution                                      │
│ Components: vol_zscore = 1.2, price_vs_vwap = 0.97                         │
│ Raw Data: Volume 1.8M (vs 1.1M avg), Price below VWAP                      │
└─────────────────────────────────────────────────────────────────────────────┘

ACTION-SIGNAL CONSISTENCY CHECK:
  REDUCE requires: PBAS < 0.50 OR cooked OR VLI distributing
  
  Checks:
    PBAS = 0.31 < 0.50 ✓
    cooked.score = 2 >= 2 ✓
    VLI = -0.42 < -0.30 (distributing) ✓
  
  Result: 3/3 conditions met → Action CONSISTENT
```

---

## Summary

Every user-facing claim must:

1. **Reference a computed signal** - No qualitative-only statements
2. **Meet defined thresholds** - Numbers must cross thresholds to justify claims
3. **Trace to raw data** - Every signal has an auditable computation path
4. **Avoid generic phrases** - Specificity required for all statements

This evidence chain ensures that the system produces defensible, reproducible recommendations.
