# ASX Portfolio Intelligence System — Computation Algorithms

## Document Purpose
This document provides the mathematical specifications and implementations for all computed signals. These algorithms form the technical foundation of the system.

---

## Design Principles

1. **Deterministic**: Same inputs always produce same outputs
2. **Documented**: Every formula explained with rationale
3. **Validated**: Each computation has test vectors
4. **Bounded**: All outputs have defined ranges

---

## 1. Price-Business Alignment Score (PBAS)

### Purpose
Quantify whether a stock's price movement is justified by business performance.

### Output Range
`0.0 - 1.0` where:
- `< 0.35`: Overpriced (price ahead of fundamentals)
- `0.35 - 0.65`: Fairly valued
- `> 0.65`: Underpriced (business ahead of price)

### Formula

```
PBAS = sigmoid(k × (BM - PM))

Where:
  BM = Business Momentum (composite score)
  PM = Price Momentum (12-month return)
  k  = Sensitivity factor (default: 5.0)
  
Business Momentum:
  BM = w₁×RevGrowth + w₂×OCFGrowth + w₃×MarginDelta + w₄×ROICDelta - w₅×Dilution

Default weights:
  w₁ = 0.35 (Revenue growth)
  w₂ = 0.25 (Operating cash flow growth)
  w₃ = 0.20 (Margin improvement)
  w₄ = 0.10 (ROIC improvement)
  w₅ = 0.10 (Dilution penalty)
```

### Implementation

```go
package signals

import (
    "math"
)

// PBASConfig holds configuration for PBAS calculation
type PBASConfig struct {
    // Weights for business momentum components
    WeightRevenue   float64 `json:"weight_revenue"`
    WeightOCF       float64 `json:"weight_ocf"`
    WeightMargin    float64 `json:"weight_margin"`
    WeightROIC      float64 `json:"weight_roic"`
    WeightDilution  float64 `json:"weight_dilution"`
    
    // Sensitivity factor for sigmoid
    SensitivityK    float64 `json:"sensitivity_k"`
    
    // Lookback period
    LookbackMonths  int     `json:"lookback_months"`
}

// DefaultPBASConfig returns the default configuration
func DefaultPBASConfig() PBASConfig {
    return PBASConfig{
        WeightRevenue:  0.35,
        WeightOCF:      0.25,
        WeightMargin:   0.20,
        WeightROIC:     0.10,
        WeightDilution: 0.10,
        SensitivityK:   5.0,
        LookbackMonths: 12,
    }
}

// PBASComputer computes Price-Business Alignment Score
type PBASComputer struct {
    config PBASConfig
}

// NewPBASComputer creates a new PBAS computer
func NewPBASComputer(config PBASConfig) *PBASComputer {
    return &PBASComputer{config: config}
}

// Compute calculates PBAS for a ticker
func (c *PBASComputer) Compute(raw TickerRaw) PBASSignal {
    // Extract components
    revGrowth := c.normalizeRevGrowth(raw.Fundamentals.RevenueYoYPct)
    ocfGrowth := c.estimateOCFGrowth(raw)
    marginDelta := c.normalizeMarginDelta(raw.Fundamentals.EBITDAMarginDeltaYoY)
    roicDelta := c.normalizeROICDelta(raw) // May be zero if not available
    dilution := c.normalizeDilution(raw.Fundamentals.Dilution12MPct)
    
    // Compute Business Momentum
    bm := c.config.WeightRevenue*revGrowth +
          c.config.WeightOCF*ocfGrowth +
          c.config.WeightMargin*marginDelta +
          c.config.WeightROIC*roicDelta -
          c.config.WeightDilution*dilution
    
    // Price Momentum (as decimal)
    pm := raw.Price.Return52WPct / 100.0
    
    // Compute divergence
    divergence := bm - pm
    
    // Apply sigmoid transformation
    pbas := sigmoid(c.config.SensitivityK * divergence)
    
    // Determine interpretation
    interpretation := "neutral"
    if pbas > 0.65 {
        interpretation = "underpriced"
    } else if pbas < 0.35 {
        interpretation = "overpriced"
    }
    
    return PBASSignal{
        Score:            round(pbas, 2),
        BusinessMomentum: round(bm, 3),
        PriceMomentum:    round(pm, 3),
        Divergence:       round(divergence, 3),
        Interpretation:   interpretation,
    }
}

// normalizeRevGrowth converts revenue growth to normalized score
// Revenue growth of 15%+ is excellent, 0-15% is moderate, negative is poor
func (c *PBASComputer) normalizeRevGrowth(revGrowthPct float64) float64 {
    // Convert to decimal and cap at reasonable bounds
    growth := revGrowthPct / 100.0
    
    // Normalize: -20% to +30% maps to -0.3 to +0.3
    normalized := growth / 100.0 * 3.0
    return clamp(normalized, -0.3, 0.3)
}

// estimateOCFGrowth estimates OCF growth from available data
func (c *PBASComputer) estimateOCFGrowth(raw TickerRaw) float64 {
    // If OCF/EBITDA ratio is good and improving, assume positive OCF growth
    ocfToEBITDA := raw.Fundamentals.OCFToEBITDA
    
    if ocfToEBITDA >= 0.9 {
        return 0.2 // Strong cash conversion suggests healthy OCF growth
    } else if ocfToEBITDA >= 0.7 {
        return 0.1
    } else if ocfToEBITDA >= 0.5 {
        return 0.0
    } else {
        return -0.1 // Poor cash conversion
    }
}

// normalizeMarginDelta converts margin change to normalized score
func (c *PBASComputer) normalizeMarginDelta(deltaYoY float64) float64 {
    // Margin delta of 2%+ is excellent, -2% or worse is concerning
    // Scale: -5% to +5% maps to -0.2 to +0.2
    normalized := deltaYoY / 25.0
    return clamp(normalized, -0.2, 0.2)
}

// normalizeROICDelta converts ROIC change to normalized score
func (c *PBASComputer) normalizeROICDelta(raw TickerRaw) float64 {
    // If ROIC not available, return neutral
    if raw.Fundamentals.ROICPct == 0 {
        return 0.0
    }
    
    // High ROIC is good, assume stable
    roic := raw.Fundamentals.ROICPct
    if roic > 20 {
        return 0.15
    } else if roic > 15 {
        return 0.10
    } else if roic > 10 {
        return 0.05
    } else {
        return 0.0
    }
}

// normalizeDilution converts dilution to penalty score
func (c *PBASComputer) normalizeDilution(dilutionPct float64) float64 {
    // Dilution is always a negative signal
    // 0-5% is acceptable, 5-10% is concerning, >10% is bad
    if dilutionPct <= 2 {
        return 0.0
    } else if dilutionPct <= 5 {
        return 0.05
    } else if dilutionPct <= 10 {
        return 0.15
    } else {
        return 0.25
    }
}

// sigmoid applies the logistic function
func sigmoid(x float64) float64 {
    return 1.0 / (1.0 + math.Exp(-x))
}

// clamp restricts a value to a range
func clamp(value, min, max float64) float64 {
    if value < min {
        return min
    }
    if value > max {
        return max
    }
    return value
}

// round rounds to specified decimal places
func round(value float64, places int) float64 {
    mult := math.Pow(10, float64(places))
    return math.Round(value*mult) / mult
}
```

### Test Vectors

```go
func TestPBASCompute(t *testing.T) {
    computer := NewPBASComputer(DefaultPBASConfig())
    
    tests := []struct {
        name     string
        raw      TickerRaw
        wantMin  float64
        wantMax  float64
        wantInterp string
    }{
        {
            name: "Strong business, moderate price",
            raw: TickerRaw{
                Price: PriceData{Return52WPct: 15.0},
                Fundamentals: FundamentalsData{
                    RevenueYoYPct:        20.0,
                    OCFToEBITDA:          0.85,
                    EBITDAMarginDeltaYoY: 2.0,
                    ROICPct:              18.0,
                    Dilution12MPct:       1.0,
                },
            },
            wantMin: 0.60,
            wantMax: 0.80,
            wantInterp: "underpriced",
        },
        {
            name: "Weak business, strong price",
            raw: TickerRaw{
                Price: PriceData{Return52WPct: 60.0},
                Fundamentals: FundamentalsData{
                    RevenueYoYPct:        5.0,
                    OCFToEBITDA:          0.55,
                    EBITDAMarginDeltaYoY: -1.0,
                    ROICPct:              8.0,
                    Dilution12MPct:       8.0,
                },
            },
            wantMin: 0.15,
            wantMax: 0.35,
            wantInterp: "overpriced",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := computer.Compute(tt.raw)
            
            if result.Score < tt.wantMin || result.Score > tt.wantMax {
                t.Errorf("PBAS = %v, want between %v and %v", 
                    result.Score, tt.wantMin, tt.wantMax)
            }
            if result.Interpretation != tt.wantInterp {
                t.Errorf("Interpretation = %v, want %v",
                    result.Interpretation, tt.wantInterp)
            }
        })
    }
}
```

---

## 2. Volume Lead Indicator (VLI)

### Purpose
Detect pre-price institutional activity (accumulation or distribution).

### Output Range
`-1.0 to 1.0` where:
- `> 0.5`: Accumulation (institutional buying)
- `-0.3 to 0.3`: Neutral
- `< -0.3`: Distribution (institutional selling)

### Formula

```
VLI = AccumulationScore - DistributionScore

Accumulation signals:
  +0.30 if vol_zscore > 0.5
  +0.20 if volume trend is rising
  +0.20 if price flat (within 0.5×ATR over 5 days)
  +0.15 if price > VWAP20
  +0.15 if vol_zscore > 1.5

Distribution signals:
  +0.40 if vol_zscore > 0.5 AND price < 0.98×VWAP
  +0.30 if volume rising AND price down >1%
```

### Implementation

```go
package signals

// VLIConfig holds configuration for VLI calculation
type VLIConfig struct {
    VolumeZScoreThreshold   float64 `json:"vol_zscore_threshold"`
    VolumeZScoreHighThreshold float64 `json:"vol_zscore_high_threshold"`
    PriceFlatATRMultiple    float64 `json:"price_flat_atr_multiple"`
    VWAPThreshold           float64 `json:"vwap_threshold"`
}

// DefaultVLIConfig returns default VLI configuration
func DefaultVLIConfig() VLIConfig {
    return VLIConfig{
        VolumeZScoreThreshold:     0.5,
        VolumeZScoreHighThreshold: 1.5,
        PriceFlatATRMultiple:      0.5,
        VWAPThreshold:             0.98,
    }
}

// VLIComputer computes Volume Lead Indicator
type VLIComputer struct {
    config VLIConfig
}

// NewVLIComputer creates a new VLI computer
func NewVLIComputer(config VLIConfig) *VLIComputer {
    return &VLIComputer{config: config}
}

// Compute calculates VLI for a ticker
func (c *VLIComputer) Compute(raw TickerRaw) VLISignal {
    volZScore := raw.Volume.ZScore20
    volTrend := raw.Volume.Trend5Dvs20D
    priceVsVWAP := raw.Price.Current / raw.Price.VWAP20
    atrPct := raw.Volatility.ATRPctOfPrice
    priceChange1D := raw.Price.Change1DPct
    
    // Estimate price flatness (would need 5-day range in real impl)
    // Approximation: if 1-day change is small and ATR is moderate
    priceFlat := math.Abs(priceChange1D) < atrPct*c.config.PriceFlatATRMultiple
    
    // Accumulation scoring
    accScore := 0.0
    
    if volZScore > c.config.VolumeZScoreThreshold {
        accScore += 0.30
    }
    
    if volTrend == "rising" {
        accScore += 0.20
    }
    
    if priceFlat {
        accScore += 0.20
    }
    
    if priceVsVWAP > 1.0 {
        accScore += 0.15
    }
    
    if volZScore > c.config.VolumeZScoreHighThreshold {
        accScore += 0.15
    }
    
    // Distribution scoring
    distScore := 0.0
    
    if volZScore > c.config.VolumeZScoreThreshold && priceVsVWAP < c.config.VWAPThreshold {
        distScore += 0.40
    }
    
    if volTrend == "rising" && priceChange1D < -1.0 {
        distScore += 0.30
    }
    
    // Net VLI
    vli := accScore - distScore
    
    // Determine label
    label := "neutral"
    if accScore > 0.5 && accScore > distScore {
        label = "accumulating"
    } else if distScore > 0.3 {
        label = "distributing"
    }
    
    // Clamp to valid range
    vli = clamp(vli, -1.0, 1.0)
    
    return VLISignal{
        Score:       round(math.Abs(vli), 2),
        Label:       label,
        VolZScore:   round(volZScore, 2),
        PriceVsVWAP: round(priceVsVWAP, 3),
    }
}
```

---

## 3. Regime Classifier

### Purpose
Classify price action into actionable regimes.

### Regime Types

| Regime | Description | Entry Signal | Exit Signal |
|--------|-------------|--------------|-------------|
| `breakout` | New highs on volume | Buy | - |
| `trend_up` | Higher highs, bullish EMAs | Add on pullback | - |
| `trend_down` | Lower lows, bearish EMAs | Avoid | Reduce |
| `accumulation` | Flat price, rising volume | Watch for breakout | - |
| `distribution` | Flat highs, weak volume | Trim | Exit |
| `range` | Mean-reverting | Range trade | - |
| `decay` | Lower lows, failed rallies | Exit | Exit |

### Implementation

```go
package signals

// RegimeClassifier classifies price action regimes
type RegimeClassifier struct{}

// NewRegimeClassifier creates a new regime classifier
func NewRegimeClassifier() *RegimeClassifier {
    return &RegimeClassifier{}
}

// Classify determines the current regime for a ticker
func (c *RegimeClassifier) Classify(raw TickerRaw) RegimeSignal {
    price := raw.Price
    vol := raw.Volume
    
    // EMA positions
    aboveEMA20 := price.Current > price.EMA20
    aboveEMA50 := price.Current > price.EMA50
    aboveEMA200 := price.Current > price.EMA200
    
    // EMA stack (bullish = 20 > 50 > 200)
    emaStackBullish := price.EMA20 > price.EMA50 && price.EMA50 > price.EMA200
    emaStackBearish := price.EMA20 < price.EMA50 && price.EMA50 < price.EMA200
    
    // Near 52-week levels
    distToHigh := (price.High52W - price.Current) / price.High52W
    distToLow := (price.Current - price.Low52W) / price.Low52W
    near52WkHigh := distToHigh < 0.05 // Within 5%
    near52WkLow := distToLow < 0.10   // Within 10%
    
    // Volume confirmation
    volExpanding := vol.ZScore20 > 0.5
    volContracting := vol.ZScore20 < -0.5
    volRising := vol.Trend5Dvs20D == "rising"
    
    // Returns
    return4W := raw.Price.Return4WPct
    return12W := raw.Price.Return12WPct
    
    // Classification logic
    var regime RegimeType
    var confidence float64
    
    // Breakout: Near highs with volume
    if near52WkHigh && volExpanding && emaStackBullish {
        regime = RegimeBreakout
        confidence = 0.80
    // Trend Up: Bullish structure, above key EMAs
    } else if emaStackBullish && aboveEMA20 && return4W > 0 {
        regime = RegimeTrendUp
        confidence = 0.70
    // Trend Down: Bearish structure
    } else if emaStackBearish && !aboveEMA50 && return4W < 0 {
        regime = RegimeTrendDown
        confidence = 0.70
    // Decay: Below EMAs, volume on down moves
    } else if !aboveEMA200 && volRising && return12W < -10 {
        regime = RegimeDecay
        confidence = 0.65
    // Distribution: Flat highs, weakening volume
    } else if near52WkHigh && !volExpanding && return4W < 3 {
        regime = RegimeDistribution
        confidence = 0.60
    // Accumulation: Flat price, rising volume
    } else if math.Abs(return4W) < 5 && volRising && volExpanding {
        regime = RegimeAccumulation
        confidence = 0.55
    // Range: Mean-reverting, no clear trend
    } else if math.Abs(return12W) < 10 && !emaStackBullish && !emaStackBearish {
        regime = RegimeRange
        confidence = 0.50
    } else {
        regime = RegimeUndefined
        confidence = 0.30
    }
    
    // Trend bias
    trendBias := "neutral"
    if aboveEMA200 {
        trendBias = "bullish"
    } else {
        trendBias = "bearish"
    }
    
    // EMA stack description
    emaStack := "mixed"
    if emaStackBullish {
        emaStack = "bullish"
    } else if emaStackBearish {
        emaStack = "bearish"
    }
    
    return RegimeSignal{
        Classification: string(regime),
        Confidence:     round(confidence, 2),
        TrendBias:      trendBias,
        EMAStack:       emaStack,
    }
}
```

---

## 4. Cooked Stock Detector

### Purpose
Identify stocks that have decoupled from fundamentals.

### Triggers (Flag if >= 2)

1. PBAS < 0.30
2. Price CAGR > 2.5× Revenue CAGR (and price up >30%)
3. Dilution > 10% in 12 months
4. Poor cash conversion (OCF/EBITDA < 0.6)
5. Price > 30% above 200 EMA

### Implementation

```go
package signals

// CookedDetector detects overvalued/decoupled stocks
type CookedDetector struct {
    threshold int // Number of triggers to flag as cooked (default: 2)
}

// NewCookedDetector creates a new cooked detector
func NewCookedDetector() *CookedDetector {
    return &CookedDetector{threshold: 2}
}

// Detect checks if a stock is "cooked"
func (d *CookedDetector) Detect(raw TickerRaw, pbas PBASSignal) CookedSignal {
    triggers := 0
    reasons := make([]string, 0)
    
    // Trigger 1: PBAS too low
    if pbas.Score < 0.30 {
        triggers++
        reasons = append(reasons, "pbas_below_0.30")
    }
    
    // Trigger 2: Price-Revenue divergence
    priceReturn := raw.Price.Return52WPct
    revGrowth := raw.Fundamentals.RevenueYoYPct
    if priceReturn > 30 && revGrowth > 0 && priceReturn > (revGrowth*2.5) {
        triggers++
        reasons = append(reasons, "price_revenue_divergence")
    }
    // Also trigger if revenue is negative but price is up
    if priceReturn > 20 && revGrowth < 0 {
        triggers++
        reasons = append(reasons, "price_up_revenue_down")
    }
    
    // Trigger 3: High dilution
    if raw.Fundamentals.Dilution12MPct > 10 {
        triggers++
        reasons = append(reasons, "dilution_above_10pct")
    }
    
    // Trigger 4: Poor cash conversion
    if raw.Fundamentals.OCFToEBITDA < 0.60 && raw.Fundamentals.OCFToEBITDA > 0 {
        triggers++
        reasons = append(reasons, "poor_cash_conversion")
    }
    
    // Trigger 5: Extended above 200 EMA
    if raw.Price.EMA200 > 0 {
        priceVs200 := raw.Price.Current / raw.Price.EMA200
        if priceVs200 > 1.30 {
            triggers++
            reasons = append(reasons, "extended_above_200ema")
        }
    }
    
    isCooked := triggers >= d.threshold
    
    // Only return reasons if actually cooked
    if !isCooked {
        reasons = nil
    }
    
    return CookedSignal{
        IsCooked: isCooked,
        Score:    triggers,
        Reasons:  reasons,
    }
}
```

---

## 5. Relative Strength Calculation

### Purpose
Measure performance relative to benchmark.

### Formula

```
RS = (1 + Stock Return) / (1 + Benchmark Return)

RS > 1.0: Outperforming
RS < 1.0: Underperforming

RS Rank = Percentile rank among universe
```

### Implementation

```go
package signals

// RSComputer computes relative strength
type RSComputer struct {
    benchmarkReturns map[string]float64 // period -> benchmark return
}

// Compute calculates RS for a ticker
func (c *RSComputer) Compute(stockReturns map[string]float64) RSSignal {
    // 3-month RS
    rs3M := (1 + stockReturns["3m"]/100) / (1 + c.benchmarkReturns["3m"]/100)
    
    // 6-month RS
    rs6M := (1 + stockReturns["6m"]/100) / (1 + c.benchmarkReturns["6m"]/100)
    
    // RS rank would be computed across universe
    // Placeholder: estimate from RS value
    rsRank := c.estimateRank(rs3M)
    
    return RSSignal{
        VsXJO3M:          round(rs3M, 2),
        VsXJO6M:          round(rs6M, 2),
        RSRankPercentile: rsRank,
    }
}

func (c *RSComputer) estimateRank(rs float64) int {
    // Approximate percentile from RS value
    // RS of 1.0 = 50th percentile
    // RS of 1.2 = ~75th percentile
    // RS of 0.8 = ~25th percentile
    if rs >= 1.3 {
        return 90
    } else if rs >= 1.2 {
        return 80
    } else if rs >= 1.1 {
        return 65
    } else if rs >= 1.0 {
        return 50
    } else if rs >= 0.9 {
        return 35
    } else if rs >= 0.8 {
        return 20
    } else {
        return 10
    }
}
```

---

## 6. Justified Return Calculation

### Purpose
Estimate what return is justified by business fundamentals.

### Formula

```
Justified Return = Business Momentum × Sector Multiple + Dividend Yield

Where Business Momentum maps to return expectations:
  BM > 0.20  → Expected 20-30% return
  BM 0.10-0.20 → Expected 10-20% return
  BM 0-0.10 → Expected 5-10% return
  BM < 0 → Expected 0-5% return or negative
```

### Implementation

```go
package signals

// JustifiedReturnComputer calculates justified returns
type JustifiedReturnComputer struct{}

// Compute calculates justified return metrics
func (c *JustifiedReturnComputer) Compute(
    raw TickerRaw, 
    pbas PBASSignal,
) JustifiedReturnSignal {
    bm := pbas.BusinessMomentum
    
    // Map business momentum to expected return
    var expectedReturn float64
    if bm > 0.20 {
        expectedReturn = 25.0
    } else if bm > 0.15 {
        expectedReturn = 20.0
    } else if bm > 0.10 {
        expectedReturn = 15.0
    } else if bm > 0.05 {
        expectedReturn = 10.0
    } else if bm > 0 {
        expectedReturn = 5.0
    } else if bm > -0.05 {
        expectedReturn = 0.0
    } else {
        expectedReturn = -5.0
    }
    
    // Actual return
    actualReturn := raw.Price.Return52WPct
    
    // Divergence
    divergence := actualReturn - expectedReturn
    
    // Interpretation
    interpretation := "aligned"
    if divergence > 15 {
        interpretation = "price_ahead"
    } else if divergence > 5 {
        interpretation = "slightly_ahead"
    } else if divergence < -15 {
        interpretation = "price_behind"
    } else if divergence < -5 {
        interpretation = "slightly_behind"
    }
    
    return JustifiedReturnSignal{
        Expected12MPct: round(expectedReturn, 1),
        Actual12MPct:   round(actualReturn, 1),
        DivergencePct:  round(divergence, 1),
        Interpretation: interpretation,
    }
}
```

---

## 7. Technical Indicators

### EMA (Exponential Moving Average)

```go
func ema(prices []float64, period int) float64 {
    if len(prices) < period {
        return 0
    }
    
    multiplier := 2.0 / float64(period+1)
    ema := sma(prices[:period], period) // Start with SMA
    
    for i := period; i < len(prices); i++ {
        ema = (prices[i]-ema)*multiplier + ema
    }
    
    return ema
}
```

### ATR (Average True Range)

```go
func atr(ohlcv []OHLCV, period int) float64 {
    if len(ohlcv) < period+1 {
        return 0
    }
    
    trueRanges := make([]float64, len(ohlcv)-1)
    for i := 1; i < len(ohlcv); i++ {
        high := ohlcv[i].High
        low := ohlcv[i].Low
        prevClose := ohlcv[i-1].Close
        
        tr := max3(
            high-low,
            math.Abs(high-prevClose),
            math.Abs(low-prevClose),
        )
        trueRanges[i-1] = tr
    }
    
    return ema(trueRanges, period)
}
```

### VWAP (Volume-Weighted Average Price)

```go
func vwap(ohlcv []OHLCV, period int) float64 {
    if len(ohlcv) < period {
        return 0
    }
    
    start := len(ohlcv) - period
    
    var sumPV float64
    var sumV float64
    
    for i := start; i < len(ohlcv); i++ {
        typicalPrice := (ohlcv[i].High + ohlcv[i].Low + ohlcv[i].Close) / 3
        sumPV += typicalPrice * float64(ohlcv[i].Volume)
        sumV += float64(ohlcv[i].Volume)
    }
    
    if sumV == 0 {
        return 0
    }
    
    return sumPV / sumV
}
```

### Volume Z-Score

```go
func volumeZScore(volumes []float64, currentVolume float64, period int) float64 {
    if len(volumes) < period {
        return 0
    }
    
    recent := volumes[len(volumes)-period:]
    mean := avg(recent)
    stdDev := stddev(recent)
    
    if stdDev == 0 {
        return 0
    }
    
    return (currentVolume - mean) / stdDev
}
```

---

## 8. Utility Functions

```go
package signals

import "math"

func sma(values []float64, period int) float64 {
    if len(values) < period {
        return 0
    }
    sum := 0.0
    for i := len(values) - period; i < len(values); i++ {
        sum += values[i]
    }
    return sum / float64(period)
}

func avg(values []float64) float64 {
    if len(values) == 0 {
        return 0
    }
    sum := 0.0
    for _, v := range values {
        sum += v
    }
    return sum / float64(len(values))
}

func stddev(values []float64) float64 {
    if len(values) < 2 {
        return 0
    }
    mean := avg(values)
    sumSquares := 0.0
    for _, v := range values {
        diff := v - mean
        sumSquares += diff * diff
    }
    return math.Sqrt(sumSquares / float64(len(values)-1))
}

func zscore(value float64, values []float64) float64 {
    mean := avg(values)
    sd := stddev(values)
    if sd == 0 {
        return 0
    }
    return (value - mean) / sd
}

func pctChange(old, new float64) float64 {
    if old == 0 {
        return 0
    }
    return ((new - old) / old) * 100
}

func returnPct(prices []float64, days int) float64 {
    n := len(prices)
    if n < days+1 {
        return 0
    }
    return pctChange(prices[n-days-1], prices[n-1])
}

func max3(a, b, c float64) float64 {
    return math.Max(a, math.Max(b, c))
}

func maxSlice(values []float64) float64 {
    if len(values) == 0 {
        return 0
    }
    max := values[0]
    for _, v := range values[1:] {
        if v > max {
            max = v
        }
    }
    return max
}

func minSlice(values []float64) float64 {
    if len(values) == 0 {
        return 0
    }
    min := values[0]
    for _, v := range values[1:] {
        if v < min {
            min = v
        }
    }
    return min
}
```

---

## Next Document
Proceed to `05-strategy-schema.md` for the complete strategy configuration schema.
