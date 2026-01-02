package signals

import (
	"testing"
	"time"
)

func TestSignalComputer_ComputeSignals(t *testing.T) {
	computer := NewSignalComputer()

	raw := TickerRaw{
		Ticker:          "GNP",
		FetchTimestamp:  time.Now(),
		HasFundamentals: true,
		Price: PriceData{
			Current:      3.50,
			Change1DPct:  1.5,
			High52W:      4.00,
			Low52W:       2.50,
			EMA20:        3.40,
			EMA50:        3.30,
			EMA200:       3.00,
			VWAP20:       3.35,
			Return1WPct:  2.0,
			Return4WPct:  5.0,
			Return12WPct: 15.0,
			Return26WPct: 20.0,
			Return52WPct: 30.0,
		},
		Volume: VolumeData{
			Current:      1000000,
			SMA20:        800000,
			ZScore20:     1.5,
			Trend5Dvs20D: "rising",
		},
		Volatility: VolatilityData{
			ATR14:         0.15,
			ATRPctOfPrice: 4.3,
		},
		Fundamentals: FundamentalsData{
			RevenueYoYPct:        20.0,
			OCFToEBITDA:          0.85,
			EBITDAMarginPct:      25.0,
			EBITDAMarginDeltaYoY: 2.0,
			ROICPct:              15.0,
			ROEPct:               18.0,
			Dilution12MPct:       1.0,
			NetDebtToEBITDA:      1.5,
			CurrentRatio:         1.8,
		},
	}

	result := computer.ComputeSignals(raw)

	// Verify basic structure
	if result.Ticker != "GNP" {
		t.Errorf("Ticker = %v, want GNP", result.Ticker)
	}
	if result.ComputeTimestamp.IsZero() {
		t.Error("ComputeTimestamp should not be zero")
	}

	// Verify PBAS computed
	if result.PBAS.Score <= 0 || result.PBAS.Score >= 1 {
		t.Errorf("PBAS.Score = %v, want between 0 and 1", result.PBAS.Score)
	}
	if result.PBAS.Interpretation == "" {
		t.Error("PBAS.Interpretation should not be empty")
	}

	// Verify VLI computed
	if result.VLI.Label == "" {
		t.Error("VLI.Label should not be empty")
	}

	// Verify Regime computed
	if result.Regime.Classification == "" {
		t.Error("Regime.Classification should not be empty")
	}
	if result.Regime.TrendBias == "" {
		t.Error("Regime.TrendBias should not be empty")
	}

	// Verify Cooked computed
	// (This stock should not be cooked based on the data)
	if result.Cooked.Score < 0 {
		t.Error("Cooked.Score should not be negative")
	}

	// Verify RS computed
	if result.RS.RSRankPercentile < 0 || result.RS.RSRankPercentile > 100 {
		t.Errorf("RS.RSRankPercentile = %v, want between 0 and 100", result.RS.RSRankPercentile)
	}

	// Verify Quality computed
	if result.Quality.Overall == "" {
		t.Error("Quality.Overall should not be empty")
	}

	// Verify JustifiedReturn computed
	if result.JustifiedReturn.Interpretation == "" {
		t.Error("JustifiedReturn.Interpretation should not be empty")
	}

	// Verify Price signals extracted
	if result.Price.Current != 3.50 {
		t.Errorf("Price.Current = %v, want 3.50", result.Price.Current)
	}
}

func TestSignalComputer_WithCustomConfig(t *testing.T) {
	pbasConfig := PBASConfig{
		WeightRevenue:  0.40,
		WeightOCF:      0.20,
		WeightMargin:   0.20,
		WeightROIC:     0.10,
		WeightDilution: 0.10,
		SensitivityK:   6.0,
	}

	vliConfig := VLIConfig{
		VolumeZScoreThreshold:     0.4,
		VolumeZScoreHighThreshold: 1.2,
		PriceFlatATRMultiple:      0.4,
		VWAPThreshold:             0.97,
	}

	computer := NewSignalComputerWithConfig(pbasConfig, vliConfig)

	raw := TickerRaw{
		Ticker:          "TEST",
		HasFundamentals: true,
		Price: PriceData{
			Current:      10.0,
			EMA200:       9.0,
			Return52WPct: 20.0,
		},
		Fundamentals: FundamentalsData{
			RevenueYoYPct: 15.0,
		},
	}

	result := computer.ComputeSignals(raw)

	if result.Ticker != "TEST" {
		t.Errorf("Ticker = %v, want TEST", result.Ticker)
	}
	if result.PBAS.Score <= 0 {
		t.Error("PBAS should be computed with custom config")
	}
}

func TestSignalComputer_SetBenchmarkReturns(t *testing.T) {
	computer := NewSignalComputer()

	benchmarks := map[string]float64{
		"3m": 5.0,
		"6m": 10.0,
	}
	computer.SetBenchmarkReturns(benchmarks)

	raw := TickerRaw{
		Ticker: "TEST",
		Price: PriceData{
			Return12WPct: 15.0, // 3M
			Return26WPct: 25.0, // 6M
		},
	}

	result := computer.ComputeSignals(raw)

	// Stock returned 15% vs benchmark 5% = RS should be > 1.0
	if result.RS.VsXJO3M < 1.0 {
		t.Errorf("RS.VsXJO3M = %v, want > 1.0 (stock beat benchmark)", result.RS.VsXJO3M)
	}
}
