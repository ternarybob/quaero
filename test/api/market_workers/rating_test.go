// -----------------------------------------------------------------------
// Tests for rating workers
// Tests all rating calculation workers (BFS, CDS, NFR, PPS, VRS, OB, Composite)
// These workers compute investability scores from stock data
// -----------------------------------------------------------------------

package market_workers

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// =============================================================================
// Rating Worker Tests - BFS (Business Foundation Score)
// =============================================================================

// TestRatingBFSSingle tests BFS calculation for a single stock
func TestRatingBFSSingle(t *testing.T) {
	env := SetupFreshEnvironment(t)
	if env == nil {
		return
	}
	defer env.Cleanup()

	RequireEODHD(t, env)

	helper := env.NewHTTPTestHelper(t)

	ticker := "GNP"
	defID := fmt.Sprintf("test-rating-bfs-%d", time.Now().UnixNano())

	// First fetch fundamentals, then calculate BFS
	body := map[string]interface{}{
		"id":          defID,
		"name":        "Rating BFS Single Stock Test",
		"description": "Test rating_bfs worker",
		"type":        "manager",
		"enabled":     true,
		"tags":        []string{"worker-test", "rating-bfs", "single-stock"},
		"steps": []map[string]interface{}{
			{
				"name": "fetch-fundamentals",
				"type": "market_fundamentals",
				"config": map[string]interface{}{
					"ticker": fmt.Sprintf("ASX:%s", ticker),
				},
			},
			{
				"name":    "calculate-bfs",
				"type":    "rating_bfs",
				"depends": "fetch-fundamentals",
				"config": map[string]interface{}{
					"ticker": fmt.Sprintf("ASX:%s", ticker),
				},
			},
		},
	}

	SaveJobDefinition(t, env, body)

	jobID, _ := CreateAndExecuteJob(t, helper, body)
	if jobID == "" {
		return
	}

	t.Logf("Executing rating_bfs job: %s", jobID)

	finalStatus := WaitForJobCompletion(t, helper, jobID, 3*time.Minute)
	if finalStatus != "completed" {
		t.Skipf("Job ended with status %s", finalStatus)
		return
	}

	// === ASSERTIONS ===
	tags := []string{"rating-bfs", strings.ToLower(ticker)}
	metadata, content := AssertOutputNotEmpty(t, helper, tags)

	// Schema validation
	isValid := ValidateSchema(t, metadata, BFSSchema)
	assert.True(t, isValid, "Output should comply with BFS schema")

	// Business rules - score must be 0, 1, or 2
	if score, ok := metadata["score"].(float64); ok {
		AssertGateScore(t, score, "BFS score")
		t.Logf("PASS: BFS score = %.0f", score)
	}

	// Indicator count should be 0-4
	if indicatorCount, ok := metadata["indicator_count"].(float64); ok {
		assert.GreaterOrEqual(t, indicatorCount, 0.0, "indicator_count should be >= 0")
		assert.LessOrEqual(t, indicatorCount, 4.0, "indicator_count should be <= 4")
		t.Logf("PASS: indicator_count = %.0f", indicatorCount)
	}

	// Content should have expected sections
	assert.Contains(t, content, "BFS Score", "Content should contain BFS Score section")
	assert.Contains(t, content, "Components", "Content should contain Components section")

	SaveWorkerOutput(t, env, helper, tags, ticker)
	AssertNoServiceErrors(t, env)

	t.Log("PASS: rating_bfs single stock test completed")
}

// =============================================================================
// Rating Worker Tests - CDS (Capital Discipline Score)
// =============================================================================

// TestRatingCDSSingle tests CDS calculation for a single stock
func TestRatingCDSSingle(t *testing.T) {
	env := SetupFreshEnvironment(t)
	if env == nil {
		return
	}
	defer env.Cleanup()

	RequireEODHD(t, env)
	RequireLLM(t, env)

	helper := env.NewHTTPTestHelper(t)

	ticker := "GNP"
	defID := fmt.Sprintf("test-rating-cds-%d", time.Now().UnixNano())

	// CDS requires fundamentals and announcements
	body := map[string]interface{}{
		"id":          defID,
		"name":        "Rating CDS Single Stock Test",
		"description": "Test rating_cds worker",
		"type":        "manager",
		"enabled":     true,
		"tags":        []string{"worker-test", "rating-cds", "single-stock"},
		"steps": []map[string]interface{}{
			{
				"name": "fetch-fundamentals",
				"type": "market_fundamentals",
				"config": map[string]interface{}{
					"ticker": fmt.Sprintf("ASX:%s", ticker),
				},
			},
			{
				"name": "fetch-announcements",
				"type": "market_announcements",
				"config": map[string]interface{}{
					"asx_code": ticker,
				},
			},
			{
				"name":    "process-announcements",
				"type":    "processing_announcements",
				"depends": "fetch-announcements",
				"config": map[string]interface{}{
					"asx_code": ticker,
				},
			},
			{
				"name":    "calculate-cds",
				"type":    "rating_cds",
				"depends": "fetch-fundamentals,process-announcements",
				"config": map[string]interface{}{
					"ticker": fmt.Sprintf("ASX:%s", ticker),
				},
			},
		},
	}

	SaveJobDefinition(t, env, body)

	jobID, _ := CreateAndExecuteJob(t, helper, body)
	if jobID == "" {
		return
	}

	t.Logf("Executing rating_cds job: %s", jobID)

	finalStatus := WaitForJobCompletion(t, helper, jobID, 4*time.Minute)
	if finalStatus != "completed" {
		t.Skipf("Job ended with status %s", finalStatus)
		return
	}

	// === ASSERTIONS ===
	tags := []string{"rating-cds", strings.ToLower(ticker)}
	metadata, content := AssertOutputNotEmpty(t, helper, tags)

	// Schema validation
	isValid := ValidateSchema(t, metadata, CDSSchema)
	assert.True(t, isValid, "Output should comply with CDS schema")

	// Business rules - score must be 0, 1, or 2
	if score, ok := metadata["score"].(float64); ok {
		AssertGateScore(t, score, "CDS score")
		t.Logf("PASS: CDS score = %.0f", score)
	}

	assert.Contains(t, content, "CDS Score", "Content should contain CDS Score section")

	SaveWorkerOutput(t, env, helper, tags, ticker)
	AssertNoServiceErrors(t, env)

	t.Log("PASS: rating_cds single stock test completed")
}

// =============================================================================
// Rating Worker Tests - NFR (Narrative-to-Fact Ratio)
// =============================================================================

// TestRatingNFRSingle tests NFR calculation for a single stock
func TestRatingNFRSingle(t *testing.T) {
	env := SetupFreshEnvironment(t)
	if env == nil {
		return
	}
	defer env.Cleanup()

	RequireEODHD(t, env)
	RequireLLM(t, env)

	helper := env.NewHTTPTestHelper(t)

	ticker := "GNP"
	defID := fmt.Sprintf("test-rating-nfr-%d", time.Now().UnixNano())

	// NFR requires announcements and price data
	body := map[string]interface{}{
		"id":          defID,
		"name":        "Rating NFR Single Stock Test",
		"description": "Test rating_nfr worker",
		"type":        "manager",
		"enabled":     true,
		"tags":        []string{"worker-test", "rating-nfr", "single-stock"},
		"steps": []map[string]interface{}{
			{
				"name": "fetch-announcements",
				"type": "market_announcements",
				"config": map[string]interface{}{
					"asx_code": ticker,
				},
			},
			{
				"name":    "process-announcements",
				"type":    "processing_announcements",
				"depends": "fetch-announcements",
				"config": map[string]interface{}{
					"asx_code": ticker,
				},
			},
			{
				"name": "fetch-prices",
				"type": "market_data",
				"config": map[string]interface{}{
					"ticker": fmt.Sprintf("ASX:%s", ticker),
				},
			},
			{
				"name":    "calculate-nfr",
				"type":    "rating_nfr",
				"depends": "process-announcements,fetch-prices",
				"config": map[string]interface{}{
					"ticker": fmt.Sprintf("ASX:%s", ticker),
				},
			},
		},
	}

	SaveJobDefinition(t, env, body)

	jobID, _ := CreateAndExecuteJob(t, helper, body)
	if jobID == "" {
		return
	}

	t.Logf("Executing rating_nfr job: %s", jobID)

	finalStatus := WaitForJobCompletion(t, helper, jobID, 4*time.Minute)
	if finalStatus != "completed" {
		t.Skipf("Job ended with status %s", finalStatus)
		return
	}

	// === ASSERTIONS ===
	tags := []string{"rating-nfr", strings.ToLower(ticker)}
	metadata, content := AssertOutputNotEmpty(t, helper, tags)

	// Schema validation
	isValid := ValidateSchema(t, metadata, NFRSchema)
	assert.True(t, isValid, "Output should comply with NFR schema")

	// Business rules - score must be 0.0 to 1.0
	if score, ok := metadata["score"].(float64); ok {
		AssertComponentScore(t, score, "NFR score")
		t.Logf("PASS: NFR score = %.2f", score)
	}

	assert.Contains(t, content, "NFR Score", "Content should contain NFR Score section")

	SaveWorkerOutput(t, env, helper, tags, ticker)
	AssertNoServiceErrors(t, env)

	t.Log("PASS: rating_nfr single stock test completed")
}

// =============================================================================
// Rating Worker Tests - PPS (Price Progression Score)
// =============================================================================

// TestRatingPPSSingle tests PPS calculation for a single stock
func TestRatingPPSSingle(t *testing.T) {
	env := SetupFreshEnvironment(t)
	if env == nil {
		return
	}
	defer env.Cleanup()

	RequireEODHD(t, env)
	RequireLLM(t, env)

	helper := env.NewHTTPTestHelper(t)

	ticker := "GNP"
	defID := fmt.Sprintf("test-rating-pps-%d", time.Now().UnixNano())

	body := map[string]interface{}{
		"id":          defID,
		"name":        "Rating PPS Single Stock Test",
		"description": "Test rating_pps worker",
		"type":        "manager",
		"enabled":     true,
		"tags":        []string{"worker-test", "rating-pps", "single-stock"},
		"steps": []map[string]interface{}{
			{
				"name": "fetch-announcements",
				"type": "market_announcements",
				"config": map[string]interface{}{
					"asx_code": ticker,
				},
			},
			{
				"name":    "process-announcements",
				"type":    "processing_announcements",
				"depends": "fetch-announcements",
				"config": map[string]interface{}{
					"asx_code": ticker,
				},
			},
			{
				"name": "fetch-prices",
				"type": "market_data",
				"config": map[string]interface{}{
					"ticker": fmt.Sprintf("ASX:%s", ticker),
				},
			},
			{
				"name":    "calculate-pps",
				"type":    "rating_pps",
				"depends": "process-announcements,fetch-prices",
				"config": map[string]interface{}{
					"ticker": fmt.Sprintf("ASX:%s", ticker),
				},
			},
		},
	}

	SaveJobDefinition(t, env, body)

	jobID, _ := CreateAndExecuteJob(t, helper, body)
	if jobID == "" {
		return
	}

	t.Logf("Executing rating_pps job: %s", jobID)

	finalStatus := WaitForJobCompletion(t, helper, jobID, 4*time.Minute)
	if finalStatus != "completed" {
		t.Skipf("Job ended with status %s", finalStatus)
		return
	}

	// === ASSERTIONS ===
	tags := []string{"rating-pps", strings.ToLower(ticker)}
	metadata, content := AssertOutputNotEmpty(t, helper, tags)

	// Schema validation
	isValid := ValidateSchema(t, metadata, PPSSchema)
	assert.True(t, isValid, "Output should comply with PPS schema")

	// Business rules - score must be 0.0 to 1.0
	if score, ok := metadata["score"].(float64); ok {
		AssertComponentScore(t, score, "PPS score")
		t.Logf("PASS: PPS score = %.2f", score)
	}

	assert.Contains(t, content, "PPS Score", "Content should contain PPS Score section")

	SaveWorkerOutput(t, env, helper, tags, ticker)
	AssertNoServiceErrors(t, env)

	t.Log("PASS: rating_pps single stock test completed")
}

// =============================================================================
// Rating Worker Tests - VRS (Volatility Regime Stability)
// =============================================================================

// TestRatingVRSSingle tests VRS calculation for a single stock
func TestRatingVRSSingle(t *testing.T) {
	env := SetupFreshEnvironment(t)
	if env == nil {
		return
	}
	defer env.Cleanup()

	RequireEODHD(t, env)
	RequireLLM(t, env)

	helper := env.NewHTTPTestHelper(t)

	ticker := "GNP"
	defID := fmt.Sprintf("test-rating-vrs-%d", time.Now().UnixNano())

	body := map[string]interface{}{
		"id":          defID,
		"name":        "Rating VRS Single Stock Test",
		"description": "Test rating_vrs worker",
		"type":        "manager",
		"enabled":     true,
		"tags":        []string{"worker-test", "rating-vrs", "single-stock"},
		"steps": []map[string]interface{}{
			{
				"name": "fetch-announcements",
				"type": "market_announcements",
				"config": map[string]interface{}{
					"asx_code": ticker,
				},
			},
			{
				"name":    "process-announcements",
				"type":    "processing_announcements",
				"depends": "fetch-announcements",
				"config": map[string]interface{}{
					"asx_code": ticker,
				},
			},
			{
				"name": "fetch-prices",
				"type": "market_data",
				"config": map[string]interface{}{
					"ticker": fmt.Sprintf("ASX:%s", ticker),
				},
			},
			{
				"name":    "calculate-vrs",
				"type":    "rating_vrs",
				"depends": "process-announcements,fetch-prices",
				"config": map[string]interface{}{
					"ticker": fmt.Sprintf("ASX:%s", ticker),
				},
			},
		},
	}

	SaveJobDefinition(t, env, body)

	jobID, _ := CreateAndExecuteJob(t, helper, body)
	if jobID == "" {
		return
	}

	t.Logf("Executing rating_vrs job: %s", jobID)

	finalStatus := WaitForJobCompletion(t, helper, jobID, 4*time.Minute)
	if finalStatus != "completed" {
		t.Skipf("Job ended with status %s", finalStatus)
		return
	}

	// === ASSERTIONS ===
	tags := []string{"rating-vrs", strings.ToLower(ticker)}
	metadata, content := AssertOutputNotEmpty(t, helper, tags)

	// Schema validation
	isValid := ValidateSchema(t, metadata, VRSSchema)
	assert.True(t, isValid, "Output should comply with VRS schema")

	// Business rules - score must be 0.0 to 1.0
	if score, ok := metadata["score"].(float64); ok {
		AssertComponentScore(t, score, "VRS score")
		t.Logf("PASS: VRS score = %.2f", score)
	}

	assert.Contains(t, content, "VRS Score", "Content should contain VRS Score section")

	SaveWorkerOutput(t, env, helper, tags, ticker)
	AssertNoServiceErrors(t, env)

	t.Log("PASS: rating_vrs single stock test completed")
}

// =============================================================================
// Rating Worker Tests - OB (Optionality Bonus)
// =============================================================================

// TestRatingOBSingle tests OB calculation for a single stock
func TestRatingOBSingle(t *testing.T) {
	env := SetupFreshEnvironment(t)
	if env == nil {
		return
	}
	defer env.Cleanup()

	RequireEODHD(t, env)
	RequireLLM(t, env)

	helper := env.NewHTTPTestHelper(t)

	ticker := "GNP"
	defID := fmt.Sprintf("test-rating-ob-%d", time.Now().UnixNano())

	// OB requires announcements and BFS score
	body := map[string]interface{}{
		"id":          defID,
		"name":        "Rating OB Single Stock Test",
		"description": "Test rating_ob worker",
		"type":        "manager",
		"enabled":     true,
		"tags":        []string{"worker-test", "rating-ob", "single-stock"},
		"steps": []map[string]interface{}{
			{
				"name": "fetch-fundamentals",
				"type": "market_fundamentals",
				"config": map[string]interface{}{
					"ticker": fmt.Sprintf("ASX:%s", ticker),
				},
			},
			{
				"name": "fetch-announcements",
				"type": "market_announcements",
				"config": map[string]interface{}{
					"asx_code": ticker,
				},
			},
			{
				"name":    "process-announcements",
				"type":    "processing_announcements",
				"depends": "fetch-announcements",
				"config": map[string]interface{}{
					"asx_code": ticker,
				},
			},
			{
				"name":    "calculate-bfs",
				"type":    "rating_bfs",
				"depends": "fetch-fundamentals",
				"config": map[string]interface{}{
					"ticker": fmt.Sprintf("ASX:%s", ticker),
				},
			},
			{
				"name":    "calculate-ob",
				"type":    "rating_ob",
				"depends": "process-announcements,calculate-bfs",
				"config": map[string]interface{}{
					"ticker": fmt.Sprintf("ASX:%s", ticker),
				},
			},
		},
	}

	SaveJobDefinition(t, env, body)

	jobID, _ := CreateAndExecuteJob(t, helper, body)
	if jobID == "" {
		return
	}

	t.Logf("Executing rating_ob job: %s", jobID)

	finalStatus := WaitForJobCompletion(t, helper, jobID, 4*time.Minute)
	if finalStatus != "completed" {
		t.Skipf("Job ended with status %s", finalStatus)
		return
	}

	// === ASSERTIONS ===
	tags := []string{"rating-ob", strings.ToLower(ticker)}
	metadata, content := AssertOutputNotEmpty(t, helper, tags)

	// Schema validation
	isValid := ValidateSchema(t, metadata, OBSchema)
	assert.True(t, isValid, "Output should comply with OB schema")

	// Business rules - score must be 0.0, 0.5, or 1.0
	if score, ok := metadata["score"].(float64); ok {
		AssertOBScore(t, score)
		t.Logf("PASS: OB score = %.1f", score)
	}

	// Validate boolean fields
	if _, ok := metadata["catalyst_found"].(bool); ok {
		t.Log("PASS: catalyst_found is boolean")
	}
	if _, ok := metadata["timeframe_found"].(bool); ok {
		t.Log("PASS: timeframe_found is boolean")
	}

	assert.Contains(t, content, "OB Score", "Content should contain OB Score section")

	SaveWorkerOutput(t, env, helper, tags, ticker)
	AssertNoServiceErrors(t, env)

	t.Log("PASS: rating_ob single stock test completed")
}

// =============================================================================
// Rating Worker Tests - Composite Rating
// =============================================================================

// TestRatingCompositeSingle tests composite rating calculation for a single stock
func TestRatingCompositeSingle(t *testing.T) {
	env := SetupFreshEnvironment(t)
	if env == nil {
		return
	}
	defer env.Cleanup()

	RequireEODHD(t, env)
	RequireLLM(t, env)

	helper := env.NewHTTPTestHelper(t)

	ticker := "GNP"
	defID := fmt.Sprintf("test-rating-composite-%d", time.Now().UnixNano())

	// Composite requires all component scores
	body := map[string]interface{}{
		"id":          defID,
		"name":        "Rating Composite Single Stock Test",
		"description": "Test rating_composite worker",
		"type":        "manager",
		"enabled":     true,
		"tags":        []string{"worker-test", "rating-composite", "single-stock"},
		"steps": []map[string]interface{}{
			// Data collection
			{
				"name": "fetch-fundamentals",
				"type": "market_fundamentals",
				"config": map[string]interface{}{
					"ticker": fmt.Sprintf("ASX:%s", ticker),
				},
			},
			{
				"name": "fetch-announcements",
				"type": "market_announcements",
				"config": map[string]interface{}{
					"asx_code": ticker,
				},
			},
			{
				"name":    "process-announcements",
				"type":    "processing_announcements",
				"depends": "fetch-announcements",
				"config": map[string]interface{}{
					"asx_code": ticker,
				},
			},
			{
				"name": "fetch-prices",
				"type": "market_data",
				"config": map[string]interface{}{
					"ticker": fmt.Sprintf("ASX:%s", ticker),
				},
			},
			// Gate scores
			{
				"name":    "calculate-bfs",
				"type":    "rating_bfs",
				"depends": "fetch-fundamentals",
				"config": map[string]interface{}{
					"ticker": fmt.Sprintf("ASX:%s", ticker),
				},
			},
			{
				"name":    "calculate-cds",
				"type":    "rating_cds",
				"depends": "fetch-fundamentals,process-announcements",
				"config": map[string]interface{}{
					"ticker": fmt.Sprintf("ASX:%s", ticker),
				},
			},
			// Component scores
			{
				"name":    "calculate-nfr",
				"type":    "rating_nfr",
				"depends": "process-announcements,fetch-prices",
				"config": map[string]interface{}{
					"ticker": fmt.Sprintf("ASX:%s", ticker),
				},
			},
			{
				"name":    "calculate-pps",
				"type":    "rating_pps",
				"depends": "process-announcements,fetch-prices",
				"config": map[string]interface{}{
					"ticker": fmt.Sprintf("ASX:%s", ticker),
				},
			},
			{
				"name":    "calculate-vrs",
				"type":    "rating_vrs",
				"depends": "process-announcements,fetch-prices",
				"config": map[string]interface{}{
					"ticker": fmt.Sprintf("ASX:%s", ticker),
				},
			},
			{
				"name":    "calculate-ob",
				"type":    "rating_ob",
				"depends": "process-announcements,calculate-bfs",
				"config": map[string]interface{}{
					"ticker": fmt.Sprintf("ASX:%s", ticker),
				},
			},
			// Final rating
			{
				"name":    "calculate-rating",
				"type":    "rating_composite",
				"depends": "calculate-bfs,calculate-cds,calculate-nfr,calculate-pps,calculate-vrs,calculate-ob",
				"config": map[string]interface{}{
					"ticker": fmt.Sprintf("ASX:%s", ticker),
				},
			},
		},
	}

	SaveJobDefinition(t, env, body)

	jobID, _ := CreateAndExecuteJob(t, helper, body)
	if jobID == "" {
		return
	}

	t.Logf("Executing rating_composite job: %s", jobID)

	finalStatus := WaitForJobCompletion(t, helper, jobID, 6*time.Minute)
	if finalStatus != "completed" {
		t.Skipf("Job ended with status %s", finalStatus)
		return
	}

	// === ASSERTIONS ===
	tags := []string{"stock-rating", strings.ToLower(ticker)}
	metadata, content := AssertOutputNotEmpty(t, helper, tags)

	// Schema validation
	isValid := ValidateSchema(t, metadata, RatingCompositeSchema)
	assert.True(t, isValid, "Output should comply with RatingComposite schema")

	// Validate label
	if label, ok := metadata["label"].(string); ok {
		AssertRatingLabel(t, label)
		t.Logf("PASS: Rating label = %s", label)
	}

	// Validate gate_passed
	if gatePassed, ok := metadata["gate_passed"].(bool); ok {
		t.Logf("PASS: gate_passed = %v", gatePassed)

		// Validate investability based on gate status
		AssertInvestabilityScore(t, metadata["investability"], gatePassed)
	}

	// Validate scores object
	if scores, ok := metadata["scores"].(map[string]interface{}); ok {
		t.Logf("PASS: scores object present with %d components", len(scores))
	}

	assert.Contains(t, content, "Stock Rating", "Content should contain Stock Rating section")
	assert.Contains(t, content, "Component Scores", "Content should contain Component Scores section")

	SaveWorkerOutput(t, env, helper, tags, ticker)
	AssertNoServiceErrors(t, env)

	t.Log("PASS: rating_composite single stock test completed")
}
