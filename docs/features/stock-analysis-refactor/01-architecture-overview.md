# ASX Portfolio Intelligence System — Architecture Overview

## Document Purpose
This document provides the high-level architecture for the ASX Portfolio Intelligence System. It defines the system boundaries, data flow, module responsibilities, and integration points.

---

## System Goals

1. **Daily Portfolio Analysis**: Automated assessment of ASX portfolio holdings
2. **Evidence-Based Decisions**: Every recommendation backed by computed metrics
3. **Signal Extraction**: Separate substance from PR in company announcements
4. **Dual Horizon Support**: Trading (1-12 weeks) and SMSF (6-24 months) strategies
5. **Screening Capability**: Find new candidates matching strategy parameters

---

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                              ORCHESTRATOR LAYER                                  │
│                         (Job scheduling, pipeline control)                       │
└─────────────────────────────────────────────────────────────────────────────────┘
                                       │
        ┌──────────────────────────────┼──────────────────────────────┐
        │                              │                              │
        ▼                              ▼                              ▼
┌───────────────┐              ┌───────────────┐              ┌───────────────┐
│   STAGE 0     │              │   STAGE 1     │              │   STAGE 2     │
│   Portfolio   │─────────────▶│   Data Fetch  │─────────────▶│   Compute     │
│   Ingestion   │              │   (Parallel)  │              │   Signals     │
└───────────────┘              └───────────────┘              └───────────────┘
        │                              │                              │
        ▼                              ▼                              ▼
  <portfolio-state>           <ticker-raw-{T}>              <ticker-signals-{T}>
                              <ticker-announcements-{T}>
        │                              │                              │
        └──────────────────────────────┼──────────────────────────────┘
                                       │
                                       ▼
                            ┌─────────────────────┐
                            │      STAGE 3        │
                            │   AI Assessment     │
                            │   (Batched, 5/call) │
                            └─────────────────────┘
                                       │
                                       ▼
                            <ticker-assessment-{T}>
                                       │
                                       ▼
                            ┌─────────────────────┐
                            │      STAGE 4        │
                            │  Portfolio Rollup   │
                            └─────────────────────┘
                                       │
                                       ▼
                              <portfolio-rollup>
                                       │
                                       ▼
                            ┌─────────────────────┐
                            │      STAGE 5        │
                            │  Report Assembly    │
                            └─────────────────────┘
                                       │
                                       ▼
                               <daily-report>
                                       │
                                       ▼
                            ┌─────────────────────┐
                            │      STAGE 6        │
                            │   Email Delivery    │
                            └─────────────────────┘
```

---

## Module Structure

```
asx-portfolio-system/
├── cmd/
│   └── main.go                     # CLI entrypoint
├── internal/
│   ├── config/
│   │   ├── portfolio.go            # Portfolio configuration loader
│   │   └── strategy.go             # Strategy configuration loader
│   ├── data/
│   │   ├── eodhd/
│   │   │   ├── client.go           # EODHD API client
│   │   │   ├── prices.go           # Price data fetcher
│   │   │   ├── fundamentals.go     # Fundamentals fetcher
│   │   │   └── technicals.go       # Technical indicators
│   │   ├── asx/
│   │   │   ├── announcements.go    # ASX announcement fetcher
│   │   │   └── parser.go           # Announcement text parser
│   │   └── navexa/
│   │       └── client.go           # Navexa API client (optional)
│   ├── compute/
│   │   ├── indicators/
│   │   │   ├── ema.go              # Exponential moving averages
│   │   │   ├── atr.go              # Average true range
│   │   │   ├── rsi.go              # Relative strength index
│   │   │   ├── vwap.go             # Volume-weighted average price
│   │   │   └── volume.go           # Volume analytics
│   │   ├── signals/
│   │   │   ├── pbas.go             # Price-Business Alignment Score
│   │   │   ├── vli.go              # Volume Lead Indicator
│   │   │   ├── regime.go           # Regime classifier
│   │   │   ├── cooked.go           # Overvaluation detector
│   │   │   └── rs.go               # Relative strength
│   │   └── announcements/
│   │       ├── classifier.go       # Announcement type classifier
│   │       ├── substance.go        # Substance score calculator
│   │       ├── entropy.go          # PR entropy scorer
│   │       └── sni.go              # Signal-to-noise index
│   ├── assessment/
│   │   ├── batch.go                # Batch processing logic
│   │   ├── prompts.go              # AI assessment prompts
│   │   └── validator.go            # Assessment validation
│   ├── report/
│   │   ├── generator.go            # Report assembly
│   │   ├── templates/              # Report templates
│   │   └── email.go                # Email formatting
│   ├── screening/
│   │   ├── screener.go             # Stock screening engine
│   │   └── filters.go              # Filter implementations
│   └── storage/
│       ├── tags.go                 # Tagged document storage
│       └── cache.go                # Computation cache
├── pkg/
│   └── models/
│       ├── portfolio.go            # Portfolio data models
│       ├── ticker.go               # Ticker data models
│       ├── signals.go              # Signal data models
│       ├── assessment.go           # Assessment data models
│       └── strategy.go             # Strategy data models
├── configs/
│   ├── portfolio.toml              # Portfolio holdings
│   └── strategy.toml               # Investment strategy
├── jobs/
│   └── daily-review.toml           # Orchestrator job definition
└── docs/
    ├── 01-architecture-overview.md
    ├── 02-data-models.md
    ├── 03-tool-specifications.md
    ├── 04-computation-algorithms.md
    ├── 05-strategy-schema.md
    ├── 06-announcement-processing.md
    ├── 07-ai-assessment-prompts.md
    ├── 08-report-generation.md
    ├── 09-validation-qa.md
    └── 10-orchestrator-job.md
```

---

## Data Flow

### Stage 0: Portfolio Ingestion
**Input**: `configs/portfolio.toml` or Navexa API
**Output**: `<portfolio-state>` tagged document
**Responsibility**: Load holdings, compute cost basis, validate structure

### Stage 1: Data Fetch
**Input**: Ticker list from `<portfolio-state>`
**Output**: `<ticker-raw-{T}>` and `<ticker-announcements-{T}>` per ticker
**Responsibility**: Fetch and compress market data, fundamentals, announcements
**Execution**: Parallel (safe for I/O operations)

### Stage 2: Signal Computation
**Input**: Raw data tags
**Output**: `<ticker-signals-{T}>` per ticker
**Responsibility**: Compute PBAS, VLI, Regime, Cooked, RS
**Execution**: Sequential (deterministic computation)

### Stage 3: AI Assessment
**Input**: Signal tags (batched, 5 per call)
**Output**: `<ticker-assessment-{T}>` per ticker
**Responsibility**: Generate trading/SMSF decisions with evidence
**Execution**: Sequential batches with validation

### Stage 4: Portfolio Rollup
**Input**: All assessment tags + portfolio state
**Output**: `<portfolio-rollup>` tag
**Responsibility**: Aggregate metrics, detect concentration, correlations

### Stage 5: Report Assembly
**Input**: All tags
**Output**: `<daily-report>` tag
**Responsibility**: Format final report for email

### Stage 6: Email Delivery
**Input**: `<daily-report>` tag
**Output**: Email sent
**Responsibility**: Deliver report to configured recipients

---

## Tagged Document System

All inter-stage communication uses tagged documents stored in a key-value store.

```go
type TaggedDocument struct {
    Tag       string    `json:"tag"`
    Content   []byte    `json:"content"`
    Format    string    `json:"format"` // yaml, json, markdown
    CreatedAt time.Time `json:"created_at"`
    Stage     string    `json:"stage"`
}

type TagStore interface {
    Set(tag string, content []byte, format string) error
    Get(tag string) (*TaggedDocument, error)
    List(prefix string) ([]string, error)
    Delete(tag string) error
}
```

---

## External Dependencies

| Dependency | Purpose | Rate Limits |
|------------|---------|-------------|
| EODHD API | Price, fundamentals, technicals | 100k/day |
| ASX Website | Announcements | Scraping (respectful) |
| Navexa API | Portfolio sync (optional) | TBD |
| Claude API | AI assessments | Per subscription |
| SMTP | Email delivery | Per provider |

---

## Configuration Files

### Portfolio Configuration
Location: `configs/portfolio.toml`
Purpose: Define holdings, cost basis, target weights
See: `05-strategy-schema.md` for full schema

### Strategy Configuration
Location: `configs/strategy.toml`
Purpose: Define investment strategy parameters
See: `05-strategy-schema.md` for full schema

### Job Configuration
Location: `jobs/daily-review.toml`
Purpose: Define orchestrator pipeline
See: `10-orchestrator-job.md` for full schema

---

## Error Handling Strategy

### Fail-Safe Defaults
- Missing data for one ticker should not block entire pipeline
- Failed AI assessment should retry once, then mark as "insufficient_data"
- Network failures should retry with exponential backoff

### Error Propagation
```go
type TickerError struct {
    Ticker  string
    Stage   string
    Err     error
    Fatal   bool
}

// Non-fatal errors are collected and reported
// Fatal errors halt the pipeline for that ticker only
```

### Recovery
- Each stage checks for existing output tags before re-processing
- Resume capability from any stage

---

## Performance Targets

| Metric | Target | Notes |
|--------|--------|-------|
| Total runtime | < 15 min | For 25 holdings |
| Data fetch (parallel) | < 60s | Rate-limited by APIs |
| Signal computation | < 30s | CPU-bound |
| AI assessment | < 8 min | Batched, 5 per call |
| Report generation | < 30s | Template rendering |

---

## Security Considerations

1. **API Keys**: Store in environment variables, never in code
2. **Portfolio Data**: Treat as sensitive, no logging of holdings
3. **Email**: Use TLS for SMTP
4. **Cache**: Clear sensitive data after pipeline completion

---

## Testing Strategy

1. **Unit Tests**: Each computation function independently tested
2. **Integration Tests**: Stage-to-stage data flow
3. **Snapshot Tests**: Report output consistency
4. **Back-test Validation**: Signal accuracy against historical data

---

## Next Steps

1. Implement data models (`02-data-models.md`)
2. Build tool specifications (`03-tool-specifications.md`)
3. Implement computation algorithms (`04-computation-algorithms.md`)
4. Define strategy schema (`05-strategy-schema.md`)
5. Build announcement processing (`06-announcement-processing.md`)
6. Create AI assessment prompts (`07-ai-assessment-prompts.md`)
7. Implement report generation (`08-report-generation.md`)
8. Add validation layer (`09-validation-qa.md`)
9. Configure orchestrator (`10-orchestrator-job.md`)
