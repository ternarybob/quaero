package signals

// TickerAssessment contains the AI-generated assessment for a ticker
type TickerAssessment struct {
	Ticker      string `json:"ticker"`
	HoldingType string `json:"holding_type"` // smsf, trader

	Decision  AssessmentDecision  `json:"decision"`
	Reasoning AssessmentReasoning `json:"reasoning"`
	EntryExit EntryExitParams     `json:"entry_exit"`

	RiskFlags    []string `json:"risk_flags"`
	ThesisStatus string   `json:"thesis_status"` // intact, weakening, strengthening, broken

	JustifiedGain JustifiedGainAssessment `json:"justified_gain_assessment"`

	// Validation
	ValidationPassed bool     `json:"validation_passed"`
	ValidationErrors []string `json:"validation_errors,omitempty"`
}

// AssessmentDecision contains the action decision
type AssessmentDecision struct {
	Action     string `json:"action"`     // accumulate, hold, reduce, exit, buy, add, trim, watch, insufficient_data
	Confidence string `json:"confidence"` // high, medium, low
	Urgency    string `json:"urgency"`    // immediate, this_week, monitor
}

// AssessmentReasoning contains the reasoning with evidence
type AssessmentReasoning struct {
	Primary  string   `json:"primary"`  // 1-2 sentence main rationale
	Evidence []string `json:"evidence"` // 3+ specific data points
}

// EntryExitParams contains stop loss, targets, and invalidation
type EntryExitParams struct {
	// For entries
	Setup     string `json:"setup,omitempty"`
	EntryZone string `json:"entry_zone,omitempty"`

	// For all
	StopLoss     string  `json:"stop_loss"`
	StopLossPct  float64 `json:"stop_loss_pct"`
	Target1      string  `json:"target_1,omitempty"`
	Invalidation string  `json:"invalidation"` // Thesis breaker
}

// JustifiedGainAssessment contains justified gain analysis
type JustifiedGainAssessment struct {
	Justified12MPct float64 `json:"justified_12m_pct"`
	CurrentGainPct  float64 `json:"current_gain_pct"`
	Verdict         string  `json:"verdict"` // aligned, ahead, behind
}

// Action type constants
const (
	ActionAccumulate       = "accumulate"
	ActionHold             = "hold"
	ActionReduce           = "reduce"
	ActionExit             = "exit"
	ActionBuy              = "buy"
	ActionAdd              = "add"
	ActionTrim             = "trim"
	ActionWatch            = "watch"
	ActionInsufficientData = "insufficient_data"
)

// Confidence level constants
const (
	ConfidenceHigh   = "high"
	ConfidenceMedium = "medium"
	ConfidenceLow    = "low"
)

// Urgency level constants
const (
	UrgencyImmediate = "immediate"
	UrgencyThisWeek  = "this_week"
	UrgencyMonitor   = "monitor"
)

// Thesis status constants
const (
	ThesisIntact        = "intact"
	ThesisWeakening     = "weakening"
	ThesisStrengthening = "strengthening"
	ThesisBroken        = "broken"
)

// IsActionable returns true if the assessment requires action
func (ta *TickerAssessment) IsActionable() bool {
	switch ta.Decision.Action {
	case ActionAccumulate, ActionReduce, ActionExit, ActionBuy, ActionAdd, ActionTrim:
		return true
	default:
		return false
	}
}

// IsUrgent returns true if the assessment requires immediate attention
func (ta *TickerAssessment) IsUrgent() bool {
	return ta.Decision.Urgency == UrgencyImmediate || ta.Decision.Urgency == UrgencyThisWeek
}
