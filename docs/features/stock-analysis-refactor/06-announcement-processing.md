# ASX Portfolio Intelligence System — Announcement Processing

## Document Purpose
This document defines the algorithms for processing ASX announcements, classifying them by type, computing substance vs PR scores, and calculating the Signal-to-Noise Index (SNI).

---

## Design Goals

1. **Separate Signal from Noise**: Identify announcements that matter vs PR fluff
2. **Quantify Substance**: Score based on specific, actionable information
3. **Validate with Price**: Confirm signal quality via market reaction
4. **Flag PR-Heavy Issuers**: Identify companies that over-communicate without substance

---

## Announcement Types

| Type | Description | Typical SNI | Example |
|------|-------------|-------------|---------|
| `quantified_contract` | Contract with $ value and terms | High | "Wins $45M 3-year contract" |
| `guidance_change` | Earnings/revenue guidance revision | High | "Upgrades FY25 EBITDA to $120M" |
| `results` | Half-year/full-year results | High | "FY24 Results" |
| `appendix_4c` | Quarterly cash flow (Appendix 4C) | High | "Quarterly Activities Report" |
| `appendix_4e` | Annual report (Appendix 4E) | High | "Preliminary Final Report" |
| `capital_raise` | Placement, rights issue, SPP | Medium-High | "Completion of $30M placement" |
| `material_change` | Director changes, M&A, restructure | Medium-High | "CEO Transition" |
| `substantial_holder` | Institutional position change | Medium | "Change in substantial holding" |
| `director_interest` | Director buying/selling | Medium | "Director's Interest Notice" |
| `trading_halt` | Trading halt | Medium | "Trading Halt" |
| `strategic_review` | Review announcements | Low-Medium | "Strategic Review Update" |
| `corporate_action` | AGM, dividends, admin | Low | "AGM Results" |
| `pr_update` | Generic update, no numbers | Low | "Company Update" |
| `administrative` | Address change, registry | Very Low | "Change of Registered Office" |

---

## Announcement Type Classifier

### Rule-Based Classification

```go
package announcements

import (
    "regexp"
    "strings"
)

// AnnouncementClassifier classifies ASX announcements
type AnnouncementClassifier struct {
    patterns map[AnnouncementType][]*regexp.Regexp
}

// NewAnnouncementClassifier creates a new classifier
func NewAnnouncementClassifier() *AnnouncementClassifier {
    c := &AnnouncementClassifier{
        patterns: make(map[AnnouncementType][]*regexp.Regexp),
    }
    c.initPatterns()
    return c
}

func (c *AnnouncementClassifier) initPatterns() {
    // Results patterns
    c.patterns[AnnTypeResults] = []*regexp.Regexp{
        regexp.MustCompile(`(?i)(half[- ]?year|full[- ]?year|annual|interim)\s*(results?|report)`),
        regexp.MustCompile(`(?i)(H1|H2|FY\d{2,4})\s*(results?|report)`),
        regexp.MustCompile(`(?i)preliminary\s*final`),
    }
    
    // Appendix 4C
    c.patterns[AnnTypeAppendix4C] = []*regexp.Regexp{
        regexp.MustCompile(`(?i)appendix\s*4c`),
        regexp.MustCompile(`(?i)quarterly\s*(activities?|cash\s*flow)\s*report`),
        regexp.MustCompile(`(?i)quarterly\s*report`),
    }
    
    // Appendix 4E
    c.patterns[AnnTypeAppendix4E] = []*regexp.Regexp{
        regexp.MustCompile(`(?i)appendix\s*4e`),
        regexp.MustCompile(`(?i)preliminary\s*final\s*report`),
    }
    
    // Quantified contract
    c.patterns[AnnTypeQuantifiedContract] = []*regexp.Regexp{
        regexp.MustCompile(`(?i)contract\s*(award|win|signed).*\$[\d,.]+[MBmb]?`),
        regexp.MustCompile(`(?i)(secures?|awarded?|wins?)\s*\$[\d,.]+[MBmb]?\s*(contract|agreement|deal)`),
        regexp.MustCompile(`(?i)\$[\d,.]+[MBmb]?\s*(contract|agreement|deal)`),
    }
    
    // Guidance change
    c.patterns[AnnTypeGuidanceChange] = []*regexp.Regexp{
        regexp.MustCompile(`(?i)(upgrades?|downgrades?|revis(es?|ion)|reaffirms?)\s*(guidance|outlook|forecast)`),
        regexp.MustCompile(`(?i)guidance\s*(upgrade|downgrade|revision|update)`),
        regexp.MustCompile(`(?i)(earnings|revenue|ebitda)\s*guidance`),
    }
    
    // Capital raise
    c.patterns[AnnTypeCapitalRaise] = []*regexp.Regexp{
        regexp.MustCompile(`(?i)(placement|raising?|capital\s*raise|rights\s*issue|spp)`),
        regexp.MustCompile(`(?i)(completion|launch)\s*of\s*\$[\d,.]+[MBmb]?`),
        regexp.MustCompile(`(?i)share\s*(purchase|placement)\s*plan`),
    }
    
    // Substantial holder
    c.patterns[AnnTypeSubstantialHolder] = []*regexp.Regexp{
        regexp.MustCompile(`(?i)substantial\s*(holder|holding|shareholder)`),
        regexp.MustCompile(`(?i)(becoming|ceasing)\s*.*substantial`),
    }
    
    // Director interest
    c.patterns[AnnTypeDirectorInterest] = []*regexp.Regexp{
        regexp.MustCompile(`(?i)director['']?s?\s*interest`),
        regexp.MustCompile(`(?i)appendix\s*3[yYzZ]`),
        regexp.MustCompile(`(?i)change\s*of\s*director['']?s?\s*interest`),
    }
    
    // Trading halt
    c.patterns[AnnTypeTradingHalt] = []*regexp.Regexp{
        regexp.MustCompile(`(?i)trading\s*halt`),
        regexp.MustCompile(`(?i)suspension\s*from\s*(trading|quotation)`),
    }
    
    // Material change (CEO, acquisitions, etc)
    c.patterns[AnnTypeMaterialChange] = []*regexp.Regexp{
        regexp.MustCompile(`(?i)(ceo|cfo|managing\s*director)\s*(appointment|resignation|transition|change)`),
        regexp.MustCompile(`(?i)(acquisition|merger|takeover)`),
        regexp.MustCompile(`(?i)material\s*(transaction|change|announcement)`),
    }
    
    // Administrative
    c.patterns[AnnTypeAdministrative] = []*regexp.Regexp{
        regexp.MustCompile(`(?i)change\s*of\s*(registered\s*office|company\s*secretary|auditor)`),
        regexp.MustCompile(`(?i)appendix\s*3b`),
        regexp.MustCompile(`(?i)cleansing\s*(notice|statement)`),
    }
    
    // Corporate action
    c.patterns[AnnTypeCorporateAction] = []*regexp.Regexp{
        regexp.MustCompile(`(?i)agm\s*(notice|results?|presentation)`),
        regexp.MustCompile(`(?i)annual\s*general\s*meeting`),
        regexp.MustCompile(`(?i)dividend\s*(announcement|notice|payment)`),
    }
}

// ClassifyType determines the announcement type from headline
func (c *AnnouncementClassifier) ClassifyType(headline, body string) AnnouncementType {
    text := headline + " " + body
    
    // Check patterns in priority order
    priorityOrder := []AnnouncementType{
        AnnTypeResults,
        AnnTypeAppendix4C,
        AnnTypeAppendix4E,
        AnnTypeGuidanceChange,
        AnnTypeQuantifiedContract,
        AnnTypeCapitalRaise,
        AnnTypeTradingHalt,
        AnnTypeSubstantialHolder,
        AnnTypeDirectorInterest,
        AnnTypeMaterialChange,
        AnnTypeAdministrative,
        AnnTypeCorporateAction,
    }
    
    for _, annType := range priorityOrder {
        for _, pattern := range c.patterns[annType] {
            if pattern.MatchString(text) {
                return annType
            }
        }
    }
    
    // Default to PR update
    return AnnTypePRUpdate
}
```

---

## Substance Score Calculator

### Purpose
Quantify how much actionable information an announcement contains.

### Scoring Factors

| Factor | Weight | Description |
|--------|--------|-------------|
| Quantification | 0.30 | Contains specific numbers ($ amounts, percentages) |
| Timing | 0.20 | Includes specific dates or timeframes |
| Counterparty | 0.15 | Names specific customers, partners |
| Binding Terms | 0.15 | Contract is signed vs MOU/non-binding |
| Cash Impact | 0.10 | Clear impact on cash flow |
| Guidance Change | 0.10 | Updates forward expectations |

### Implementation

```go
package announcements

import (
    "regexp"
    "strings"
)

// SubstanceScorer computes substance score for announcements
type SubstanceScorer struct {
    dollarPattern    *regexp.Regexp
    percentPattern   *regexp.Regexp
    datePattern      *regexp.Regexp
    timeframePattern *regexp.Regexp
}

// NewSubstanceScorer creates a new substance scorer
func NewSubstanceScorer() *SubstanceScorer {
    return &SubstanceScorer{
        dollarPattern:    regexp.MustCompile(`\$[\d,.]+\s*[MBKmk]?(illion)?`),
        percentPattern:   regexp.MustCompile(`\d+(\.\d+)?%`),
        datePattern:      regexp.MustCompile(`(?i)(january|february|march|april|may|june|july|august|september|october|november|december|\d{1,2}/\d{1,2}/\d{2,4}|Q[1-4]\s*\d{2,4}|FY\d{2,4})`),
        timeframePattern: regexp.MustCompile(`(?i)(\d+[- ]?(year|month|week|day)|immediate|commenc|start|begin)`),
    }
}

// ComputeSubstanceScore calculates substance score for an announcement
func (s *SubstanceScorer) ComputeSubstanceScore(raw RawAnnouncement) float64 {
    headline := raw.Headline
    body := raw.Body
    text := headline + " " + body
    textLower := strings.ToLower(text)
    
    score := 0.0
    
    // Quantification (0.30)
    dollarMatches := s.dollarPattern.FindAllString(text, -1)
    percentMatches := s.percentPattern.FindAllString(text, -1)
    if len(dollarMatches) > 0 {
        score += 0.20
        if len(dollarMatches) > 2 {
            score += 0.05 // Multiple figures
        }
    }
    if len(percentMatches) > 0 {
        score += 0.10
    }
    
    // Timing (0.20)
    if s.datePattern.MatchString(text) {
        score += 0.10
    }
    if s.timeframePattern.MatchString(text) {
        score += 0.10
    }
    
    // Counterparty (0.15)
    // Look for company names (capitalized words that aren't common)
    if containsCounterparty(textLower) {
        score += 0.15
    }
    
    // Binding Terms (0.15)
    if containsBindingTerms(textLower) {
        score += 0.15
    } else if containsNonBindingTerms(textLower) {
        score -= 0.05 // Penalty for MOU/non-binding
    }
    
    // Cash Impact (0.10)
    if containsCashImpact(textLower) {
        score += 0.10
    }
    
    // Guidance Change (0.10)
    if containsGuidanceChange(textLower) {
        score += 0.10
    }
    
    // Cap at 1.0
    if score > 1.0 {
        score = 1.0
    }
    if score < 0.0 {
        score = 0.0
    }
    
    return score
}

func containsCounterparty(text string) bool {
    // Major counterparties
    counterparties := []string{
        "rio tinto", "bhp", "fortescue", "woodside", "santos",
        "woolworths", "coles", "wesfarmers", "telstra", "optus",
        "qantas", "virgin", "commonwealth", "westpac", "anz", "nab",
        "government", "defence", "department", "council",
    }
    for _, cp := range counterparties {
        if strings.Contains(text, cp) {
            return true
        }
    }
    return false
}

func containsBindingTerms(text string) bool {
    binding := []string{
        "signed", "executed", "binding", "awarded", "secured",
        "contracted", "confirmed", "finalised", "completed",
    }
    for _, term := range binding {
        if strings.Contains(text, term) {
            return true
        }
    }
    return false
}

func containsNonBindingTerms(text string) bool {
    nonBinding := []string{
        "mou", "memorandum of understanding", "non-binding",
        "indicative", "preliminary", "in principle", "exploring",
        "potential", "possible", "may", "could",
    }
    for _, term := range nonBinding {
        if strings.Contains(text, term) {
            return true
        }
    }
    return false
}

func containsCashImpact(text string) bool {
    cashTerms := []string{
        "cash", "payment", "receipt", "invoice", "milestone",
        "revenue", "proceeds", "funding", "cash flow",
    }
    for _, term := range cashTerms {
        if strings.Contains(text, term) {
            return true
        }
    }
    return false
}

func containsGuidanceChange(text string) bool {
    guidance := []string{
        "guidance", "outlook", "forecast", "expects", "anticipates",
        "targets", "upgrades", "downgrades", "revises", "reaffirms",
    }
    for _, term := range guidance {
        if strings.Contains(text, term) {
            return true
        }
    }
    return false
}
```

---

## PR Entropy Score Calculator

### Purpose
Detect PR-speak and promotional language that adds noise.

### High-Entropy Indicators (PR-Speak)

- Excessive adjectives: "excited", "pleased", "significant", "transformational"
- Forward-looking without specifics: "aims to", "expects to", "positioned to"
- Vague superlatives: "leading", "best-in-class", "world-class"
- Buzzwords without substance: "synergies", "leverage", "optimize"

### Implementation

```go
package announcements

import (
    "regexp"
    "strings"
)

// PREntropyScorer computes PR entropy score
type PREntropyScorer struct {
    adjectivePattern     *regexp.Regexp
    forwardLookingPattern *regexp.Regexp
    superlativePattern    *regexp.Regexp
    buzzwordPattern       *regexp.Regexp
}

// NewPREntropyScorer creates a new PR entropy scorer
func NewPREntropyScorer() *PREntropyScorer {
    return &PREntropyScorer{
        adjectivePattern: regexp.MustCompile(`(?i)\b(excited|pleased|thrilled|delighted|proud|significant|substantial|excellent|outstanding|exceptional|strong|robust|solid|impressive|remarkable|tremendous|fantastic|great|wonderful)\b`),
        
        forwardLookingPattern: regexp.MustCompile(`(?i)\b(aims? to|expects? to|positioned to|poised to|well.?placed|looking forward|anticipates?|believes?|confident|optimistic|on track|progressing|advancing)\b`),
        
        superlativePattern: regexp.MustCompile(`(?i)\b(leading|market.?leading|best.?in.?class|world.?class|premier|top.?tier|cutting.?edge|innovative|revolutionary|game.?changing|disruptive|transformational|transformative)\b`),
        
        buzzwordPattern: regexp.MustCompile(`(?i)\b(synergies?|leverage|optimize|streamline|ecosystem|paradigm|holistic|scalable|sustainable|strategic|dynamic|proactive|synergistic|value.?add|unlock)\b`),
    }
}

// ComputePREntropy calculates PR entropy score
func (s *PREntropyScorer) ComputePREntropy(body string) float64 {
    if len(body) == 0 {
        return 0.5 // Neutral if no body
    }
    
    // Count words
    words := strings.Fields(body)
    wordCount := len(words)
    if wordCount == 0 {
        return 0.5
    }
    
    // Count PR indicators
    adjectiveMatches := len(s.adjectivePattern.FindAllString(body, -1))
    forwardMatches := len(s.forwardLookingPattern.FindAllString(body, -1))
    superlativeMatches := len(s.superlativePattern.FindAllString(body, -1))
    buzzwordMatches := len(s.buzzwordPattern.FindAllString(body, -1))
    
    totalPRWords := adjectiveMatches + forwardMatches + superlativeMatches + buzzwordMatches
    
    // PR density = PR words / total words
    prDensity := float64(totalPRWords) / float64(wordCount)
    
    // Scale to 0-1 range
    // 0% PR = 0.0 entropy
    // 5% PR = 0.5 entropy  
    // 10%+ PR = 1.0 entropy
    entropy := prDensity * 10.0
    if entropy > 1.0 {
        entropy = 1.0
    }
    
    return entropy
}
```

---

## Reaction Validator

### Purpose
Validate announcement substance by measuring market reaction.

### Reaction Metrics

| Metric | Calculation | Threshold |
|--------|-------------|-----------|
| Price T+1 | Close[T+1] / Close[T] - 1 | Abs > 2% = significant |
| Price T+3 | Close[T+3] / Close[T] - 1 | Confirm direction |
| Volume T+1 | Volume[T+1] / VolSMA20 | > 1.5x = confirmed |
| Held 50% | Price[T+3] retains 50% of T+1 move | True = sustained |

### Implementation

```go
package announcements

import (
    "math"
    "time"
)

// ReactionValidator validates announcement reactions
type ReactionValidator struct{}

// NewReactionValidator creates a new reaction validator
func NewReactionValidator() *ReactionValidator {
    return &ReactionValidator{}
}

// ComputeReaction calculates reaction metrics
func (v *ReactionValidator) ComputeReaction(
    annDate time.Time,
    priceData []OHLCV,
) ReactionData {
    // Find announcement date in price data
    annIdx := v.findDateIndex(annDate, priceData)
    if annIdx < 0 || annIdx >= len(priceData)-3 {
        return ReactionData{} // Not enough data
    }
    
    priceT0 := priceData[annIdx].Close
    priceT1 := priceData[annIdx+1].Close
    priceT3 := priceData[annIdx+3].Close
    volumeT1 := float64(priceData[annIdx+1].Volume)
    
    // Compute volume SMA20
    volSMA20 := v.computeVolSMA(priceData, annIdx, 20)
    
    // Price changes
    priceT1Pct := ((priceT1 / priceT0) - 1) * 100
    priceT3Pct := ((priceT3 / priceT0) - 1) * 100
    
    // Volume ratio
    volumeRatio := volumeT1 / volSMA20
    
    // Check if move held 50%
    held50 := false
    if priceT1Pct > 0 {
        held50 = priceT3Pct >= (priceT1Pct * 0.5)
    } else if priceT1Pct < 0 {
        held50 = priceT3Pct <= (priceT1Pct * 0.5)
    }
    
    return ReactionData{
        PriceT1Pct:    round(priceT1Pct, 2),
        PriceT3Pct:    round(priceT3Pct, 2),
        VolumeT1Ratio: round(volumeRatio, 2),
        Held50Pct:     held50,
    }
}

// ScoreReaction converts reaction data to a 0-1 score
func (v *ReactionValidator) ScoreReaction(r ReactionData) float64 {
    score := 0.0
    
    // Price move significance (0.4)
    absMove := math.Abs(r.PriceT1Pct)
    if absMove >= 5.0 {
        score += 0.40
    } else if absMove >= 3.0 {
        score += 0.30
    } else if absMove >= 2.0 {
        score += 0.20
    } else if absMove >= 1.0 {
        score += 0.10
    }
    
    // Volume confirmation (0.3)
    if r.VolumeT1Ratio >= 2.5 {
        score += 0.30
    } else if r.VolumeT1Ratio >= 2.0 {
        score += 0.25
    } else if r.VolumeT1Ratio >= 1.5 {
        score += 0.20
    } else if r.VolumeT1Ratio >= 1.2 {
        score += 0.10
    }
    
    // Move sustainability (0.3)
    if r.Held50Pct {
        score += 0.30
    } else {
        // Partial credit if T+3 is same direction as T+1
        if (r.PriceT1Pct > 0 && r.PriceT3Pct > 0) ||
           (r.PriceT1Pct < 0 && r.PriceT3Pct < 0) {
            score += 0.15
        }
    }
    
    return score
}

func (v *ReactionValidator) findDateIndex(date time.Time, data []OHLCV) int {
    for i, d := range data {
        if sameDay(d.Date, date) {
            return i
        }
        // If announcement after market, use next day
        if d.Date.After(date) {
            return i
        }
    }
    return -1
}

func (v *ReactionValidator) computeVolSMA(data []OHLCV, idx, period int) float64 {
    start := idx - period
    if start < 0 {
        start = 0
    }
    
    sum := 0.0
    count := 0
    for i := start; i < idx; i++ {
        sum += float64(data[i].Volume)
        count++
    }
    
    if count == 0 {
        return 1 // Avoid division by zero
    }
    return sum / float64(count)
}

func sameDay(a, b time.Time) bool {
    return a.Year() == b.Year() && a.YearDay() == b.YearDay()
}
```

---

## Signal-to-Noise Index (SNI)

### Formula

```
SNI = SubstanceScore × ReactionScore × (1 - PREntropyScore)

Where:
  SubstanceScore ∈ [0, 1]
  ReactionScore ∈ [0, 1]
  PREntropyScore ∈ [0, 1]
  
Result: SNI ∈ [0, 1]
  > 0.60 = High-signal announcement
  0.30-0.60 = Moderate signal
  < 0.30 = Noise
```

### Signal Classification

```go
// ClassifySignal determines signal class based on SNI and reaction
func ClassifySignal(sni float64, reaction ReactionData) string {
    isPositive := reaction.PriceT1Pct > 0
    
    if sni >= 0.60 {
        if isPositive {
            return "HIGH_POSITIVE"
        }
        return "HIGH_NEGATIVE"
    }
    
    if sni >= 0.30 {
        if isPositive {
            return "MODERATE_POSITIVE"
        }
        return "MODERATE_NEGATIVE"
    }
    
    return "NOISE"
}
```

---

## PR-Heavy Issuer Detection

### Criteria

A company is flagged as "PR-heavy" if:
1. Announcement frequency > 6 per month average
2. Median SNI < 0.25
3. < 20% of announcements are high-signal

### Implementation

```go
// DetectPRHeavyIssuer checks if a company is a PR-heavy issuer
func DetectPRHeavyIssuer(announcements []Announcement, daysBack int) bool {
    if len(announcements) == 0 {
        return false
    }
    
    months := float64(daysBack) / 30.0
    if months < 1 {
        months = 1
    }
    
    // Check frequency
    frequencyPerMonth := float64(len(announcements)) / months
    if frequencyPerMonth > 6 {
        // High frequency, check quality
        
        // Compute median SNI
        snis := make([]float64, len(announcements))
        highSignalCount := 0
        for i, ann := range announcements {
            snis[i] = ann.SNI
            if ann.SNI >= 0.60 {
                highSignalCount++
            }
        }
        
        medianSNI := median(snis)
        highSignalPct := float64(highSignalCount) / float64(len(announcements))
        
        // PR-heavy if low median SNI and few high-signal
        if medianSNI < 0.25 && highSignalPct < 0.20 {
            return true
        }
    }
    
    return false
}

func median(values []float64) float64 {
    if len(values) == 0 {
        return 0
    }
    
    sorted := make([]float64, len(values))
    copy(sorted, values)
    sort.Float64s(sorted)
    
    mid := len(sorted) / 2
    if len(sorted)%2 == 0 {
        return (sorted[mid-1] + sorted[mid]) / 2
    }
    return sorted[mid]
}
```

---

## Summary Generation (AI-Assisted)

For high-signal announcements, generate a 2-line summary using Claude:

```go
// GenerateSummary creates an AI summary of an announcement
func GenerateSummary(raw RawAnnouncement, client *anthropic.Client) (string, error) {
    prompt := fmt.Sprintf(`Summarize this ASX announcement in exactly 2 lines:
- Line 1: What happened (key facts, numbers)
- Line 2: Impact or significance

Headline: %s
Body: %s

Be specific. Include dollar amounts, dates, and counterparties if mentioned.
Do not use promotional language or adjectives.`, raw.Headline, truncate(raw.Body, 2000))

    resp, err := client.Messages.Create(context.Background(), anthropic.MessageCreateParams{
        Model:     "claude-sonnet-4-20250514",
        MaxTokens: 200,
        Messages: []anthropic.Message{
            {Role: "user", Content: prompt},
        },
    })
    
    if err != nil {
        return "", err
    }
    
    return extractText(resp.Content), nil
}
```

---

## Next Document
Proceed to `07-ai-assessment-prompts.md` for the AI assessment prompt templates.
