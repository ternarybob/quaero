package signals

// VLIConfig holds configuration for VLI calculation
type VLIConfig struct {
	VolumeZScoreThreshold     float64 `json:"vol_zscore_threshold"`
	VolumeZScoreHighThreshold float64 `json:"vol_zscore_high_threshold"`
	PriceFlatATRMultiple      float64 `json:"price_flat_atr_multiple"`
	VWAPThreshold             float64 `json:"vwap_threshold"`
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
	atrPct := raw.Volatility.ATRPctOfPrice
	priceChange1D := raw.Price.Change1DPct

	// Calculate price vs VWAP (handle zero VWAP)
	priceVsVWAP := 1.0
	if raw.Price.VWAP20 > 0 {
		priceVsVWAP = raw.Price.Current / raw.Price.VWAP20
	}

	// Estimate price flatness (if 1-day change is small relative to ATR)
	priceFlat := false
	if atrPct > 0 {
		priceFlat = abs(priceChange1D) < atrPct*c.config.PriceFlatATRMultiple*100
	}

	// Accumulation scoring
	accScore := 0.0

	// High volume above threshold
	if volZScore > c.config.VolumeZScoreThreshold {
		accScore += 0.30
	}

	// Volume trend rising
	if volTrend == "rising" {
		accScore += 0.20
	}

	// Price flat (consolidation with volume = accumulation)
	if priceFlat {
		accScore += 0.20
	}

	// Price above VWAP (strong demand)
	if priceVsVWAP > 1.0 {
		accScore += 0.15
	}

	// Very high volume
	if volZScore > c.config.VolumeZScoreHighThreshold {
		accScore += 0.15
	}

	// Distribution scoring
	distScore := 0.0

	// High volume with price below VWAP = selling pressure
	if volZScore > c.config.VolumeZScoreThreshold && priceVsVWAP < c.config.VWAPThreshold {
		distScore += 0.40
	}

	// Rising volume with falling price = distribution
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
		Score:       round(vli, 2),
		Label:       label,
		VolZScore:   round(volZScore, 2),
		PriceVsVWAP: round(priceVsVWAP, 3),
	}
}
