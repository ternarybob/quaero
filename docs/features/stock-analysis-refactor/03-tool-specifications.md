# ASX Portfolio Intelligence System — Tool Specifications

## Document Purpose
This document defines the interface specifications for all tools in the system. Each tool has defined inputs, outputs, error handling, and integration points.

---

## Tool Design Principles

1. **Single Responsibility**: Each tool does one thing well
2. **Idempotency**: Same inputs produce same outputs
3. **Compression**: Output only decision-relevant data
4. **Validation**: Validate inputs and outputs
5. **Error Isolation**: One ticker's failure doesn't block others

---

## Tool Registry

| Tool ID | Stage | Purpose | Async |
|---------|-------|---------|-------|
| `portfolio_load` | 0 | Load portfolio configuration | No |
| `eodhd_fetch` | 1 | Fetch market data | Yes |
| `asx_announcements_fetch` | 1 | Fetch announcements | Yes |
| `compute_signals` | 2 | Compute derived signals | No |
| `ai_assess_batch` | 3 | AI assessment of holdings | No |
| `portfolio_rollup` | 4 | Aggregate portfolio metrics | No |
| `assemble_report` | 5 | Generate final report | No |
| `send_email` | 6 | Deliver report | Yes |

---

## Tool: `portfolio_load`

### Purpose
Load portfolio holdings from configuration file or Navexa API.

### Interface

```go
type PortfolioLoadTool struct {
    configPath string
    navexaClient *navexa.Client // optional
}

type PortfolioLoadInput struct {
    Source           string `json:"source"` // "manual" or "navexa"
    ConfigPath       string `json:"config_path,omitempty"`
    NavexaPortfolioID string `json:"navexa_portfolio_id,omitempty"`
}

type PortfolioLoadOutput struct {
    Tag     string         `json:"tag"` // "portfolio-state"
    Content PortfolioState `json:"content"`
    Errors  []string       `json:"errors,omitempty"`
}

func (t *PortfolioLoadTool) Execute(input PortfolioLoadInput) (*PortfolioLoadOutput, error)
```

### Implementation

```go
func (t *PortfolioLoadTool) Execute(input PortfolioLoadInput) (*PortfolioLoadOutput, error) {
    var state PortfolioState
    
    switch input.Source {
    case "manual":
        // Load from TOML config
        config, err := t.loadTOMLConfig(input.ConfigPath)
        if err != nil {
            return nil, fmt.Errorf("failed to load config: %w", err)
        }
        state = t.configToState(config)
        
    case "navexa":
        // Fetch from Navexa API
        portfolio, err := t.navexaClient.GetPortfolio(input.NavexaPortfolioID)
        if err != nil {
            return nil, fmt.Errorf("failed to fetch from Navexa: %w", err)
        }
        state = t.navexaToState(portfolio)
        
    default:
        return nil, fmt.Errorf("unknown source: %s", input.Source)
    }
    
    // Compute derived values
    state.ComputeAggregations()
    
    // Validate
    if err := state.Validate(); err != nil {
        return nil, fmt.Errorf("invalid portfolio state: %w", err)
    }
    
    return &PortfolioLoadOutput{
        Tag:     "portfolio-state",
        Content: state,
    }, nil
}
```

### Output Tag Schema
See `PortfolioState` in `02-data-models.md`

---

## Tool: `eodhd_fetch`

### Purpose
Fetch and compress market data from EODHD API.

### Interface

```go
type EODHDFetchTool struct {
    client       *eodhd.Client
    apiKey       string
    lookbackDays int
}

type EODHDFetchInput struct {
    Ticker             string `json:"ticker"` // ASX format: "SRG.AU"
    LookbackDays       int    `json:"lookback_days"` // Default: 252
    IncludeFundamentals bool  `json:"include_fundamentals"`
    IncludeTechnicals   bool  `json:"include_technicals"`
}

type EODHDFetchOutput struct {
    Tag     string    `json:"tag"` // "ticker-raw-{ticker}"
    Content TickerRaw `json:"content"`
    Errors  []string  `json:"errors,omitempty"`
}

func (t *EODHDFetchTool) Execute(input EODHDFetchInput) (*EODHDFetchOutput, error)
```

### Implementation

```go
func (t *EODHDFetchTool) Execute(input EODHDFetchInput) (*EODHDFetchOutput, error) {
    ticker := input.Ticker
    lookback := input.LookbackDays
    if lookback == 0 {
        lookback = 252
    }
    
    // Fetch OHLCV
    endDate := time.Now()
    startDate := endDate.AddDate(0, 0, -lookback)
    
    ohlcv, err := t.client.GetEOD(ticker, startDate, endDate)
    if err != nil {
        return nil, fmt.Errorf("failed to fetch OHLCV: %w", err)
    }
    
    // Compress to derived values
    priceData := t.compressPriceData(ohlcv)
    volumeData := t.compressVolumeData(ohlcv)
    volatilityData := t.computeVolatility(ohlcv)
    
    // Fetch XJO for relative strength
    xjoData, err := t.client.GetEOD("XJO.INDX", startDate, endDate)
    if err != nil {
        // Non-fatal, continue without RS
        xjoData = nil
    }
    rsData := t.computeRelativeStrength(ohlcv, xjoData)
    
    // Fetch fundamentals if requested
    var fundamentals *FundamentalsData
    if input.IncludeFundamentals {
        fund, err := t.client.GetFundamentals(ticker)
        if err != nil {
            // Non-fatal for ETFs
            fundamentals = nil
        } else {
            fundamentals = t.compressFundamentals(fund)
        }
    }
    
    raw := TickerRaw{
        Ticker:          strings.TrimSuffix(ticker, ".AU"),
        FetchTimestamp:  time.Now(),
        Price:           priceData,
        Volume:          volumeData,
        Volatility:      volatilityData,
        RelativeStrength: rsData,
        Fundamentals:    fundamentals,
        HasFundamentals: fundamentals != nil,
        DataQuality:     t.assessDataQuality(ohlcv, fundamentals),
    }
    
    return &EODHDFetchOutput{
        Tag:     fmt.Sprintf("ticker-raw-%s", raw.Ticker),
        Content: raw,
    }, nil
}
```

### Data Compression Functions

```go
func (t *EODHDFetchTool) compressPriceData(ohlcv []OHLCV) PriceData {
    n := len(ohlcv)
    if n == 0 {
        return PriceData{}
    }
    
    closes := extractCloses(ohlcv)
    
    return PriceData{
        Current:     ohlcv[n-1].Close,
        PrevClose:   ohlcv[n-2].Close,
        Open:        ohlcv[n-1].Open,
        High:        ohlcv[n-1].High,
        Low:         ohlcv[n-1].Low,
        Change1DPct: pctChange(ohlcv[n-2].Close, ohlcv[n-1].Close),
        
        High52W: max(extractHighs(ohlcv)),
        Low52W:  min(extractLows(ohlcv)),
        EMA20:   ema(closes, 20),
        EMA50:   ema(closes, 50),
        EMA200:  ema(closes, 200),
        VWAP20:  vwap(ohlcv, 20),
        
        Return1WPct:  returnPct(closes, 5),
        Return4WPct:  returnPct(closes, 20),
        Return12WPct: returnPct(closes, 60),
        Return26WPct: returnPct(closes, 130),
        Return52WPct: returnPct(closes, 252),
    }
}

func (t *EODHDFetchTool) compressVolumeData(ohlcv []OHLCV) VolumeData {
    volumes := extractVolumes(ohlcv)
    n := len(volumes)
    
    sma20 := sma(volumes, 20)
    
    // Determine trend
    recent5Avg := avg(volumes[n-5:])
    trend := "flat"
    if recent5Avg > sma20*1.2 {
        trend = "rising"
    } else if recent5Avg < sma20*0.8 {
        trend = "falling"
    }
    
    return VolumeData{
        Current:      int64(volumes[n-1]),
        SMA20:        sma20,
        SMA50:        sma(volumes, 50),
        ZScore20:     zscore(volumes[n-1], volumes[n-20:]),
        Trend5Dvs20D: trend,
    }
}
```

### EODHD API Endpoints Used

| Endpoint | Purpose |
|----------|---------|
| `/eod/{ticker}` | Historical OHLCV |
| `/fundamentals/{ticker}` | Company fundamentals |
| `/technical/{ticker}` | Pre-computed technicals (optional) |

### Rate Limiting

```go
// Rate limiter for EODHD API
type RateLimiter struct {
    requests int
    window   time.Duration
    mu       sync.Mutex
    count    int
    resetAt  time.Time
}

func (r *RateLimiter) Wait() {
    r.mu.Lock()
    defer r.mu.Unlock()
    
    if time.Now().After(r.resetAt) {
        r.count = 0
        r.resetAt = time.Now().Add(r.window)
    }
    
    if r.count >= r.requests {
        sleepDuration := time.Until(r.resetAt)
        time.Sleep(sleepDuration)
        r.count = 0
        r.resetAt = time.Now().Add(r.window)
    }
    
    r.count++
}
```

---

## Tool: `asx_announcements_fetch`

### Purpose
Fetch and classify ASX announcements for a ticker.

### Interface

```go
type ASXAnnouncementsTool struct {
    scraper    *asx.Scraper
    classifier *AnnouncementClassifier
}

type ASXAnnouncementsInput struct {
    Ticker           string `json:"ticker"` // ASX format: "SRG"
    DaysBack         int    `json:"days_back"` // Default: 30
    IncludeBodySummary bool `json:"include_body_summary"`
}

type ASXAnnouncementsOutput struct {
    Tag     string              `json:"tag"` // "ticker-announcements-{ticker}"
    Content TickerAnnouncements `json:"content"`
    Errors  []string            `json:"errors,omitempty"`
}

func (t *ASXAnnouncementsTool) Execute(input ASXAnnouncementsInput) (*ASXAnnouncementsOutput, error)
```

### Implementation

```go
func (t *ASXAnnouncementsTool) Execute(input ASXAnnouncementsInput) (*ASXAnnouncementsOutput, error) {
    ticker := input.Ticker
    daysBack := input.DaysBack
    if daysBack == 0 {
        daysBack = 30
    }
    
    // Fetch raw announcements from ASX
    since := time.Now().AddDate(0, 0, -daysBack)
    rawAnnouncements, err := t.scraper.GetAnnouncements(ticker, since)
    if err != nil {
        return nil, fmt.Errorf("failed to fetch announcements: %w", err)
    }
    
    // Fetch price data for reaction validation
    priceData, err := t.getPriceDataForReactions(ticker, rawAnnouncements)
    if err != nil {
        // Non-fatal, continue without reactions
        priceData = nil
    }
    
    // Process each announcement
    processed := make([]Announcement, 0, len(rawAnnouncements))
    for _, raw := range rawAnnouncements {
        ann := t.processAnnouncement(raw, priceData, input.IncludeBodySummary)
        processed = append(processed, ann)
    }
    
    // Compute summary stats
    summary := t.computeSummary(processed)
    
    // Detect PR-heavy issuer
    prHeavy := t.detectPRHeavyIssuer(processed, daysBack)
    
    content := TickerAnnouncements{
        Ticker:              ticker,
        FetchTimestamp:      time.Now(),
        AnnouncementCount30D: len(processed),
        PRHeavyIssuer:       prHeavy,
        Announcements:       processed,
        Summary:             summary,
    }
    
    return &ASXAnnouncementsOutput{
        Tag:     fmt.Sprintf("ticker-announcements-%s", ticker),
        Content: content,
    }, nil
}

func (t *ASXAnnouncementsTool) processAnnouncement(
    raw RawAnnouncement, 
    priceData []OHLCV,
    includeSummary bool,
) Announcement {
    // Classify type
    annType := t.classifier.ClassifyType(raw.Headline, raw.Body)
    
    // Compute substance score
    substanceScore := t.classifier.ComputeSubstanceScore(raw)
    
    // Compute PR entropy
    prEntropy := t.classifier.ComputePREntropy(raw.Body)
    
    // Compute reaction if price data available
    var reaction ReactionData
    var reactionScore float64
    if priceData != nil {
        reaction = t.computeReaction(raw.Date, priceData)
        reactionScore = t.scoreReaction(reaction)
    }
    
    // Compute SNI
    sni := substanceScore * reactionScore * (1 - prEntropy)
    
    // Determine signal class
    signalClass := t.classifySignal(sni, reaction)
    
    // Generate summary if requested
    var summary string
    if includeSummary {
        summary = t.generateSummary(raw)
    }
    
    return Announcement{
        Date:           raw.Date.Format("2006-01-02"),
        Headline:       raw.Headline,
        Type:           string(annType),
        SubstanceScore: substanceScore,
        PREntropyScore: prEntropy,
        Reaction:       reaction,
        ReactionScore:  reactionScore,
        SNI:            sni,
        Summary:        summary,
        SignalClass:    signalClass,
    }
}
```

### ASX Scraping Details

```go
type ASXScraper struct {
    baseURL    string
    httpClient *http.Client
    userAgent  string
}

func (s *ASXScraper) GetAnnouncements(ticker string, since time.Time) ([]RawAnnouncement, error) {
    // ASX announcements URL
    url := fmt.Sprintf("%s/asx/statistics/announcements.do?by=asxCode&asxCode=%s&timeframe=D&period=M3",
        s.baseURL, ticker)
    
    // Respectful scraping
    time.Sleep(500 * time.Millisecond)
    
    resp, err := s.httpClient.Get(url)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    // Parse HTML/JSON response
    announcements := s.parseAnnouncementsPage(resp.Body)
    
    // Filter by date
    filtered := make([]RawAnnouncement, 0)
    for _, ann := range announcements {
        if ann.Date.After(since) {
            filtered = append(filtered, ann)
        }
    }
    
    return filtered, nil
}
```

---

## Tool: `compute_signals`

### Purpose
Compute all derived signals from raw data.

### Interface

```go
type ComputeSignalsTool struct {
    pbas   *PBASComputer
    vli    *VLIComputer
    regime *RegimeClassifier
    cooked *CookedDetector
}

type ComputeSignalsInput struct {
    Ticker           string `json:"ticker"`
    RawDataTag       string `json:"raw_data_tag"`       // "ticker-raw-{ticker}"
    AnnouncementsTag string `json:"announcements_tag"` // "ticker-announcements-{ticker}"
}

type ComputeSignalsOutput struct {
    Tag     string        `json:"tag"` // "ticker-signals-{ticker}"
    Content TickerSignals `json:"content"`
    Errors  []string      `json:"errors,omitempty"`
}

func (t *ComputeSignalsTool) Execute(
    input ComputeSignalsInput, 
    store TagStore,
) (*ComputeSignalsOutput, error)
```

### Implementation

```go
func (t *ComputeSignalsTool) Execute(
    input ComputeSignalsInput,
    store TagStore,
) (*ComputeSignalsOutput, error) {
    // Load raw data
    rawDoc, err := store.Get(input.RawDataTag)
    if err != nil {
        return nil, fmt.Errorf("failed to load raw data: %w", err)
    }
    var raw TickerRaw
    if err := yaml.Unmarshal(rawDoc.Content, &raw); err != nil {
        return nil, err
    }
    
    // Load announcements
    annDoc, err := store.Get(input.AnnouncementsTag)
    if err != nil {
        // Non-fatal
        annDoc = nil
    }
    var announcements TickerAnnouncements
    if annDoc != nil {
        yaml.Unmarshal(annDoc.Content, &announcements)
    }
    
    // Compute PBAS
    pbas := t.pbas.Compute(raw)
    
    // Compute VLI
    vli := t.vli.Compute(raw)
    
    // Compute Regime
    regime := t.regime.Classify(raw)
    
    // Compute Cooked
    cooked := t.cooked.Detect(raw, pbas)
    
    // Compute RS
    rs := t.computeRS(raw)
    
    // Compute Quality
    quality := t.computeQuality(raw)
    
    // Compute Justified Return
    justifiedReturn := t.computeJustifiedReturn(raw, pbas)
    
    // Extract announcement signals
    annSignals := t.extractAnnouncementSignals(announcements)
    
    // Compile risk flags
    riskFlags := t.compileRiskFlags(raw, pbas, vli, regime, cooked)
    
    signals := TickerSignals{
        Ticker:           input.Ticker,
        ComputeTimestamp: time.Now(),
        Price:            t.extractPriceSignals(raw),
        PBAS:             pbas,
        VLI:              vli,
        Regime:           regime,
        RS:               rs,
        Cooked:           cooked,
        Quality:          quality,
        Announcements:    annSignals,
        JustifiedReturn:  justifiedReturn,
        RiskFlags:        riskFlags,
    }
    
    return &ComputeSignalsOutput{
        Tag:     fmt.Sprintf("ticker-signals-%s", input.Ticker),
        Content: signals,
    }, nil
}
```

---

## Tool: `ai_assess_batch`

### Purpose
Generate AI assessments for a batch of holdings.

### Interface

```go
type AIAssessBatchTool struct {
    client    *anthropic.Client
    validator *AssessmentValidator
    strategy  *Strategy
}

type AIAssessBatchInput struct {
    Tickers       []string `json:"tickers"`
    SignalTags    []string `json:"signal_tags"` // ["ticker-signals-X", ...]
    HoldingTypes  map[string]string `json:"holding_types"` // ticker -> "smsf"|"trader"
    ThinkingBudget int `json:"thinking_budget"` // Default: 16000
}

type AIAssessBatchOutput struct {
    Assessments []TickerAssessment `json:"assessments"`
    Tags        []string           `json:"tags"` // ["ticker-assessment-X", ...]
    Errors      []string           `json:"errors,omitempty"`
}

func (t *AIAssessBatchTool) Execute(
    input AIAssessBatchInput,
    store TagStore,
) (*AIAssessBatchOutput, error)
```

### Implementation

```go
func (t *AIAssessBatchTool) Execute(
    input AIAssessBatchInput,
    store TagStore,
) (*AIAssessBatchOutput, error) {
    // Load signals for batch
    signals := make([]TickerSignals, 0, len(input.Tickers))
    for _, tag := range input.SignalTags {
        doc, err := store.Get(tag)
        if err != nil {
            continue // Skip missing
        }
        var sig TickerSignals
        yaml.Unmarshal(doc.Content, &sig)
        signals = append(signals, sig)
    }
    
    // Build prompt
    prompt := t.buildAssessmentPrompt(signals, input.HoldingTypes, t.strategy)
    
    // Call Claude
    response, err := t.client.Messages.Create(context.Background(), anthropic.MessageCreateParams{
        Model:     "claude-sonnet-4-20250514",
        MaxTokens: 8000,
        Messages: []anthropic.Message{
            {Role: "user", Content: prompt},
        },
        Thinking: &anthropic.ThinkingConfig{
            Type:        "enabled",
            BudgetTokens: input.ThinkingBudget,
        },
    })
    if err != nil {
        return nil, fmt.Errorf("AI assessment failed: %w", err)
    }
    
    // Parse response
    assessments := t.parseAssessments(response.Content)
    
    // Validate each assessment
    validatedAssessments := make([]TickerAssessment, 0, len(assessments))
    tags := make([]string, 0, len(assessments))
    var errors []string
    
    for _, assessment := range assessments {
        validation := t.validator.Validate(assessment)
        assessment.ValidationPassed = validation.Valid
        assessment.ValidationErrors = validation.Errors
        
        if !validation.Valid {
            // Retry with feedback
            corrected, err := t.retryWithFeedback(assessment, validation.Errors, signals)
            if err != nil {
                errors = append(errors, fmt.Sprintf("%s: %v", assessment.Ticker, err))
                continue
            }
            assessment = corrected
        }
        
        validatedAssessments = append(validatedAssessments, assessment)
        tags = append(tags, fmt.Sprintf("ticker-assessment-%s", assessment.Ticker))
    }
    
    return &AIAssessBatchOutput{
        Assessments: validatedAssessments,
        Tags:        tags,
        Errors:      errors,
    }, nil
}
```

### Batch Processing

```go
const OptimalBatchSize = 5

func ProcessAllHoldings(
    holdings []Holding,
    tool *AIAssessBatchTool,
    store TagStore,
) ([]TickerAssessment, error) {
    // Create batches
    batches := createBatches(holdings, OptimalBatchSize)
    
    allAssessments := make([]TickerAssessment, 0, len(holdings))
    
    for i, batch := range batches {
        log.Printf("Processing batch %d/%d", i+1, len(batches))
        
        tickers := extractTickers(batch)
        signalTags := makeSignalTags(tickers)
        holdingTypes := makeHoldingTypeMap(batch)
        
        output, err := tool.Execute(AIAssessBatchInput{
            Tickers:      tickers,
            SignalTags:   signalTags,
            HoldingTypes: holdingTypes,
        }, store)
        
        if err != nil {
            log.Printf("Batch %d failed: %v", i+1, err)
            continue
        }
        
        allAssessments = append(allAssessments, output.Assessments...)
        
        // Rate limiting between batches
        time.Sleep(time.Second)
    }
    
    return allAssessments, nil
}
```

---

## Tool: `portfolio_rollup`

### Purpose
Aggregate individual assessments into portfolio-level metrics.

### Interface

```go
type PortfolioRollupTool struct{}

type PortfolioRollupInput struct {
    PortfolioStateTag string   `json:"portfolio_state_tag"`
    AssessmentTags    []string `json:"assessment_tags"`
}

type PortfolioRollupOutput struct {
    Tag     string          `json:"tag"` // "portfolio-rollup"
    Content PortfolioRollup `json:"content"`
}

func (t *PortfolioRollupTool) Execute(
    input PortfolioRollupInput,
    store TagStore,
) (*PortfolioRollupOutput, error)
```

### Implementation

See `04-computation-algorithms.md` for detailed rollup calculations.

---

## Tool: `assemble_report`

### Purpose
Generate the final daily report from all components.

### Interface

```go
type AssembleReportTool struct {
    templates *template.Template
}

type AssembleReportInput struct {
    PortfolioStateTag  string `json:"portfolio_state_tag"`
    PortfolioRollupTag string `json:"portfolio_rollup_tag"`
    AssessmentTags     []string `json:"assessment_tags"`
    SignalTags         []string `json:"signal_tags"`
    IncludeScreening   bool `json:"include_screening"`
}

type AssembleReportOutput struct {
    Tag     string      `json:"tag"` // "daily-report"
    Content DailyReport `json:"content"`
}

func (t *AssembleReportTool) Execute(
    input AssembleReportInput,
    store TagStore,
) (*AssembleReportOutput, error)
```

### Implementation

See `08-report-generation.md` for detailed report assembly logic.

---

## Tool: `send_email`

### Purpose
Deliver the daily report via email.

### Interface

```go
type SendEmailTool struct {
    smtpHost     string
    smtpPort     int
    smtpUser     string
    smtpPassword string
    fromAddress  string
}

type SendEmailInput struct {
    To          string `json:"to"`
    Subject     string `json:"subject"`
    BodyFromTag string `json:"body_from_tag"` // "daily-report"
    Format      string `json:"format"` // "text" or "html"
}

type SendEmailOutput struct {
    Sent      bool      `json:"sent"`
    Timestamp time.Time `json:"timestamp"`
    Error     string    `json:"error,omitempty"`
}

func (t *SendEmailTool) Execute(
    input SendEmailInput,
    store TagStore,
) (*SendEmailOutput, error)
```

---

## Error Handling

### Error Types

```go
type ToolError struct {
    Tool    string `json:"tool"`
    Ticker  string `json:"ticker,omitempty"`
    Stage   string `json:"stage"`
    Message string `json:"message"`
    Fatal   bool   `json:"fatal"`
}

func (e ToolError) Error() string {
    if e.Ticker != "" {
        return fmt.Sprintf("[%s/%s] %s: %s", e.Stage, e.Ticker, e.Tool, e.Message)
    }
    return fmt.Sprintf("[%s] %s: %s", e.Stage, e.Tool, e.Message)
}
```

### Error Recovery

```go
// RetryConfig defines retry behavior
type RetryConfig struct {
    MaxRetries     int
    InitialBackoff time.Duration
    MaxBackoff     time.Duration
    BackoffFactor  float64
}

func WithRetry[T any](fn func() (T, error), config RetryConfig) (T, error) {
    var result T
    var lastErr error
    backoff := config.InitialBackoff
    
    for i := 0; i <= config.MaxRetries; i++ {
        result, lastErr = fn()
        if lastErr == nil {
            return result, nil
        }
        
        if i < config.MaxRetries {
            time.Sleep(backoff)
            backoff = time.Duration(float64(backoff) * config.BackoffFactor)
            if backoff > config.MaxBackoff {
                backoff = config.MaxBackoff
            }
        }
    }
    
    return result, lastErr
}
```

---

## Next Document
Proceed to `04-computation-algorithms.md` for signal computation implementations.
