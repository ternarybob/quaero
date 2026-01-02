package signals

import (
	"fmt"
	"strings"
	"time"
)

// Signal descriptions - static explanations of what each signal measures
const (
	DescriptionPBAS         = "Measures alignment between business fundamentals and price action. Scores above 0.6 suggest underpricing; below 0.4 suggest overpricing."
	DescriptionVLI          = "Detects institutional accumulation or distribution based on volume patterns and price behavior relative to VWAP."
	DescriptionRegime       = "Classifies the current price action phase (breakout, trend, accumulation, distribution, range, or decay)."
	DescriptionCooked       = "Identifies stocks that may be overvalued or decoupled from fundamentals. Score ≥2 triggers a warning."
	DescriptionRS           = "Measures price performance relative to the ASX 200 benchmark over 3 and 6 month periods."
	DescriptionQuality      = "Assesses business quality based on cash conversion, balance sheet risk, and margin trends."
	DescriptionJustifiedRet = "Compares expected return (based on business momentum) to actual price return to identify mispricing."
	DescriptionRiskFlags    = "Aggregated list of risk indicators detected across all signal computations."
)

// SignalComputer orchestrates all signal computations
type SignalComputer struct {
	pbas      *PBASComputer
	vli       *VLIComputer
	regime    *RegimeClassifier
	cooked    *CookedDetector
	rs        *RSComputer
	quality   *QualityComputer
	justified *JustifiedReturnComputer
}

// NewSignalComputer creates a new SignalComputer with default configurations
func NewSignalComputer() *SignalComputer {
	return &SignalComputer{
		pbas:      NewPBASComputer(DefaultPBASConfig()),
		vli:       NewVLIComputer(DefaultVLIConfig()),
		regime:    NewRegimeClassifier(),
		cooked:    NewCookedDetector(),
		rs:        NewRSComputer(),
		quality:   NewQualityComputer(),
		justified: NewJustifiedReturnComputer(),
	}
}

// NewSignalComputerWithConfig creates a SignalComputer with custom configurations
func NewSignalComputerWithConfig(pbasConfig PBASConfig, vliConfig VLIConfig) *SignalComputer {
	return &SignalComputer{
		pbas:      NewPBASComputer(pbasConfig),
		vli:       NewVLIComputer(vliConfig),
		regime:    NewRegimeClassifier(),
		cooked:    NewCookedDetector(),
		rs:        NewRSComputer(),
		quality:   NewQualityComputer(),
		justified: NewJustifiedReturnComputer(),
	}
}

// SetBenchmarkReturns sets the benchmark returns for RS calculation
func (c *SignalComputer) SetBenchmarkReturns(returns map[string]float64) {
	c.rs.SetBenchmarkReturns(returns)
}

// ComputeSignals computes all signals for a ticker
func (c *SignalComputer) ComputeSignals(raw TickerRaw) TickerSignals {
	// 1. Compute PBAS first (other signals depend on it)
	pbas := c.pbas.Compute(raw)

	// 2. Compute other signals
	vli := c.vli.Compute(raw)
	regime := c.regime.Classify(raw)
	cooked := c.cooked.Detect(raw, pbas)

	// 3. Compute RS from raw data
	rs := c.rs.ComputeFromRaw(raw)

	// 4. Quality and justified return
	quality := c.quality.Compute(raw)
	justified := c.justified.Compute(raw, pbas)

	// 5. Extract price signals
	priceSignals := c.extractPriceSignals(raw)

	// 6. Compile risk flags
	riskFlags := c.compileRiskFlags(raw, pbas, vli, regime, cooked)

	// 7. Set descriptions for all signals
	pbas.Description = DescriptionPBAS
	vli.Description = DescriptionVLI
	regime.Description = DescriptionRegime
	cooked.Description = DescriptionCooked
	rs.Description = DescriptionRS
	quality.Description = DescriptionQuality
	justified.Description = DescriptionJustifiedRet

	// 8. Generate AI comments for each signal
	pbas.Comment = c.generatePBASComment(pbas)
	vli.Comment = c.generateVLIComment(vli)
	regime.Comment = c.generateRegimeComment(regime)
	cooked.Comment = c.generateCookedComment(cooked)
	rs.Comment = c.generateRSComment(rs)
	quality.Comment = c.generateQualityComment(quality)
	justified.Comment = c.generateJustifiedComment(justified)

	return TickerSignals{
		Ticker:               raw.Ticker,
		ComputeTimestamp:     time.Now(),
		Price:                priceSignals,
		PBAS:                 pbas,
		VLI:                  vli,
		Regime:               regime,
		RS:                   rs,
		Cooked:               cooked,
		Quality:              quality,
		JustifiedReturn:      justified,
		RiskFlags:            riskFlags,
		RiskFlagsDescription: DescriptionRiskFlags,
		// Announcements are populated separately if available
		Announcements: AnnouncementSignals{},
	}
}

// extractPriceSignals extracts price-related signals from raw data
func (c *SignalComputer) extractPriceSignals(raw TickerRaw) PriceSignals {
	price := raw.Price

	// Determine vs EMA positions
	vsEMA20 := c.compareToLevel(price.Current, price.EMA20)
	vsEMA50 := c.compareToLevel(price.Current, price.EMA50)
	vsEMA200 := c.compareToLevel(price.Current, price.EMA200)

	// Calculate distance to 52-week levels
	distTo52WHighPct := 0.0
	if price.High52W > 0 {
		distTo52WHighPct = (price.High52W - price.Current) / price.High52W * 100
	}

	distTo52WLowPct := 0.0
	if price.Low52W > 0 {
		distTo52WLowPct = (price.Current - price.Low52W) / price.Low52W * 100
	}

	return PriceSignals{
		Current:              price.Current,
		Change1DPct:          round(price.Change1DPct, 2),
		Return12WPct:         round(price.Return12WPct, 2),
		Return52WPct:         round(price.Return52WPct, 2),
		VsEMA20:              vsEMA20,
		VsEMA50:              vsEMA50,
		VsEMA200:             vsEMA200,
		DistanceTo52WHighPct: round(distTo52WHighPct, 2),
		DistanceTo52WLowPct:  round(distTo52WLowPct, 2),
	}
}

// compareToLevel returns "above", "below", or "at" based on price vs level
func (c *SignalComputer) compareToLevel(price, level float64) string {
	if level == 0 {
		return "unknown"
	}

	pctDiff := (price - level) / level * 100

	if pctDiff > 1 {
		return "above"
	} else if pctDiff < -1 {
		return "below"
	}
	return "at"
}

// compileRiskFlags aggregates risk indicators from all signals
func (c *SignalComputer) compileRiskFlags(raw TickerRaw, pbas PBASSignal, vli VLISignal, regime RegimeSignal, cooked CookedSignal) []string {
	flags := make([]string, 0)

	// Cooked stock flags
	if cooked.IsCooked {
		flags = append(flags, "cooked_stock")
		flags = append(flags, cooked.Reasons...)
	}

	// Distribution regime
	if regime.Classification == string(RegimeDistribution) {
		flags = append(flags, "distribution_regime")
	}

	// Decay regime
	if regime.Classification == string(RegimeDecay) {
		flags = append(flags, "decay_regime")
	}

	// Bearish trend bias below 200 EMA
	if regime.TrendBias == "bearish" && raw.Price.Current < raw.Price.EMA200 {
		flags = append(flags, "below_200ema")
	}

	// Distribution volume pattern
	if vli.Label == "distributing" {
		flags = append(flags, "distribution_volume")
	}

	// Very low PBAS (even if not cooked)
	if pbas.Score < 0.35 {
		flags = append(flags, "low_pbas")
	}

	// High dilution
	if raw.Fundamentals.Dilution12MPct > 10 {
		flags = append(flags, "high_dilution")
	}

	// Poor cash conversion
	if raw.Fundamentals.OCFToEBITDA > 0 && raw.Fundamentals.OCFToEBITDA < 0.60 {
		flags = append(flags, "poor_cash_conversion")
	}

	// High leverage
	if raw.Fundamentals.NetDebtToEBITDA > 3 {
		flags = append(flags, "high_leverage")
	}

	// Near 52-week low
	if raw.Price.Low52W > 0 {
		distToLow := (raw.Price.Current - raw.Price.Low52W) / raw.Price.Low52W
		if distToLow < 0.10 {
			flags = append(flags, "near_52w_low")
		}
	}

	return flags
}

// SetAnnouncementSignals sets announcement signals on a TickerSignals struct
// This is called separately when announcement data is available
func SetAnnouncementSignals(signals *TickerSignals, announcements AnnouncementSignals) {
	signals.Announcements = announcements
}

// generatePBASComment generates a contextual comment for the PBAS signal
func (c *SignalComputer) generatePBASComment(s PBASSignal) string {
	switch {
	case s.Score >= 0.7:
		return "Strong business momentum with lagging price suggests significant underpricing. Price may catch up to fundamentals."
	case s.Score >= 0.55:
		return "Business fundamentals are outpacing price action. Mild underpricing may present opportunity."
	case s.Score <= 0.3:
		return "Price has run ahead of business fundamentals. Elevated risk of mean reversion."
	case s.Score <= 0.45:
		return "Price is slightly ahead of business momentum. Caution warranted."
	default:
		return "Business and price momentum are broadly aligned. No significant mispricing detected."
	}
}

// generateVLIComment generates a contextual comment for the VLI signal
func (c *SignalComputer) generateVLIComment(s VLISignal) string {
	switch s.Label {
	case "accumulating":
		if s.Score > 0.7 {
			return "Strong accumulation pattern detected. Elevated volume on up-moves suggests institutional buying."
		}
		return "Mild accumulation signal. Volume patterns suggest building interest."
	case "distributing":
		if s.Score < -0.7 {
			return "Strong distribution pattern detected. Large holders may be reducing exposure significantly."
		}
		return "Mild distribution signal. Volume patterns suggest some selling pressure."
	default:
		return "Volume patterns are neutral with no clear accumulation or distribution signal."
	}
}

// generateRegimeComment generates a contextual comment for the Regime signal
func (c *SignalComputer) generateRegimeComment(s RegimeSignal) string {
	confidence := ""
	if s.Confidence >= 0.7 {
		confidence = "High confidence: "
	} else if s.Confidence >= 0.5 {
		confidence = "Moderate confidence: "
	} else {
		confidence = "Low confidence: "
	}

	switch s.Classification {
	case string(RegimeBreakout):
		return confidence + "Stock is breaking out of consolidation with strong momentum. Key level breakout in progress."
	case string(RegimeTrendUp):
		return confidence + "Established uptrend with bullish EMA alignment. Trend-following strategies may be appropriate."
	case string(RegimeTrendDown):
		return confidence + "Established downtrend with bearish EMA alignment. Defensive positioning recommended."
	case string(RegimeAccumulation):
		return confidence + "Stock in accumulation phase. Smart money may be building positions ahead of a move."
	case string(RegimeDistribution):
		return confidence + "Price action suggests distribution phase. May precede price weakness."
	case string(RegimeRange):
		return confidence + "Stock trading in a defined range. Watch for breakout or breakdown."
	case string(RegimeDecay):
		return confidence + "Decaying momentum suggests loss of institutional interest. Elevated downside risk."
	default:
		return confidence + "Price action regime is unclear. Mixed signals present."
	}
}

// generateCookedComment generates a contextual comment for the Cooked signal
func (c *SignalComputer) generateCookedComment(s CookedSignal) string {
	if s.IsCooked {
		reasonList := strings.Join(s.Reasons, ", ")
		return fmt.Sprintf("Multiple warning signs triggered (%s). Elevated risk of price correction.", reasonList)
	}
	switch s.Score {
	case 0:
		return "No warning signs detected. Price appears grounded in fundamentals."
	case 1:
		return "One minor warning flag present but not concerning on its own. Monitor for additional signals."
	default:
		return "Some warning flags present but below threshold. Maintain awareness."
	}
}

// generateRSComment generates a contextual comment for the RS signal
func (c *SignalComputer) generateRSComment(s RSSignal) string {
	switch {
	case s.RSRankPercentile >= 80:
		return "Strong outperformance vs ASX 200. Top-tier relative strength indicates institutional favor."
	case s.RSRankPercentile >= 60:
		return "Above-average relative strength. Outperforming the broader market."
	case s.RSRankPercentile <= 20:
		return "Significant underperformance vs benchmark. Relative weakness may indicate structural issues."
	case s.RSRankPercentile <= 40:
		return "Below-average relative strength. Lagging the broader market."
	default:
		return "Performing in line with the broader market. Neither outperforming nor underperforming."
	}
}

// generateQualityComment generates a contextual comment for the Quality signal
func (c *SignalComputer) generateQualityComment(s QualitySignal) string {
	var comments []string

	switch s.Overall {
	case "good":
		comments = append(comments, "Strong business quality across key metrics.")
	case "fair":
		comments = append(comments, "Acceptable business quality with some areas for improvement.")
	case "poor":
		comments = append(comments, "Quality concerns identified.")
	}

	// Add specific observations
	if s.BalanceSheetRisk == "high" {
		comments = append(comments, "High balance sheet risk warrants caution.")
	}
	if s.CashConversion == "poor" {
		comments = append(comments, "Weak cash conversion may indicate earnings quality issues.")
	}
	if s.MarginTrend == "declining" {
		comments = append(comments, "Declining margins suggest competitive or cost pressures.")
	}
	if s.MarginTrend == "improving" {
		comments = append(comments, "Improving margins indicate operational efficiency gains.")
	}

	return strings.Join(comments, " ")
}

// generateJustifiedComment generates a contextual comment for the JustifiedReturn signal
func (c *SignalComputer) generateJustifiedComment(s JustifiedReturnSignal) string {
	switch s.Interpretation {
	case "price_behind":
		return fmt.Sprintf("Price has underperformed what business momentum justifies by %.1f%%. Potential catch-up opportunity.", -s.DivergencePct)
	case "slightly_behind":
		return "Price is slightly behind justified returns. Minor underpricing present."
	case "price_ahead":
		return fmt.Sprintf("Price has outrun justified returns by %.1f%%. May be priced for perfection.", s.DivergencePct)
	case "slightly_ahead":
		return "Price is slightly ahead of justified returns. Modest overpricing present."
	case "aligned":
		return "Price returns are aligned with business fundamentals. Fair valuation indicated."
	default:
		return "Justified return analysis inconclusive due to insufficient data."
	}
}
