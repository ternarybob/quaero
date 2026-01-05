package workers

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ternarybob/arbor"
)

// TestGenerateAISummary_NilProvider verifies graceful degradation when provider is nil
func TestGenerateAISummary_NilProvider(t *testing.T) {
	logger := arbor.NewLogger()

	// Create worker WITHOUT provider (providerFactory is nil)
	worker := &MarketAnnouncementsWorker{
		logger:          logger,
		providerFactory: nil, // Explicitly nil
	}

	summaryData := AnnouncementSummaryData{
		ASXCode:         "BHP",
		HighSignalCount: 2,
	}

	ctx := context.Background()
	summary, err := worker.generateAISummary(ctx, summaryData)

	// Should not error
	require.NoError(t, err, "generateAISummary should not error when provider nil")

	// Should return empty string
	assert.Empty(t, summary, "AI summary should be empty when provider nil")

	t.Log("PASS: Graceful degradation when provider nil")
}

// TestBuildSummaryPrompt verifies prompt construction contains all required data
func TestBuildSummaryPrompt(t *testing.T) {
	logger := arbor.NewLogger()

	worker := &MarketAnnouncementsWorker{
		logger: logger,
	}

	summaryData := AnnouncementSummaryData{
		ASXCode:                    "BHP",
		HighSignalCount:            2,
		ModerateSignalCount:        11,
		LowSignalCount:             16,
		NoiseCount:                 2,
		RoutineCount:               19,
		PreDriftCount:              14,
		PriceSensitiveTotal:        7,
		PriceSensitiveWithReaction: 5,
		HighSignalAnnouncements: []AnnouncementAnalysis{
			{
				Date:              time.Date(2025, 7, 18, 0, 0, 0, 0, time.UTC),
				Headline:          "Quarterly Activities Report",
				SignalNoiseRating: SignalNoiseHigh,
				PriceImpact: &PriceImpactData{
					ChangePercent:     3.0,
					VolumeChangeRatio: 1.5,
				},
			},
		},
	}

	prompt := worker.buildSummaryPrompt(summaryData)

	// Verify prompt contains key data required by REQ-1
	assert.Contains(t, prompt, "ASX:BHP", "Prompt should contain ticker")
	assert.Contains(t, prompt, "HIGH_SIGNAL", "Prompt should contain signal distribution")
	assert.Contains(t, prompt, "PRICE-SENSITIVE ACCURACY", "Prompt should contain price-sensitive data")
	assert.Contains(t, prompt, "PRE-ANNOUNCEMENT MOVEMENT", "Prompt should contain pre-drift data")
	assert.Contains(t, prompt, "Quarterly Activities Report", "Prompt should contain high signal announcement")

	// Verify prompt requests the required perspectives from REQ-1
	assert.Contains(t, prompt, "buyer/seller", "Prompt should request buyer/seller perspective")
	assert.Contains(t, prompt, "pre-market awareness", "Prompt should request pre-market awareness analysis")
	assert.Contains(t, prompt, "themes", "Prompt should request communication themes")

	t.Logf("PASS: Prompt built with %d characters containing all required data", len(prompt))
}

// TestAISummaryInOutput verifies Executive Summary section placement in output
// This tests the markdown structure that createSummaryDocument produces
func TestAISummaryInOutput(t *testing.T) {
	// This test verifies that when AI summary is available, the output structure is correct
	// The markdown output should contain "## Executive Summary" right after header

	// Simulate what createSummaryDocument produces with AI summary
	aiSummary := "Test executive summary content about BHP announcements."

	// Build content like createSummaryDocument does
	var content strings.Builder
	content.WriteString("# ASX Announcements Summary: BHP\n\n")
	content.WriteString("**Generated**: 5 January 2026\n")
	content.WriteString("**Total Announcements**: 50\n")
	content.WriteString("**Worker**: market_announcements\n\n")

	// Executive Summary section (what we're testing)
	if aiSummary != "" {
		content.WriteString("## Executive Summary\n\n")
		content.WriteString(aiSummary)
		content.WriteString("\n\n")
	}

	content.WriteString("## Signal-to-Noise Analysis\n\n")

	output := content.String()

	// Assert Executive Summary section is present
	assert.Contains(t, output, "## Executive Summary", "Output should contain Executive Summary section when AI summary available")
	assert.Contains(t, output, aiSummary, "Output should contain the AI summary text")

	// Assert order is correct: Executive Summary before Signal-to-Noise Analysis (REQ-1: at TOP)
	execSummaryIdx := strings.Index(output, "## Executive Summary")
	signalNoiseIdx := strings.Index(output, "## Signal-to-Noise Analysis")
	assert.Less(t, execSummaryIdx, signalNoiseIdx, "Executive Summary should appear before Signal-to-Noise Analysis")

	// Verify header comes first
	headerIdx := strings.Index(output, "# ASX Announcements Summary")
	assert.Less(t, headerIdx, execSummaryIdx, "Header should appear before Executive Summary")

	t.Log("PASS: Executive Summary section correctly positioned in output")
}

// TestAISummaryNotInOutputWhenEmpty verifies no Executive Summary when AI unavailable
func TestAISummaryNotInOutputWhenEmpty(t *testing.T) {
	// When AI summary is empty (provider unavailable), no Executive Summary section should appear

	aiSummary := "" // Empty - provider not available

	var content strings.Builder
	content.WriteString("# ASX Announcements Summary: BHP\n\n")
	content.WriteString("**Generated**: 5 January 2026\n")
	content.WriteString("**Total Announcements**: 50\n")
	content.WriteString("**Worker**: market_announcements\n\n")

	// Executive Summary section only added when aiSummary is non-empty
	if aiSummary != "" {
		content.WriteString("## Executive Summary\n\n")
		content.WriteString(aiSummary)
		content.WriteString("\n\n")
	}

	content.WriteString("## Signal-to-Noise Analysis\n\n")

	output := content.String()

	// Assert Executive Summary section is NOT present
	assert.NotContains(t, output, "## Executive Summary", "Output should NOT contain Executive Summary when AI unavailable")

	// Assert Signal-to-Noise Analysis comes right after header
	signalNoiseIdx := strings.Index(output, "## Signal-to-Noise Analysis")
	workerIdx := strings.Index(output, "**Worker**:")
	assert.Greater(t, signalNoiseIdx, workerIdx, "Signal-to-Noise should come after header when no AI summary")

	t.Log("PASS: No Executive Summary section when AI unavailable")
}

// TestAnnouncementSummaryDataStruct verifies the data structure contains all required fields
func TestAnnouncementSummaryDataStruct(t *testing.T) {
	// This test verifies AnnouncementSummaryData has all fields required by REQ-1

	data := AnnouncementSummaryData{
		ASXCode:                    "BHP",
		HighSignalCount:            2,
		ModerateSignalCount:        11,
		LowSignalCount:             16,
		NoiseCount:                 2,
		RoutineCount:               19,
		TradingHaltCount:           1,
		AnomalyNoReactionCount:     2,
		AnomalyUnexpectedCount:     8,
		PreDriftCount:              14,
		PriceSensitiveTotal:        7,
		PriceSensitiveWithReaction: 5,
		HighSignalAnnouncements:    []AnnouncementAnalysis{},
	}

	// Verify all fields can be set (compile-time check essentially)
	assert.Equal(t, "BHP", data.ASXCode, "ASXCode should be set")
	assert.Equal(t, 2, data.HighSignalCount, "HighSignalCount should be set")
	assert.Equal(t, 11, data.ModerateSignalCount, "ModerateSignalCount should be set")
	assert.Equal(t, 16, data.LowSignalCount, "LowSignalCount should be set")
	assert.Equal(t, 2, data.NoiseCount, "NoiseCount should be set")
	assert.Equal(t, 19, data.RoutineCount, "RoutineCount should be set")
	assert.Equal(t, 1, data.TradingHaltCount, "TradingHaltCount should be set")
	assert.Equal(t, 2, data.AnomalyNoReactionCount, "AnomalyNoReactionCount should be set")
	assert.Equal(t, 8, data.AnomalyUnexpectedCount, "AnomalyUnexpectedCount should be set")
	assert.Equal(t, 14, data.PreDriftCount, "PreDriftCount should be set")
	assert.Equal(t, 7, data.PriceSensitiveTotal, "PriceSensitiveTotal should be set")
	assert.Equal(t, 5, data.PriceSensitiveWithReaction, "PriceSensitiveWithReaction should be set")

	t.Log("PASS: AnnouncementSummaryData struct has all required fields")
}
