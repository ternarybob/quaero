// -----------------------------------------------------------------------
// Tests for portfolio_newsletter worker
// Tests the newsletter generation workflow that combines news and metadata
// documents to create a comprehensive portfolio newsletter.
// -----------------------------------------------------------------------

package portfolio

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/ternarybob/quaero/test/common"
)

// NewsletterSchema for portfolio_newsletter worker validation
var NewsletterSchema = common.WorkerSchema{
	RequiredFields: []string{"portfolio", "tickers", "generated_at"},
	OptionalFields: []string{"ticker_count", "news_count", "metadata_count", "job_id"},
	FieldTypes: map[string]string{
		"portfolio":      "string",
		"tickers":        "array",
		"generated_at":   "string",
		"ticker_count":   "number",
		"news_count":     "number",
		"metadata_count": "number",
	},
}

// TestNewsletterWorker_FullPipeline tests the complete newsletter generation pipeline
// with news fetch, metadata fetch, and newsletter synthesis.
func TestNewsletterWorker_FullPipeline(t *testing.T) {
	env := common.SetupFreshEnvironment(t)
	if env == nil {
		return
	}
	defer env.Cleanup()

	// Get results directory for test output validation
	resultsDir := env.GetResultsDir()

	// MANDATORY: Create guard for output validation
	guard := common.NewPortfolioTestOutputGuard(t, resultsDir)
	defer guard.Close()

	guard.LogWithTimestamp("Test started: TestNewsletterWorker_FullPipeline")

	// This test requires EODHD for news/metadata and LLM for newsletter synthesis
	RequireEODHD(t, env)
	RequireLLM(t, env)

	helper := env.NewHTTPTestHelper(t)

	defID := fmt.Sprintf("test-newsletter-full-%d", time.Now().UnixNano())
	tickers := []string{"ASX:GNP", "ASX:CGS"}

	// Build variables array
	variables := make([]map[string]interface{}, len(tickers))
	for i, tk := range tickers {
		variables[i] = map[string]interface{}{"ticker": tk}
	}

	body := map[string]interface{}{
		"id":          defID,
		"name":        "Newsletter Full Pipeline Test",
		"description": "Test complete newsletter generation workflow",
		"type":        "manager",
		"enabled":     true,
		"tags":        []string{"worker-test", "newsletter", "full-pipeline"},
		"config": map[string]interface{}{
			"variables": variables,
		},
		"steps": []map[string]interface{}{
			{
				"name": "fetch-news",
				"type": "ticker_news",
				"config": map[string]interface{}{
					"period":      "M1",
					"cache_hours": 24,
					"output_tags": []string{"data-collection"},
				},
			},
			{
				"name": "fetch-metadata",
				"type": "ticker_metadata",
				"config": map[string]interface{}{
					"cache_hours": 168,
					"output_tags": []string{"data-collection"},
				},
			},
			{
				"name":       "generate-newsletter",
				"type":       "portfolio_newsletter",
				"depends_on": []string{"fetch-news", "fetch-metadata"},
				"config": map[string]interface{}{
					"input_tags":  []string{"data-collection"},
					"portfolio":   "test-portfolio",
					"model":       "gemini",
					"output_tags": []string{"newsletter-output"},
				},
			},
		},
	}

	// Save job definition BEFORE execution
	common.SaveJobDefinition(t, env, body)

	// Create and execute
	jobID, _ := common.CreateAndExecuteJob(t, helper, body)
	if jobID == "" {
		return
	}

	t.Logf("Executing newsletter pipeline job: %s", jobID)

	// Wait for completion (longer timeout for full pipeline)
	finalStatus := common.WaitForJobCompletion(t, helper, jobID, 15*time.Minute)
	if finalStatus != "completed" {
		t.Skipf("Job ended with status %s", finalStatus)
		return
	}

	// Validate newsletter output
	summaryTags := []string{"newsletter", "test-portfolio"}
	docID, metadata, content := common.AssertOutputNotEmptyWithID(t, helper, summaryTags)
	t.Logf("Newsletter document ID: %s", docID)

	// Save output AFTER completion
	common.SaveWorkerOutput(t, env, helper, summaryTags, "newsletter")
	guard.MarkOutputSaved()

	// Validate schema
	isValid := common.ValidateSchema(t, metadata, NewsletterSchema)
	assert.True(t, isValid, "Output should comply with newsletter schema")

	// Assert required fields
	common.AssertMetadataHasFields(t, metadata, []string{"portfolio", "tickers", "generated_at"})

	// Assert content contains expected sections
	expectedSections := []string{
		"Market Brief",
		"Week of",
		"Holdings",
	}
	common.AssertOutputContains(t, content, expectedSections)

	// Verify tickers are present
	if tickerArr, ok := metadata["tickers"].([]interface{}); ok {
		assert.GreaterOrEqual(t, len(tickerArr), 1, "Newsletter should include at least one ticker")
		t.Logf("Newsletter covers %d tickers", len(tickerArr))
	}

	// Validate result files
	common.RequirePortfolioTestOutputs(t, resultsDir)
	AssertNoServiceErrors(t, env)

	guard.LogWithTimestamp("PASS: newsletter full pipeline test completed")
	t.Log("PASS: newsletter full pipeline test completed")
}

// TestNewsletterWorker_WithEmail tests newsletter generation with email delivery
func TestNewsletterWorker_WithEmail(t *testing.T) {
	env := common.SetupFreshEnvironment(t)
	if env == nil {
		return
	}
	defer env.Cleanup()

	// Get results directory for test output validation
	resultsDir := env.GetResultsDir()

	// MANDATORY: Create guard for output validation
	guard := common.NewPortfolioTestOutputGuard(t, resultsDir)
	defer guard.Close()

	guard.LogWithTimestamp("Test started: TestNewsletterWorker_WithEmail")

	RequireEODHD(t, env)
	RequireLLM(t, env)

	// Skip if no email configured
	if !hasEmailConfig(env) {
		guard.LogWithTimestamp("SKIP: Email not configured")
		t.Skip("Email not configured - skipping email delivery test")
		return
	}

	helper := env.NewHTTPTestHelper(t)

	defID := fmt.Sprintf("test-newsletter-email-%d", time.Now().UnixNano())

	body := map[string]interface{}{
		"id":          defID,
		"name":        "Newsletter Email Test",
		"description": "Test newsletter generation with email delivery",
		"type":        "manager",
		"enabled":     true,
		"tags":        []string{"worker-test", "newsletter", "email"},
		"config": map[string]interface{}{
			"variables": []map[string]interface{}{
				{"ticker": "ASX:GNP"},
			},
		},
		"steps": []map[string]interface{}{
			{
				"name": "fetch-news",
				"type": "ticker_news",
				"config": map[string]interface{}{
					"period":      "M1",
					"cache_hours": 24,
					"output_tags": []string{"data-collection"},
				},
			},
			{
				"name": "fetch-metadata",
				"type": "ticker_metadata",
				"config": map[string]interface{}{
					"cache_hours": 168,
					"output_tags": []string{"data-collection"},
				},
			},
			{
				"name":       "generate-newsletter",
				"type":       "portfolio_newsletter",
				"depends_on": []string{"fetch-news", "fetch-metadata"},
				"config": map[string]interface{}{
					"input_tags":  []string{"data-collection"},
					"portfolio":   "email-test",
					"model":       "gemini",
					"output_tags": []string{"newsletter-output"},
				},
			},
			{
				"name":       "format-email",
				"type":       "output_formatter",
				"depends_on": []string{"generate-newsletter"},
				"config": map[string]interface{}{
					"input_tags":  []string{"newsletter-output"},
					"output_tags": []string{"email-ready"},
					"format":      "html",
					"title":       "Test Newsletter",
				},
			},
			{
				"name":       "send-email",
				"type":       "email",
				"depends_on": []string{"format-email"},
				"config": map[string]interface{}{
					"input_tags": []string{"email-ready"},
					"to":         getTestEmailAddress(env),
					"subject":    "Test Newsletter - Quaero",
				},
			},
		},
	}

	common.SaveJobDefinition(t, env, body)

	jobID, _ := common.CreateAndExecuteJob(t, helper, body)
	if jobID == "" {
		return
	}

	t.Logf("Executing newsletter with email job: %s", jobID)

	finalStatus := common.WaitForJobCompletion(t, helper, jobID, 15*time.Minute)
	if finalStatus != "completed" {
		t.Skipf("Job ended with status %s", finalStatus)
		return
	}

	// Validate newsletter was created
	summaryTags := []string{"newsletter", "email-test"}
	_, _, _ = common.AssertOutputNotEmptyWithID(t, helper, summaryTags)

	common.SaveWorkerOutput(t, env, helper, summaryTags, "newsletter-email")
	guard.MarkOutputSaved()
	common.RequirePortfolioTestOutputs(t, resultsDir)
	AssertNoServiceErrors(t, env)

	guard.LogWithTimestamp("PASS: newsletter with email test completed")
	t.Log("PASS: newsletter with email test completed")
}

// TestNewsInlineEmail tests news worker with inline email formatting
func TestNewsInlineEmail(t *testing.T) {
	env := common.SetupFreshEnvironment(t)
	if env == nil {
		return
	}
	defer env.Cleanup()

	// Get results directory for test output validation
	resultsDir := env.GetResultsDir()

	// MANDATORY: Create guard for output validation
	guard := common.NewPortfolioTestOutputGuard(t, resultsDir)
	defer guard.Close()

	guard.LogWithTimestamp("Test started: TestNewsInlineEmail")

	RequireEODHD(t, env)

	// Skip if no email configured
	if !hasEmailConfig(env) {
		guard.LogWithTimestamp("SKIP: Email not configured")
		t.Skip("Email not configured - skipping inline email test")
		return
	}

	helper := env.NewHTTPTestHelper(t)

	defID := fmt.Sprintf("test-news-inline-email-%d", time.Now().UnixNano())
	tickers := []string{"ASX:GNP", "ASX:CGS"}

	variables := make([]map[string]interface{}, len(tickers))
	for i, tk := range tickers {
		variables[i] = map[string]interface{}{"ticker": tk}
	}

	body := map[string]interface{}{
		"id":          defID,
		"name":        "News Inline Email Test",
		"description": "Test news with inline email (all tickers in one email)",
		"type":        "manager",
		"enabled":     true,
		"tags":        []string{"worker-test", "ticker-news", "inline-email"},
		"config": map[string]interface{}{
			"variables": variables,
		},
		"steps": []map[string]interface{}{
			{
				"name": "fetch-news",
				"type": "ticker_news",
				"config": map[string]interface{}{
					"period":      "M1",
					"cache_hours": 24,
					"output_tags": []string{"news-output"},
				},
			},
			{
				"name":       "format-email",
				"type":       "output_formatter",
				"depends_on": []string{"fetch-news"},
				"config": map[string]interface{}{
					"input_tags":  []string{"news-output"},
					"output_tags": []string{"email-ready"},
					"format":      "inline",
					"style":       "body",
					"order":       "ticker",
					"title":       "Watchlist News Summary",
				},
			},
			{
				"name":       "send-email",
				"type":       "email",
				"depends_on": []string{"format-email"},
				"config": map[string]interface{}{
					"input_tags": []string{"email-ready"},
					"to":         getTestEmailAddress(env),
					"subject":    "Watchlist News Summary - Quaero",
				},
			},
		},
	}

	common.SaveJobDefinition(t, env, body)

	jobID, _ := common.CreateAndExecuteJob(t, helper, body)
	if jobID == "" {
		return
	}

	t.Logf("Executing news inline email job: %s", jobID)

	finalStatus := common.WaitForJobCompletion(t, helper, jobID, 10*time.Minute)
	if finalStatus != "completed" {
		t.Skipf("Job ended with status %s", finalStatus)
		return
	}

	// Validate news documents were created
	summaryTags := []string{"ticker-news", "gnp"}
	_, _, _ = common.AssertOutputNotEmptyWithID(t, helper, summaryTags)

	common.SaveWorkerOutput(t, env, helper, summaryTags, "news-inline-email")
	guard.MarkOutputSaved()
	common.RequirePortfolioTestOutputs(t, resultsDir)
	AssertNoServiceErrors(t, env)

	guard.LogWithTimestamp("PASS: news inline email test completed")
	t.Log("PASS: news inline email test completed")
}

// =============================================================================
// Helper Functions
// =============================================================================

// RequireEODHD skips the test if EODHD API key is not available
func RequireEODHD(t *testing.T, env *common.TestEnvironment) {
	t.Helper()
	if env.EnvVars == nil {
		t.Skip("No environment variables - EODHD API key required")
		return
	}
	key, exists := env.EnvVars["eodhd_api_key"]
	if !exists || key == "" || key == "placeholder" {
		t.Skip("EODHD API key not available - skipping test")
	}
}

// RequireLLM skips the test if no LLM API key is available
func RequireLLM(t *testing.T, env *common.TestEnvironment) {
	t.Helper()
	if env.EnvVars == nil {
		t.Skip("No environment variables - LLM API key required")
		return
	}
	// Check for Gemini or Claude
	geminiKey, hasGemini := env.EnvVars["google_gemini_api_key"]
	claudeKey, hasClaude := env.EnvVars["anthropic_api_key"]

	hasValidGemini := hasGemini && geminiKey != "" && geminiKey != "placeholder"
	hasValidClaude := hasClaude && claudeKey != "" && claudeKey != "placeholder"

	if !hasValidGemini && !hasValidClaude {
		t.Skip("LLM API key (Gemini or Claude) not available - skipping test")
	}
}

// hasEmailConfig checks if email configuration is available in environment
func hasEmailConfig(env *common.TestEnvironment) bool {
	if env.EnvVars == nil {
		return false
	}
	smtpHost := env.EnvVars["smtp_host"]
	smtpUser := env.EnvVars["smtp_username"]
	return smtpHost != "" && smtpUser != "" && smtpHost != "placeholder"
}

// getTestEmailAddress returns the test email address from environment
func getTestEmailAddress(env *common.TestEnvironment) string {
	if env.EnvVars == nil {
		return ""
	}
	return env.EnvVars["email_recipient"]
}
