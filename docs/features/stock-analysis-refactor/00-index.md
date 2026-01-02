# ASX Portfolio Intelligence System — Documentation Index

## Overview

This documentation set provides a complete specification for building an AI-powered ASX portfolio analysis system. The system generates daily reports with evidence-based recommendations for both SMSF (long-term) and trader (short-term) portfolios.

---

## Core Principles

1. **Evidence-Based**: Every recommendation must cite specific, quantified data
2. **Signal Over Noise**: Separate substantive announcements from PR
3. **Correctness First**: Accuracy takes priority over speed
4. **Dual Horizon**: Support both 6-24 month SMSF and 1-12 week trader strategies
5. **Full Traceability**: Every claim traces to computed signals

---

## Document Map

| # | Document | Purpose |
|---|----------|---------|
| 01 | [Architecture Overview](01-architecture-overview.md) | System structure, data flow, module layout |
| 02 | [Data Models](02-data-models.md) | All data structures and schemas |
| 03 | [Tool Specifications](03-tool-specifications.md) | Tool interfaces and behaviors |
| 04 | [Computation Algorithms](04-computation-algorithms.md) | PBAS, VLI, Regime, Cooked algorithms |
| 05 | [Strategy Schema](05-strategy-schema.md) | Complete strategy configuration |
| 06 | [Announcement Processing](06-announcement-processing.md) | Classification, SNI, PR detection |
| 07 | [AI Assessment Prompts](07-ai-assessment-prompts.md) | Prompt templates, validation rules |
| 08 | [Report Generation](08-report-generation.md) | Report assembly and formatting |
| 09 | [Validation & QA](09-validation-qa.md) | Quality gates, error handling |
| 10 | [Orchestrator Job](10-orchestrator-job.md) | Pipeline configuration |
| 11 | [Example Output](11-example-output.md) | Complete example email report |
| 12 | [Evidence Chain](12-evidence-chain.md) | Signal-to-output mapping |

---

## Implementation Order

### Phase 1: Foundation (Week 1)
1. Set up project structure per `01-architecture-overview.md`
2. Implement data models from `02-data-models.md`
3. Build `portfolio_load` tool
4. Build `eodhd_fetch` tool with compression
5. Build `asx_announcements_fetch` tool
6. **Test**: Single ticker end-to-end data fetch

### Phase 2: Signal Computation (Week 2)
1. Implement PBAS algorithm from `04-computation-algorithms.md`
2. Implement VLI algorithm
3. Implement Regime classifier
4. Implement Cooked detector
5. Build `compute_signals` tool
6. **Test**: Validate signals against known examples

### Phase 3: AI Assessment (Week 3)
1. Create prompt templates from `07-ai-assessment-prompts.md`
2. Build batch processing logic
3. Implement validation rules from `09-validation-qa.md`
4. Build `ai_assess_batch` tool
5. Implement retry logic
6. **Test**: Batch of 5 holdings

### Phase 4: Report Assembly (Week 4)
1. Build `portfolio_rollup` tool
2. Build `assemble_report` tool from `08-report-generation.md`
3. Implement email formatting
4. Build `send_email` tool
5. **Test**: Full portfolio daily run

### Phase 5: Validation & Production (Week 5)
1. Implement all validation checkpoints
2. Set up orchestrator job from `10-orchestrator-job.md`
3. Back-test signal generation
4. Performance tracking setup
5. Documentation review
6. Production deployment

---

## Key Algorithms Summary

### PBAS (Price-Business Alignment Score)
**Purpose**: Quantify if price movement is justified by fundamentals

```
Business Momentum = 0.35×RevGrowth + 0.25×OCFGrowth + 0.20×MarginDelta 
                    + 0.10×ROIC - 0.10×Dilution

PBAS = sigmoid(5 × (BusinessMomentum - PriceMomentum))

Interpretation:
  > 0.65: Underpriced
  0.35-0.65: Fair
  < 0.35: Overpriced
```

### VLI (Volume Lead Indicator)
**Purpose**: Detect institutional accumulation/distribution

```
Accumulation signals: elevated volume + flat price + above VWAP
Distribution signals: elevated volume + weak price + below VWAP

VLI = AccumulationScore - DistributionScore

Range: -1.0 to +1.0
  > +0.50: Accumulating
  < -0.30: Distributing
```

### SNI (Signal-to-Noise Index)
**Purpose**: Rate announcement quality

```
SNI = SubstanceScore × ReactionScore × (1 - PREntropyScore)

  > 0.60: High-signal (act on it)
  0.30-0.60: Moderate
  < 0.30: Noise (ignore)
```

---

## Configuration Files

| File | Purpose |
|------|---------|
| `configs/portfolio.toml` | Holdings, cost basis, targets |
| `configs/strategy.toml` | Investment strategy parameters |
| `jobs/daily-review.toml` | Orchestrator job definition |

---

## Data Sources

| Source | Purpose | Rate Limit |
|--------|---------|------------|
| EODHD API | Price, fundamentals, technicals | 100k/day |
| ASX Website | Announcements | Respectful scraping |
| Navexa API | Portfolio sync (optional) | TBD |

---

## Environment Variables

```
EODHD_API_KEY       # Required: Market data API
SMTP_HOST           # Required: Email server
SMTP_USER           # Required: Email username  
SMTP_PASSWORD       # Required: Email password
NAVEXA_API_KEY      # Optional: Portfolio sync
LOG_LEVEL           # Optional: DEBUG/INFO/WARNING/ERROR
```

---

## Quality Standards

### Evidence Requirements
- Every action needs 3+ evidence points with specific numbers
- No generic phrases ("solid fundamentals", "well-positioned")
- All claims must trace to computed signals

### Validation Gates
- Stage 0: Portfolio structure valid
- Stage 1: Price data present for each ticker
- Stage 2: All signals in valid ranges, no NaN
- Stage 3: Evidence quality, action consistency
- Stage 4: Holdings accounted for, totals reconcile
- Stage 5: Required sections present

### Processing Targets
- Total pipeline: < 15 minutes for 25 holdings
- Data fetch: < 60 seconds (parallel)
- AI assessment: < 10 minutes (batched)
- Report generation: < 30 seconds

---

## Testing Checklist

- [ ] PBAS computation produces expected ranges
- [ ] VLI correctly identifies accumulation/distribution
- [ ] Regime classifier handles all regime types
- [ ] Cooked detector triggers on correct conditions
- [ ] Announcement classifier categorizes correctly
- [ ] SNI calculation matches expected values
- [ ] Evidence validation rejects generic phrases
- [ ] Action-signal consistency enforced
- [ ] Report renders correctly
- [ ] Email delivery works
- [ ] Full pipeline completes successfully

---

## Support

For implementation questions, refer to the specific document sections. Each document contains pseudocode and examples that can be directly translated to implementation.
