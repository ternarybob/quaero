package signals

// QualityComputer assesses business quality
type QualityComputer struct{}

// NewQualityComputer creates a new quality computer
func NewQualityComputer() *QualityComputer {
	return &QualityComputer{}
}

// Compute calculates quality signals
func (c *QualityComputer) Compute(raw TickerRaw) QualitySignal {
	if !raw.HasFundamentals {
		return QualitySignal{
			Overall:          "unknown",
			CashConversion:   "unknown",
			BalanceSheetRisk: "unknown",
			MarginTrend:      "unknown",
		}
	}

	// Assess cash conversion
	cashConversion := c.assessCashConversion(raw.Fundamentals)

	// Assess balance sheet risk
	balanceSheetRisk := c.assessBalanceSheetRisk(raw.Fundamentals)

	// Assess margin trend
	marginTrend := c.assessMarginTrend(raw.Fundamentals)

	// Overall quality is the combination
	overall := c.assessOverall(cashConversion, balanceSheetRisk, marginTrend)

	return QualitySignal{
		Overall:          overall,
		CashConversion:   cashConversion,
		BalanceSheetRisk: balanceSheetRisk,
		MarginTrend:      marginTrend,
	}
}

// assessCashConversion evaluates cash conversion quality
func (c *QualityComputer) assessCashConversion(f FundamentalsData) string {
	ocfToEBITDA := f.OCFToEBITDA

	// OCF/EBITDA thresholds
	switch {
	case ocfToEBITDA >= 0.85:
		return "good"
	case ocfToEBITDA >= 0.70:
		return "fair"
	case ocfToEBITDA > 0:
		return "poor"
	default:
		return "unknown" // No data
	}
}

// assessBalanceSheetRisk evaluates balance sheet risk
func (c *QualityComputer) assessBalanceSheetRisk(f FundamentalsData) string {
	netDebtToEBITDA := f.NetDebtToEBITDA
	currentRatio := f.CurrentRatio

	// Combined assessment
	// Net Debt/EBITDA < 1.5 is low risk, 1.5-3 is medium, >3 is high
	// Current ratio > 1.5 is safe, 1-1.5 is adequate, <1 is concerning

	if netDebtToEBITDA < 0 {
		// Net cash position is very safe
		return "low"
	}

	switch {
	case netDebtToEBITDA <= 1.5:
		if currentRatio >= 1.2 || currentRatio == 0 {
			return "low"
		}
		return "medium"
	case netDebtToEBITDA <= 3.0:
		if currentRatio >= 1.5 {
			return "medium"
		}
		return "high"
	default:
		return "high"
	}
}

// assessMarginTrend evaluates margin trajectory
func (c *QualityComputer) assessMarginTrend(f FundamentalsData) string {
	marginDelta := f.EBITDAMarginDeltaYoY

	// Margin change thresholds
	switch {
	case marginDelta > 2.0:
		return "improving"
	case marginDelta >= -1.0:
		return "stable"
	default:
		return "declining"
	}
}

// assessOverall combines individual assessments into overall quality
func (c *QualityComputer) assessOverall(cashConversion, balanceSheetRisk, marginTrend string) string {
	// Score each dimension
	score := 0

	switch cashConversion {
	case "good":
		score += 2
	case "fair":
		score += 1
	}

	switch balanceSheetRisk {
	case "low":
		score += 2
	case "medium":
		score += 1
	}

	switch marginTrend {
	case "improving":
		score += 2
	case "stable":
		score += 1
	}

	// Overall rating based on total score (max 6)
	switch {
	case score >= 5:
		return "good"
	case score >= 3:
		return "fair"
	default:
		return "poor"
	}
}

// Quality rating constants
const (
	QualityGood    = "good"
	QualityFair    = "fair"
	QualityPoor    = "poor"
	QualityUnknown = "unknown"
)

// Balance sheet risk constants
const (
	RiskLow    = "low"
	RiskMedium = "medium"
	RiskHigh   = "high"
)

// Margin trend constants
const (
	MarginImproving = "improving"
	MarginStable    = "stable"
	MarginDeclining = "declining"
)
