package signals

import (
	"regexp"
	"strings"
)

// AssessmentValidator validates AI-generated assessments
type AssessmentValidator struct {
	genericPhrases []string
	numberRegex    *regexp.Regexp
}

// NewAssessmentValidator creates a new assessment validator
func NewAssessmentValidator() *AssessmentValidator {
	return &AssessmentValidator{
		genericPhrases: []string{
			"solid fundamentals",
			"well-positioned",
			"strong outlook",
			"good management",
			"attractive valuation",
			"quality company",
			"solid growth",
			"strong growth",
			"excellent growth",
			"good growth",
			"solid performance",
			"strong performance",
		},
		numberRegex: regexp.MustCompile(`\d+\.?\d*`),
	}
}

// Validate checks if an assessment meets quality requirements
func (v *AssessmentValidator) Validate(assessment TickerAssessment, sig TickerSignals) ValidationResult {
	result := ValidationResult{Valid: true}

	// Check evidence count - require at least 3 points
	if len(assessment.Reasoning.Evidence) < 3 {
		result.Valid = false
		result.Errors = append(result.Errors, "requires at least 3 evidence points")
	}

	// Check evidence quality
	for _, evidence := range assessment.Reasoning.Evidence {
		// Must contain a number
		if !v.containsNumber(evidence) {
			result.Valid = false
			result.Errors = append(result.Errors, "evidence lacks quantification: '"+truncateString(evidence, 50)+"'")
		}

		// Must not contain generic phrases
		lowerEvidence := strings.ToLower(evidence)
		for _, phrase := range v.genericPhrases {
			if strings.Contains(lowerEvidence, phrase) {
				result.Valid = false
				result.Errors = append(result.Errors, "generic phrase in evidence: '"+phrase+"'")
			}
		}
	}

	// Action-signal consistency check
	consistency, warning := v.checkActionConsistency(assessment.Decision.Action, sig)
	if !consistency {
		result.Valid = false
		result.Errors = append(result.Errors, "action inconsistent with signals: "+warning)
	}

	// Check stop loss is specified for actionable items
	if assessment.IsActionable() && assessment.EntryExit.StopLoss == "" {
		result.Warnings = append(result.Warnings, "no stop loss specified for actionable recommendation")
	}

	// Check confidence is specified
	if assessment.Decision.Confidence == "" {
		result.Warnings = append(result.Warnings, "confidence level not specified")
	}

	return result
}

// containsNumber checks if a string contains at least one number
func (v *AssessmentValidator) containsNumber(s string) bool {
	return v.numberRegex.MatchString(s)
}

// checkActionConsistency validates that the action aligns with signal data
func (v *AssessmentValidator) checkActionConsistency(action string, sig TickerSignals) (bool, string) {
	switch action {
	case ActionAccumulate:
		// Accumulate requires: PBAS > 0.55, not cooked, not in decay
		if sig.PBAS.Score < 0.55 {
			return false, "PBAS too low for accumulate"
		}
		if sig.Cooked.IsCooked {
			return false, "stock is cooked, cannot accumulate"
		}
		if sig.Regime.Classification == string(RegimeDecay) {
			return false, "stock in decay regime, cannot accumulate"
		}
		return true, ""

	case ActionBuy, ActionAdd:
		// Buy/Add requires: VLI > 0.30, favorable regime
		if sig.VLI.Score < 0.30 {
			return false, "VLI too low for buy/add"
		}
		favorableRegimes := map[string]bool{
			string(RegimeTrendUp):      true,
			string(RegimeAccumulation): true,
			string(RegimeBreakout):     true,
		}
		if !favorableRegimes[sig.Regime.Classification] {
			return false, "regime not favorable for buy/add"
		}
		return true, ""

	case ActionReduce, ActionExit:
		// Reduce/Exit is valid when: PBAS < 0.50 OR cooked OR distributing
		if sig.PBAS.Score < 0.50 || sig.Cooked.IsCooked || sig.VLI.Label == "distributing" {
			return true, ""
		}
		return false, "signals don't support reduce/exit"

	case ActionTrim:
		// Trim is valid when there are any risk flags or PBAS declining
		if len(sig.RiskFlags) > 0 || sig.PBAS.Score < 0.60 {
			return true, ""
		}
		return false, "no risk flags to justify trim"

	case ActionHold, ActionWatch, ActionInsufficientData:
		// These are always valid
		return true, ""

	default:
		return true, ""
	}
}

// truncateString truncates a string to maxLen chars with ellipsis
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
