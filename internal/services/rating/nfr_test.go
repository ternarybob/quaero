package rating

import (
	"testing"
	"time"
)

func TestCalculateNFR(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name          string
		announcements []Announcement
		wantScore     float64
		tolerance     float64
	}{
		{
			name:          "no announcements - neutral",
			announcements: []Announcement{},
			wantScore:     0.5,
			tolerance:     0.001,
		},
		{
			name: "all fact-based - high score",
			announcements: []Announcement{
				{Date: now, Type: TypeQuarterly},
				{Date: now, Type: TypeAnnualReport},
				{Date: now, Type: TypeContract},
				{Date: now, Type: TypeAcquisition},
			},
			wantScore: 1.0,
			tolerance: 0.001,
		},
		{
			name: "all narrative-based - low score",
			announcements: []Announcement{
				{Date: now, Type: TypeDrilling},
				{Date: now, Type: TypeTradingHalt},
				{Date: now, Type: TypeOther},
			},
			wantScore: 0.0,
			tolerance: 0.001,
		},
		{
			name: "mixed - 50/50",
			announcements: []Announcement{
				{Date: now, Type: TypeQuarterly},
				{Date: now, Type: TypeAnnualReport},
				{Date: now, Type: TypeDrilling},
				{Date: now, Type: TypeTradingHalt},
			},
			wantScore: 0.5,
			tolerance: 0.001,
		},
		{
			name: "mostly fact-based - high score",
			announcements: []Announcement{
				{Date: now, Type: TypeQuarterly},
				{Date: now, Type: TypeAnnualReport},
				{Date: now, Type: TypeContract},
				{Date: now, Type: TypeDrilling},
			},
			wantScore: 0.75,
			tolerance: 0.001,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateNFR(tt.announcements)
			if result.Score < tt.wantScore-tt.tolerance || result.Score > tt.wantScore+tt.tolerance {
				t.Errorf("CalculateNFR() score = %f, want %f", result.Score, tt.wantScore)
			}
			if result.Reasoning == "" {
				t.Error("CalculateNFR() reasoning should not be empty")
			}

			// Verify components match
			if len(tt.announcements) > 0 {
				if result.Components.TotalAnnouncements != len(tt.announcements) {
					t.Errorf("CalculateNFR() total = %d, want %d",
						result.Components.TotalAnnouncements, len(tt.announcements))
				}
			}
		})
	}
}

func TestIsFactBased(t *testing.T) {
	tests := []struct {
		annType  AnnouncementType
		wantFact bool
	}{
		{TypeQuarterly, true},
		{TypeAnnualReport, true},
		{TypeContract, true},
		{TypeAcquisition, true},
		{TypeTradingHalt, false},
		{TypeCapitalRaise, false},
		{TypeDrilling, false},
		{TypeOther, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.annType), func(t *testing.T) {
			got := isFactBased(tt.annType)
			if got != tt.wantFact {
				t.Errorf("isFactBased(%s) = %v, want %v", tt.annType, got, tt.wantFact)
			}
		})
	}
}
