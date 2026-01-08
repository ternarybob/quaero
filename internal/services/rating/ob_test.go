package rating

import (
	"testing"
	"time"
)

func TestCalculateOB(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name          string
		announcements []Announcement
		bfsScore      int
		wantScore     float64
		wantCatalyst  bool
		wantTimeframe bool
	}{
		{
			name:          "no announcements",
			announcements: []Announcement{},
			bfsScore:      2,
			wantScore:     0.0,
			wantCatalyst:  false,
			wantTimeframe: false,
		},
		{
			name: "weak BFS blocks optionality",
			announcements: []Announcement{
				{Date: now, Headline: "drilling results expected q1"},
			},
			bfsScore:      0, // Weak foundation
			wantScore:     0.0,
			wantCatalyst:  false,
			wantTimeframe: false,
		},
		{
			name: "catalyst with timeframe - full bonus",
			announcements: []Announcement{
				{Date: now, Headline: "drilling results expected q1"},
			},
			bfsScore:      1,
			wantScore:     1.0,
			wantCatalyst:  true,
			wantTimeframe: true,
		},
		{
			name: "catalyst without timeframe - partial bonus",
			announcements: []Announcement{
				{Date: now, Headline: "drilling program underway"},
			},
			bfsScore:      1,
			wantScore:     0.5,
			wantCatalyst:  true,
			wantTimeframe: false,
		},
		{
			name: "no catalyst found",
			announcements: []Announcement{
				{Date: now, Headline: "annual general meeting notice"},
			},
			bfsScore:      2,
			wantScore:     0.0,
			wantCatalyst:  false,
			wantTimeframe: false,
		},
		{
			name: "old announcements ignored",
			announcements: []Announcement{
				{Date: now.AddDate(0, 0, -100), Headline: "drilling results expected q1"},
			},
			bfsScore:      1,
			wantScore:     0.0, // Announcement too old
			wantCatalyst:  false,
			wantTimeframe: false,
		},
		{
			name: "contract with expected timeframe",
			announcements: []Announcement{
				{Date: now, Headline: "major contract expected to commence january"},
			},
			bfsScore:      2,
			wantScore:     1.0,
			wantCatalyst:  true,
			wantTimeframe: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateOB(tt.announcements, tt.bfsScore)

			if result.Score != tt.wantScore {
				t.Errorf("CalculateOB() score = %f, want %f", result.Score, tt.wantScore)
			}
			if result.CatalystFound != tt.wantCatalyst {
				t.Errorf("CalculateOB() catalystFound = %v, want %v",
					result.CatalystFound, tt.wantCatalyst)
			}
			if result.TimeframeFound != tt.wantTimeframe {
				t.Errorf("CalculateOB() timeframeFound = %v, want %v",
					result.TimeframeFound, tt.wantTimeframe)
			}
			if result.Reasoning == "" {
				t.Error("CalculateOB() reasoning should not be empty")
			}
		})
	}
}

func TestHasCatalyst(t *testing.T) {
	tests := []struct {
		headline string
		want     bool
	}{
		{"drilling program commenced", true},
		{"quarterly results released", true},
		{"new contract signed", true},
		{"acquisition completed", true},
		{"merger proposal", true},
		{"fda approval received", true},
		{"trial results positive", true},
		{"production milestone", true},
		{"commissioning complete", true},
		{"offtake agreement signed", true},
		{"resource upgrade announced", true},
		{"feasibility study complete", true},
		{"license granted", true},
		{"permit approved", true},
		{"general meeting notice", false},
		{"director appointment", false},
		{"appendix 4g released", false},
	}

	for _, tt := range tests {
		t.Run(tt.headline, func(t *testing.T) {
			got := hasCatalyst(tt.headline)
			if got != tt.want {
				t.Errorf("hasCatalyst(%q) = %v, want %v", tt.headline, got, tt.want)
			}
		})
	}
}

func TestHasTimeframe(t *testing.T) {
	tests := []struct {
		headline string
		want     bool
	}{
		{"expected q1 2025", true},
		{"results in q2", true},
		{"delivery in 2025", true},
		{"commencing january", true},
		{"due in march", true},
		{"this quarter", true},
		{"next quarter", true},
		{"this month", true},
		{"next month", true},
		{"imminent announcement", true},
		{"pending approval", true},
		{"expected soon", true},
		{"scheduled for release", true},
		{"planned for completion", true},
		{"within 2 weeks", true},
		{"within 3 months", true},
		{"general update", false},
		{"progress report", false},
	}

	for _, tt := range tests {
		t.Run(tt.headline, func(t *testing.T) {
			got := hasTimeframe(tt.headline)
			if got != tt.want {
				t.Errorf("hasTimeframe(%q) = %v, want %v", tt.headline, got, tt.want)
			}
		})
	}
}
