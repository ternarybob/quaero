package githublogs

import (
	"bufio"
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/go-github/v57/github"
)

// JobContext holds the result of the log retrieval and processing.
type JobContext struct {
	RepoName     string
	Conclusion   string
	SanitizedLog string
}

// ParseLogURL extracts owner, repo, and jobID from a GitHub Action URL.
// Expected format: https://github.com/owner/repo/actions/runs/runID/job/jobID
func ParseLogURL(rawURL string) (owner, repo string, jobID int64, err error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", "", 0, fmt.Errorf("invalid URL: %w", err)
	}

	// Path should look like /owner/repo/actions/runs/runID/job/jobID
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 7 || parts[2] != "actions" || parts[3] != "runs" || parts[5] != "job" {
		return "", "", 0, fmt.Errorf("unexpected URL format, expected .../actions/runs/<runID>/job/<jobID>")
	}

	owner = parts[0]
	repo = parts[1]
	jobIDStr := parts[6]

	jobID, err = strconv.ParseInt(jobIDStr, 10, 64)
	if err != nil {
		return "", "", 0, fmt.Errorf("invalid job ID: %w", err)
	}

	return owner, repo, jobID, nil
}

// GetJobLog fetches the raw logs for a specific job ID.
func GetJobLog(ctx context.Context, client *github.Client, owner, repo string, jobID int64) (string, error) {
	// Get the job details first to check status/conclusion if needed,
	// but for now we just want the logs.
	// The GetWorkflowJobLogs API redirects to a raw URL.
	// go-github handles the redirect if we use the right method.

	url, _, err := client.Actions.GetWorkflowJobLogs(ctx, owner, repo, jobID, 10)
	if err != nil {
		return "", fmt.Errorf("failed to get job logs URL: %w", err)
	}

	// The URL returned is a redirect location to the raw log (e.g. blob storage).
	// We need to fetch the content from that URL.
	// Since it might be a signed URL to Azure/S3, we use a standard http client.
	// However, go-github's GetWorkflowJobLogs with followRedirects=true (last arg)
	// returns the *URL* to the logs, not the logs themselves, if I recall correctly,
	// OR it returns the logs if we don't pass a writer?
	// Wait, looking at go-github docs/source:
	// func (s *ActionsService) GetWorkflowJobLogs(ctx context.Context, owner, repo string, jobID int64, followRedirects bool) (*url.URL, *Response, error)
	// It returns the URL. We then need to fetch it.

	// Let's use the http client from the github client context if possible, or just http.Get
	// But the URL might require auth headers if it's internal, though usually these are signed SAS URLs.
	// Let's assume standard http.Get works for the signed URL.

	resp, err := client.Client().Get(url.String())
	if err != nil {
		return "", fmt.Errorf("failed to fetch raw logs from %s: %w", url.String(), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("failed to fetch logs, status: %d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)
	var sb strings.Builder
	for scanner.Scan() {
		sb.WriteString(scanner.Text())
		sb.WriteByte('\n')
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error reading log response: %w", err)
	}

	return sb.String(), nil
}

// SanitizeLogForAI processes the raw log to make it suitable for LLM consumption.
func SanitizeLogForAI(rawLog string) string {
	lines := strings.Split(rawLog, "\n")
	var sanitizedLines []string

	// Regex to strip timestamps (e.g., "2023-10-27T10:00:00.1234567Z ")
	// Standard GitHub timestamps are ISO8601 at the start of the line.
	timestampRegex := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d+Z\s+`)

	// Keywords to look for
	errorKeywords := []string{"error", "fatal", "panic", "fail"}

	// Helper to check if line contains keywords
	isErrorLine := func(line string) bool {
		lower := strings.ToLower(line)
		for _, kw := range errorKeywords {
			if strings.Contains(lower, kw) {
				return true
			}
		}
		return false
	}

	// We want to keep:
	// 1. Lines around errors (context window)
	// 2. The last 50 lines (summary)

	keepIndices := make(map[int]bool)
	totalLines := len(lines)
	contextWindow := 10
	tailLines := 50

	// Identify lines to keep based on errors
	for i, line := range lines {
		// Strip timestamp for processing (and for final output)
		cleanLine := timestampRegex.ReplaceAllString(line, "")
		lines[i] = cleanLine // Update the line in place to be clean

		if isErrorLine(cleanLine) {
			start := i - contextWindow
			if start < 0 {
				start = 0
			}
			end := i + contextWindow
			if end >= totalLines {
				end = totalLines - 1
			}
			for k := start; k <= end; k++ {
				keepIndices[k] = true
			}
		}
	}

	// Identify lines to keep based on tail
	startTail := totalLines - tailLines
	if startTail < 0 {
		startTail = 0
	}
	for i := startTail; i < totalLines; i++ {
		keepIndices[i] = true
	}

	// Construct result
	// We iterate through all lines to maintain order
	lastAddedIndex := -1
	for i := 0; i < totalLines; i++ {
		if keepIndices[i] {
			if lastAddedIndex != -1 && i > lastAddedIndex+1 {
				sanitizedLines = append(sanitizedLines, "...") // Indicate skipped lines
			}
			sanitizedLines = append(sanitizedLines, lines[i])
			lastAddedIndex = i
		}
	}

	return strings.Join(sanitizedLines, "\n")
}
