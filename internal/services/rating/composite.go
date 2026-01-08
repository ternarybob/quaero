package rating

import "fmt"

// Investability formula weights
const (
	WeightBFS = 12.5
	WeightCDS = 12.5
	WeightNFR = 25.0
	WeightPPS = 25.0
	WeightVRS = 15.0
	WeightOB  = 10.0
)

// Label thresholds
const (
	ThresholdLowAlpha       = 25.0
	ThresholdWatchlist      = 50.0
	ThresholdInvestable     = 65.0
	ThresholdHighConviction = 80.0
)

// CalculateRating combines all component scores into final rating.
//
// Gate Logic:
// - Gate passes if BFS >= 1 AND CDS >= 1
// - If gate fails, label = SPECULATIVE and investability is nil
//
// Investability Formula (if gate passes):
// investability = BFS*12.5 + CDS*12.5 + NFR*25 + PPS*25 + VRS*15 + OB*10
// Range: 0-100
//
// Labels (if gate passes):
// - LOW_ALPHA: 25-50
// - WATCHLIST: 50-65
// - INVESTABLE: 65-80
// - HIGH_CONVICTION: 80+
func CalculateRating(bfs BFSResult, cds CDSResult, nfr NFRResult, pps PPSResult, vrs VRSResult, ob OBResult) RatingResult {
	scores := AllScores{
		BFS: bfs,
		CDS: cds,
		NFR: nfr,
		PPS: pps,
		VRS: vrs,
		OB:  ob,
	}

	// Check gate
	gatePassed := bfs.Score >= 1 && cds.Score >= 1

	if !gatePassed {
		reasons := []string{}
		if bfs.Score < 1 {
			reasons = append(reasons, fmt.Sprintf("BFS=%d", bfs.Score))
		}
		if cds.Score < 1 {
			reasons = append(reasons, fmt.Sprintf("CDS=%d", cds.Score))
		}

		return RatingResult{
			Label:         LabelSpeculative,
			Investability: nil,
			GatePassed:    false,
			Scores:        scores,
			Reasoning:     fmt.Sprintf("Gate failed: %v", reasons),
		}
	}

	// Calculate investability score
	investability := calculateInvestability(bfs, cds, nfr, pps, vrs, ob)
	investabilityPtr := &investability

	// Determine label based on investability
	label := determineLabel(investability)

	// Build reasoning
	reasoning := fmt.Sprintf("Investability=%.1f: BFS=%d CDS=%d NFR=%.2f PPS=%.2f VRS=%.2f OB=%.1f",
		investability, bfs.Score, cds.Score, nfr.Score, pps.Score, vrs.Score, ob.Score)

	return RatingResult{
		Label:         label,
		Investability: investabilityPtr,
		GatePassed:    true,
		Scores:        scores,
		Reasoning:     reasoning,
	}
}

// calculateInvestability applies the weighted formula
func calculateInvestability(bfs BFSResult, cds CDSResult, nfr NFRResult, pps PPSResult, vrs VRSResult, ob OBResult) float64 {
	// Gate scores (0, 1, 2) normalized to 0-1 for weighting
	bfsNorm := float64(bfs.Score) / 2.0
	cdsNorm := float64(cds.Score) / 2.0

	// Calculate weighted sum
	score := bfsNorm*WeightBFS +
		cdsNorm*WeightCDS +
		nfr.Score*WeightNFR +
		pps.Score*WeightPPS +
		vrs.Score*WeightVRS +
		ob.Score*WeightOB

	return ClampFloat64(score, 0, 100)
}

// determineLabel assigns rating label based on investability score
func determineLabel(investability float64) RatingLabel {
	if investability >= ThresholdHighConviction {
		return LabelHighConviction
	}
	if investability >= ThresholdInvestable {
		return LabelInvestable
	}
	if investability >= ThresholdWatchlist {
		return LabelWatchlist
	}
	if investability >= ThresholdLowAlpha {
		return LabelLowAlpha
	}
	return LabelSpeculative
}
