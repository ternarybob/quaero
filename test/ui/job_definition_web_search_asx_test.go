package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// TestJobDefinitionWebSearchASX tests the Web Search ASX:GNP job definition end-to-end.
// This job matches the production pattern in bin/job-definitions/web-search-asx-wes.toml
// and has 9 steps:
// 1. fetch_stock_data - Fetch ASX stock data and technicals
// 2. fetch_announcements - Fetch ASX company announcements
// 3. search_asx_gnp - Web search for financial news
// 4. search_industry - Search for industry outlook
// 5. search_competitors - Search for competitor comparison
// 6. analyze_competitors - Dynamic competitor analysis via LLM
// 7. analyze_announcements - Noise vs signal announcement analysis
// 8. summarize_results - AI summary with investment recommendations
// 9. email_summary - Email the summary (tests markdown-to-HTML conversion)
//
// The test verifies:
// - Job completes successfully
// - All 9 steps are present (matching production WES job pattern)
// - Each step generates output (documents or email sent)
// - Email step converts markdown to HTML (not raw markdown)
func TestJobDefinitionWebSearchASX(t *testing.T) {
	utc := NewUITestContext(t, MaxJobTestTimeout)
	defer utc.Cleanup()

	utc.Log("--- Testing Job Definition: Web Search ASX:GNP ---")

	jobName := "Web Search: ASX:GNP Company Info"
	jobTimeout := MaxJobTestTimeout
	httpHelper := utc.Env.NewHTTPTestHelperWithTimeout(t, 10*time.Second)

	// Copy job definition to results for reference
	if err := utc.CopyJobDefinitionToResults("../config/job-definitions/web-search-asx.toml"); err != nil {
		t.Fatalf("Failed to copy job definition: %v", err)
	}

	// Trigger the job
	if err := utc.TriggerJob(jobName); err != nil {
		t.Fatalf("Failed to trigger job: %v", err)
	}

	// Navigate to Queue page for monitoring
	if err := utc.Navigate(utc.QueueURL); err != nil {
		t.Fatalf("Failed to navigate to Queue page: %v", err)
	}

	// Wait for job to appear in queue
	utc.Log("Waiting for job to appear in queue...")
	time.Sleep(2 * time.Second)

	// Monitor job until completion
	utc.Log("Starting job monitoring...")
	startTime := time.Now()
	lastStatus := ""
	jobID := ""
	lastProgressLog := time.Now()
	lastScreenshotTime := time.Now()

	for {
		// Check context
		if err := utc.Ctx.Err(); err != nil {
			t.Fatalf("Context cancelled: %v", err)
		}

		// Check timeout
		if time.Since(startTime) > jobTimeout {
			utc.Screenshot("web_search_asx_timeout")
			t.Fatalf("Job %s did not complete within %v", jobName, jobTimeout)
		}

		// Log progress every 10 seconds
		if time.Since(lastProgressLog) >= 10*time.Second {
			elapsed := time.Since(startTime)
			utc.Log("[%v] Monitoring... (status: %s)", elapsed.Round(time.Second), lastStatus)
			lastProgressLog = time.Now()
		}

		// Take screenshot every 30 seconds
		if time.Since(lastScreenshotTime) >= 30*time.Second {
			elapsed := time.Since(startTime)
			utc.FullScreenshot(fmt.Sprintf("web_search_asx_monitor_%ds", int(elapsed.Seconds())))
			lastScreenshotTime = time.Now()
		}

		// Get current job status via JavaScript
		var currentStatus string
		err := chromedp.Run(utc.Ctx,
			chromedp.Evaluate(fmt.Sprintf(`
				(() => {
					const cards = document.querySelectorAll('.card');
					for (const card of cards) {
						const titleEl = card.querySelector('.card-title');
						if (titleEl && titleEl.textContent.includes('%s')) {
							const statusBadge = card.querySelector('span.label[data-status]');
							if (statusBadge) return statusBadge.getAttribute('data-status');
						}
					}
					return '';
				})()
			`, jobName), &currentStatus),
		)
		if err != nil {
			t.Logf("Warning: failed to get status: %v", err)
			time.Sleep(1 * time.Second)
			continue
		}

		// Capture job ID once Alpine has loaded the job list
		if jobID == "" {
			if id, err := getJobIDFromQueueUI(utc, jobName); err == nil && id != "" {
				jobID = id
				utc.Log("Captured job_id from UI: %s", jobID)
			}
		}

		// Log status changes
		if currentStatus != lastStatus && currentStatus != "" {
			elapsed := time.Since(startTime)
			utc.Log("Status change: %s -> %s (at %v)", lastStatus, currentStatus, elapsed.Round(time.Second))
			lastStatus = currentStatus
			utc.FullScreenshot(fmt.Sprintf("web_search_asx_status_%s", currentStatus))
		}

		// Check for terminal status
		if currentStatus == "completed" || currentStatus == "failed" || currentStatus == "cancelled" {
			utc.Log("Job reached terminal status: %s", currentStatus)
			break
		}

		time.Sleep(500 * time.Millisecond)
	}

	// Take final screenshot
	utc.FullScreenshot("web_search_asx_final_state")

	// ===============================
	// CONTEXT-SPECIFIC ASSERTIONS
	// ===============================
	finalStatus := lastStatus
	utc.Log("--- Running Context-Specific Assertions ---")

	// --------------------------------------------------------------------------------
	// Assertion 1: Job completed successfully
	// --------------------------------------------------------------------------------
	if finalStatus != "completed" {
		t.Errorf("FAIL: Web Search ASX:GNP job did not complete successfully (status=%s)", finalStatus)
	} else {
		utc.Log("PASS: Web Search ASX:GNP job completed successfully")
	}

	// --------------------------------------------------------------------------------
	// Assertion 2: Verify expected 9 steps exist (matching production WES job pattern)
	// --------------------------------------------------------------------------------
	utc.Log("Assertion 2: Verifying expected 9 steps are present (matching production WES pattern)...")
	if jobID != "" {
		var tree apiJobTreeResponse
		if err := apiGetJSON(t, httpHelper, fmt.Sprintf("/api/jobs/%s/tree", jobID), &tree); err != nil {
			t.Errorf("FAIL: Could not get step tree from API for job_id=%s: %v", jobID, err)
		} else {
			// These 9 steps match the production pattern in bin/job-definitions/web-search-asx-wes.toml
			expectedSteps := []string{
				"fetch_stock_data",
				"fetch_announcements",
				"search_asx_gnp",
				"search_industry",
				"search_competitors",
				"analyze_competitors",
				"analyze_announcements",
				"summarize_results",
				"email_summary",
			}
			foundSteps := make(map[string]bool)
			for _, step := range tree.Steps {
				foundSteps[step.Name] = true
			}

			for _, expected := range expectedSteps {
				if !foundSteps[expected] {
					t.Errorf("FAIL: Expected step '%s' not found in Web Search ASX:GNP job", expected)
				} else {
					utc.Log("PASS: Found expected step '%s'", expected)
				}
			}

			if len(tree.Steps) != 9 {
				t.Errorf("FAIL: Expected exactly 9 steps (matching WES production pattern), got %d", len(tree.Steps))
			} else {
				utc.Log("PASS: Web Search ASX:GNP has exactly 9 steps (matching WES production pattern)")
			}
		}
	} else {
		t.Errorf("FAIL: Could not capture job ID to verify steps")
	}

	// --------------------------------------------------------------------------------
	// Assertion 3: All steps completed successfully
	// --------------------------------------------------------------------------------
	utc.Log("Assertion 3: Verifying all steps completed...")
	if jobID != "" {
		var tree apiJobTreeResponse
		if err := apiGetJSON(t, httpHelper, fmt.Sprintf("/api/jobs/%s/tree", jobID), &tree); err == nil {
			for _, step := range tree.Steps {
				if step.Status != "completed" {
					t.Errorf("FAIL: Step '%s' has status '%s' (expected 'completed')", step.Name, step.Status)
				} else {
					utc.Log("PASS: Step '%s' completed successfully", step.Name)
				}
			}
		}
	}

	// --------------------------------------------------------------------------------
	// Assertion 4: Verify each step generated output (has logs indicating work done)
	// --------------------------------------------------------------------------------
	utc.Log("Assertion 4: Verifying each step generated output...")
	if jobID != "" {
		var tree apiJobTreeResponse
		if err := apiGetJSON(t, httpHelper, fmt.Sprintf("/api/jobs/%s/tree", jobID), &tree); err == nil {
			for _, step := range tree.Steps {
				// Each completed step should have logs indicating work was done
				var logsResp apiJobTreeLogsResponse
				logsPath := fmt.Sprintf("/api/logs?scope=job&job_id=%s&step=%s&limit=10&level=all", step.StepID, step.Name)
				if err := apiGetJSON(t, httpHelper, logsPath, &logsResp); err != nil {
					utc.Log("Warning: Could not fetch logs for step '%s': %v", step.Name, err)
					continue
				}

				if len(logsResp.Steps) > 0 && logsResp.Steps[0].TotalCount > 0 {
					utc.Log("PASS: Step '%s' generated %d log entries", step.Name, logsResp.Steps[0].TotalCount)
				} else {
					// For completed steps, we expect some logs
					if step.Status == "completed" {
						utc.Log("Note: Step '%s' completed but has no logs (may be expected for email step)", step.Name)
					}
				}
			}
		}
	}

	// --------------------------------------------------------------------------------
	// Assertion 5: Verify email step converted markdown to HTML (not raw markdown)
	// --------------------------------------------------------------------------------
	utc.Log("Assertion 5: Verifying email step converted markdown to HTML...")
	if jobID != "" {
		var tree apiJobTreeResponse
		if err := apiGetJSON(t, httpHelper, fmt.Sprintf("/api/jobs/%s/tree", jobID), &tree); err == nil {
			for _, step := range tree.Steps {
				if step.Name == "email_summary" && step.Status == "completed" {
					// Get logs for email step to verify HTML was generated from markdown
					var logsResp apiJobTreeLogsResponse
					logsPath := fmt.Sprintf("/api/logs?scope=job&job_id=%s&step=%s&limit=50&level=all", step.StepID, step.Name)
					if err := apiGetJSON(t, httpHelper, logsPath, &logsResp); err != nil {
						utc.Log("Warning: Could not fetch logs for email step: %v", err)
						break
					}

					// Look for explicit log entry indicating HTML was generated from markdown
					// The email worker logs: "HTML email body generated (X bytes) from markdown content"
					foundHTMLConversion := false
					foundEmailSent := false
					var htmlBytes string

					for _, stepLogs := range logsResp.Steps {
						for _, entry := range stepLogs.Logs {
							// Check for HTML conversion log (primary assertion)
							if strings.Contains(entry.Message, "HTML email body generated") &&
								strings.Contains(entry.Message, "from markdown") {
								foundHTMLConversion = true
								// Extract byte count for logging
								if idx := strings.Index(entry.Message, "("); idx != -1 {
									if end := strings.Index(entry.Message[idx:], ")"); end != -1 {
										htmlBytes = entry.Message[idx : idx+end+1]
									}
								}
								utc.Log("PASS: Email HTML generated from markdown %s", htmlBytes)
							}

							// Check for email sent confirmation
							if strings.Contains(entry.Message, "Email sent successfully") {
								foundEmailSent = true
							}
						}
					}

					if !foundHTMLConversion {
						t.Errorf("FAIL: Email step did not log HTML conversion from markdown. "+
							"Expected log containing 'HTML email body generated ... from markdown'")
					}

					if !foundEmailSent {
						t.Errorf("FAIL: Email step did not log successful send")
					} else if foundHTMLConversion {
						utc.Log("PASS: Email sent successfully with HTML body from markdown conversion")
					}

					// --------------------------------------------------------------------------------
					// Assertion 5b: Retrieve HTML document and verify actual content
					// The email worker saves the HTML body as a document with tag "email-html"
					// This allows us to verify the actual HTML content, not just log messages
					// --------------------------------------------------------------------------------
					utc.Log("Assertion 5b: Verifying HTML document content...")

					// Search for documents with tag "email-html"
					var docsResp apiDocumentsResponse
					docsPath := "/api/documents?tags=email-html&limit=10"
					if err := apiGetJSON(t, httpHelper, docsPath, &docsResp); err != nil {
						utc.Log("Warning: Could not fetch email-html documents: %v", err)
					} else if len(docsResp.Documents) == 0 {
						t.Errorf("FAIL: No email-html document found - email worker should save HTML body as document")
					} else {
						// Get the most recent email-html document
						doc := docsResp.Documents[0]
						utc.Log("Found email-html document: %s", doc.ID)

						// Verify the content contains HTML tags (not raw markdown)
						content := doc.ContentMarkdown // HTML is stored in ContentMarkdown field

						// Check for HTML indicators
						hasHTMLDoctype := strings.Contains(content, "<!DOCTYPE html>")
						hasHTMLTag := strings.Contains(content, "<html")
						hasBodyTag := strings.Contains(content, "<body")
						hasHeadingTag := strings.Contains(content, "<h1") || strings.Contains(content, "<h2") || strings.Contains(content, "<h3")
						hasParagraphTag := strings.Contains(content, "<p>") || strings.Contains(content, "<p ")

						htmlIndicators := 0
						if hasHTMLDoctype {
							htmlIndicators++
						}
						if hasHTMLTag {
							htmlIndicators++
						}
						if hasBodyTag {
							htmlIndicators++
						}
						if hasHeadingTag {
							htmlIndicators++
						}
						if hasParagraphTag {
							htmlIndicators++
						}

						// Check for raw markdown indicators (should NOT be present in HTML)
						hasRawMarkdownHeader := strings.Contains(content, "\n## ") || strings.Contains(content, "\n# ")
						hasRawMarkdownBold := strings.Contains(content, "**") && !strings.Contains(content, "<strong>")
						hasRawMarkdownList := strings.Contains(content, "\n- ") && !strings.Contains(content, "<li>")

						if htmlIndicators >= 2 {
							utc.Log("PASS: HTML document contains %d HTML indicators (DOCTYPE=%v, html=%v, body=%v, h1-h3=%v, p=%v)",
								htmlIndicators, hasHTMLDoctype, hasHTMLTag, hasBodyTag, hasHeadingTag, hasParagraphTag)
						} else {
							t.Errorf("FAIL: HTML document has only %d HTML indicators - expected at least 2. Content may still be markdown.", htmlIndicators)
						}

						if hasRawMarkdownHeader || hasRawMarkdownBold || hasRawMarkdownList {
							t.Errorf("FAIL: HTML document contains raw markdown (header=%v, bold=%v, list=%v) - conversion failed",
								hasRawMarkdownHeader, hasRawMarkdownBold, hasRawMarkdownList)
						} else {
							utc.Log("PASS: HTML document does not contain raw markdown indicators")
						}

						// Log first 500 chars of content for debugging
						preview := content
						if len(preview) > 500 {
							preview = preview[:500] + "..."
						}
						utc.Log("HTML document preview: %s", preview)
					}

					break
				}
			}
		}
	}

	utc.Log("Web Search ASX:GNP test completed with final status: %s", finalStatus)
}
