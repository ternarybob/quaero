package announcements

import (
	"testing"
	"time"
)

func TestClassifyRelevance(t *testing.T) {
	tests := []struct {
		name           string
		headline       string
		annType        string
		priceSensitive bool
		wantCategory   string
	}{
		{
			name:           "price sensitive gets HIGH",
			headline:       "Company Update",
			annType:        "",
			priceSensitive: true,
			wantCategory:   "HIGH",
		},
		{
			name:           "quarterly report gets HIGH",
			headline:       "Quarterly Activities Report",
			annType:        "Periodic Reports",
			priceSensitive: false,
			wantCategory:   "HIGH",
		},
		{
			name:           "dividend announcement gets HIGH",
			headline:       "Dividend Announcement",
			annType:        "Dividend",
			priceSensitive: false,
			wantCategory:   "HIGH",
		},
		{
			name:           "director appointment gets MEDIUM",
			headline:       "Appointment of Director",
			annType:        "Company Administration",
			priceSensitive: false,
			wantCategory:   "MEDIUM",
		},
		{
			name:           "exploration update gets MEDIUM",
			headline:       "Exploration Results",
			annType:        "Progress Report",
			priceSensitive: false,
			wantCategory:   "MEDIUM",
		},
		{
			name:           "appendix 3B gets LOW",
			headline:       "Appendix 3B - Proposed Issue of Securities",
			annType:        "Company Administration",
			priceSensitive: false,
			wantCategory:   "LOW",
		},
		{
			name:           "cleansing notice gets LOW",
			headline:       "Cleansing Statement",
			annType:        "Company Administration",
			priceSensitive: false,
			wantCategory:   "LOW",
		},
		{
			name:           "unclassified gets NOISE",
			headline:       "Other Notice",
			annType:        "Other",
			priceSensitive: false,
			wantCategory:   "NOISE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			category, _ := ClassifyRelevance(tt.headline, tt.annType, tt.priceSensitive)
			if category != tt.wantCategory {
				t.Errorf("ClassifyRelevance() category = %v, want %v", category, tt.wantCategory)
			}
		})
	}
}

func TestIsRoutineAnnouncement(t *testing.T) {
	tests := []struct {
		name        string
		headline    string
		wantRoutine bool
		wantType    string
	}{
		{
			name:        "appendix 3Y is routine",
			headline:    "Appendix 3Y - Change of Director's Interest",
			wantRoutine: true,
			wantType:    "Director Interest (3Y)",
		},
		{
			name:        "appendix 3B is routine",
			headline:    "Appendix 3B - New Issue",
			wantRoutine: true,
			wantType:    "New Issue (3B)",
		},
		{
			name:        "AGM notice is routine",
			headline:    "Notice of Annual General Meeting",
			wantRoutine: true,
			wantType:    "AGM Notice",
		},
		{
			name:        "cleansing notice is routine",
			headline:    "Cleansing Notice under section 708A",
			wantRoutine: true,
			wantType:    "Cleansing Notice",
		},
		{
			name:        "quarterly report is not routine",
			headline:    "Quarterly Activities Report",
			wantRoutine: false,
			wantType:    "",
		},
		{
			name:        "acquisition is not routine",
			headline:    "Acquisition of New Asset",
			wantRoutine: false,
			wantType:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isRoutine, routineType := IsRoutineAnnouncement(tt.headline)
			if isRoutine != tt.wantRoutine {
				t.Errorf("IsRoutineAnnouncement() isRoutine = %v, want %v", isRoutine, tt.wantRoutine)
			}
			if routineType != tt.wantType {
				t.Errorf("IsRoutineAnnouncement() routineType = %v, want %v", routineType, tt.wantType)
			}
		})
	}
}

func TestDetectTradingHalt(t *testing.T) {
	tests := []struct {
		name              string
		headline          string
		wantTradingHalt   bool
		wantReinstatement bool
	}{
		{
			name:              "trading halt detected",
			headline:          "Trading Halt",
			wantTradingHalt:   true,
			wantReinstatement: false,
		},
		{
			name:              "voluntary suspension detected",
			headline:          "Voluntary Suspension Request",
			wantTradingHalt:   true,
			wantReinstatement: false,
		},
		{
			name:              "reinstatement detected",
			headline:          "Reinstatement to Official Quotation",
			wantTradingHalt:   false,
			wantReinstatement: true,
		},
		{
			name:              "resumption of trading detected",
			headline:          "Resumption of Trading",
			wantTradingHalt:   false,
			wantReinstatement: true,
		},
		{
			name:              "regular announcement not detected",
			headline:          "Quarterly Report",
			wantTradingHalt:   false,
			wantReinstatement: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isHalt, isReinstatement := DetectTradingHalt(tt.headline)
			if isHalt != tt.wantTradingHalt {
				t.Errorf("DetectTradingHalt() isHalt = %v, want %v", isHalt, tt.wantTradingHalt)
			}
			if isReinstatement != tt.wantReinstatement {
				t.Errorf("DetectTradingHalt() isReinstatement = %v, want %v", isReinstatement, tt.wantReinstatement)
			}
		})
	}
}

func TestDetectDividendAnnouncement(t *testing.T) {
	tests := []struct {
		name       string
		headline   string
		annType    string
		wantResult bool
	}{
		{
			name:       "dividend in headline",
			headline:   "Final Dividend Declaration",
			annType:    "",
			wantResult: true,
		},
		{
			name:       "DRP in headline",
			headline:   "DRP Election Notice",
			annType:    "",
			wantResult: true,
		},
		{
			name:       "franking in type",
			headline:   "Distribution Notice",
			annType:    "Franked Distribution",
			wantResult: true,
		},
		{
			name:       "non-dividend",
			headline:   "Quarterly Report",
			annType:    "Periodic Reports",
			wantResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectDividendAnnouncement(tt.headline, tt.annType)
			if result != tt.wantResult {
				t.Errorf("DetectDividendAnnouncement() = %v, want %v", result, tt.wantResult)
			}
		})
	}
}

func TestCalculateSignalNoise(t *testing.T) {
	tests := []struct {
		name            string
		ann             RawAnnouncement
		impact          *PriceImpactData
		isTradingHalt   bool
		isReinstatement bool
		wantRating      SignalNoiseRating
	}{
		{
			name: "routine announcement",
			ann: RawAnnouncement{
				Headline: "Appendix 3Y - Director Interest Change",
			},
			impact:     nil,
			wantRating: SignalNoiseRoutine,
		},
		{
			name: "high signal - significant price change",
			ann: RawAnnouncement{
				Headline:       "Acquisition Announcement",
				PriceSensitive: true,
			},
			impact: &PriceImpactData{
				ChangePercent:     5.0,
				VolumeChangeRatio: 1.5,
			},
			wantRating: SignalNoiseHigh,
		},
		{
			name: "high signal - volume spike",
			ann: RawAnnouncement{
				Headline:       "Major Contract",
				PriceSensitive: true,
			},
			impact: &PriceImpactData{
				ChangePercent:     1.0,
				VolumeChangeRatio: 2.5,
			},
			wantRating: SignalNoiseHigh,
		},
		{
			name: "moderate signal",
			ann: RawAnnouncement{
				Headline:       "Progress Report",
				PriceSensitive: true,
			},
			impact: &PriceImpactData{
				ChangePercent:     1.8,
				VolumeChangeRatio: 1.3,
			},
			wantRating: SignalNoiseModerate,
		},
		{
			name: "low signal",
			ann: RawAnnouncement{
				Headline:       "Update",
				PriceSensitive: false,
			},
			impact: &PriceImpactData{
				ChangePercent:     0.7,
				VolumeChangeRatio: 1.1,
			},
			wantRating: SignalNoiseLow,
		},
		{
			name: "noise - no impact",
			ann: RawAnnouncement{
				Headline:       "Minor Update",
				PriceSensitive: false,
			},
			impact: &PriceImpactData{
				ChangePercent:     0.1,
				VolumeChangeRatio: 1.0,
			},
			wantRating: SignalNoiseNone,
		},
		{
			name: "no price data - price sensitive",
			ann: RawAnnouncement{
				Headline:       "Announcement",
				PriceSensitive: true,
			},
			impact:     nil,
			wantRating: SignalNoiseModerate,
		},
		{
			name: "no price data - trading halt",
			ann: RawAnnouncement{
				Headline: "Trading Halt",
			},
			impact:        nil,
			isTradingHalt: true,
			wantRating:    SignalNoiseLow,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateSignalNoise(tt.ann, tt.impact, tt.isTradingHalt, tt.isReinstatement)
			if result.Rating != tt.wantRating {
				t.Errorf("CalculateSignalNoise() rating = %v, want %v", result.Rating, tt.wantRating)
			}
		})
	}
}

func TestDeduplicateAnnouncements(t *testing.T) {
	date1 := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)
	date2 := time.Date(2026, 1, 7, 10, 0, 0, 0, time.UTC)

	announcements := []RawAnnouncement{
		{Date: date1, Headline: "Appendix 3Y - Director A"},
		{Date: date1, Headline: "Appendix 3Y - Director B"},
		{Date: date1, Headline: "Appendix 3Y - Director C"},
		{Date: date1, Headline: "Quarterly Report"},
		{Date: date2, Headline: "Appendix 3Y - Director D"},
		{Date: date2, Headline: "Trading Update"},
	}

	result, stats := DeduplicateAnnouncements(announcements)

	// Should have 4 unique: 1 Appendix 3Y for each day + Quarterly + Trading Update
	if stats.TotalBefore != 6 {
		t.Errorf("TotalBefore = %d, want 6", stats.TotalBefore)
	}
	if stats.TotalAfter != 4 {
		t.Errorf("TotalAfter = %d, want 4", stats.TotalAfter)
	}
	if stats.DuplicatesFound != 2 {
		t.Errorf("DuplicatesFound = %d, want 2", stats.DuplicatesFound)
	}
	if len(result) != 4 {
		t.Errorf("len(result) = %d, want 4", len(result))
	}
}

func TestCalculatePriceImpact(t *testing.T) {
	// Create price data
	prices := []PriceBar{
		{Date: time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC), Close: 1.00, Volume: 100000},
		{Date: time.Date(2026, 1, 6, 0, 0, 0, 0, time.UTC), Close: 1.02, Volume: 110000},
		{Date: time.Date(2026, 1, 7, 0, 0, 0, 0, time.UTC), Close: 1.05, Volume: 120000},
		{Date: time.Date(2026, 1, 8, 0, 0, 0, 0, time.UTC), Close: 1.10, Volume: 200000},
		{Date: time.Date(2026, 1, 9, 0, 0, 0, 0, time.UTC), Close: 1.12, Volume: 150000},
	}

	// Announcement on Jan 8
	annDate := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)

	impact := CalculatePriceImpact(annDate, prices)

	if impact == nil {
		t.Fatal("CalculatePriceImpact returned nil")
	}

	if impact.PriceBefore != 1.05 {
		t.Errorf("PriceBefore = %f, want 1.05", impact.PriceBefore)
	}
	if impact.PriceAfter != 1.10 {
		t.Errorf("PriceAfter = %f, want 1.10", impact.PriceAfter)
	}

	// Change should be approximately 4.76% ((1.10 - 1.05) / 1.05 * 100)
	expectedChange := ((1.10 - 1.05) / 1.05) * 100
	if impact.ChangePercent < expectedChange-0.1 || impact.ChangePercent > expectedChange+0.1 {
		t.Errorf("ChangePercent = %f, want approximately %f", impact.ChangePercent, expectedChange)
	}
}

func TestProcessAnnouncements(t *testing.T) {
	raw := []RawAnnouncement{
		{
			Date:           time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC),
			Headline:       "Quarterly Activities Report",
			Type:           "Periodic Reports",
			PriceSensitive: true,
		},
		{
			Date:           time.Date(2026, 1, 8, 9, 0, 0, 0, time.UTC),
			Headline:       "Appendix 3Y - Director Interest Change",
			Type:           "Company Administration",
			PriceSensitive: false,
		},
	}

	prices := []PriceBar{
		{Date: time.Date(2026, 1, 7, 0, 0, 0, 0, time.UTC), Close: 1.00, Volume: 100000},
		{Date: time.Date(2026, 1, 8, 0, 0, 0, 0, time.UTC), Close: 1.05, Volume: 150000},
	}

	processed, summary, dedupStats := ProcessAnnouncements(raw, prices)

	if len(processed) != 2 {
		t.Errorf("len(processed) = %d, want 2", len(processed))
	}

	if summary.TotalCount != 2 {
		t.Errorf("TotalCount = %d, want 2", summary.TotalCount)
	}

	if summary.HighRelevanceCount != 1 {
		t.Errorf("HighRelevanceCount = %d, want 1", summary.HighRelevanceCount)
	}

	if dedupStats.DuplicatesFound != 0 {
		t.Errorf("DuplicatesFound = %d, want 0", dedupStats.DuplicatesFound)
	}

	// Check that quarterly report is classified as HIGH
	for _, p := range processed {
		if p.Headline == "Quarterly Activities Report" {
			if p.RelevanceCategory != "HIGH" {
				t.Errorf("Quarterly report category = %s, want HIGH", p.RelevanceCategory)
			}
		}
		// Check that Appendix 3Y is marked as routine
		if p.Headline == "Appendix 3Y - Director Interest Change" {
			if !p.IsRoutine {
				t.Error("Appendix 3Y should be marked as routine")
			}
			if p.SignalNoiseRating != SignalNoiseRoutine {
				t.Errorf("Appendix 3Y signal rating = %s, want ROUTINE", p.SignalNoiseRating)
			}
		}
	}
}
