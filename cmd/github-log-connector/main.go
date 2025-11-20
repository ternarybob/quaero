package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/google/go-github/v57/github"
	"github.com/ternarybob/quaero/internal/githublogs"
	"golang.org/x/oauth2"
)

func main() {
	urlFlag := flag.String("url", "", "GitHub Action Job URL (e.g., https://github.com/owner/repo/actions/runs/runID/job/jobID)")
	flag.Parse()

	if *urlFlag == "" {
		fmt.Println("Error: -url flag is required")
		flag.Usage()
		os.Exit(1)
	}

	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		fmt.Println("Error: GITHUB_TOKEN environment variable is required")
		os.Exit(1)
	}

	// 1. Parse URL
	owner, repo, jobID, err := githublogs.ParseLogURL(*urlFlag)
	if err != nil {
		fmt.Printf("Error parsing URL: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Fetching logs for %s/%s Job ID: %d...\n", owner, repo, jobID)

	// 2. Authenticate
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	// 3. Get Logs
	rawLog, err := githublogs.GetJobLog(ctx, client, owner, repo, jobID)
	if err != nil {
		fmt.Printf("Error fetching logs: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Raw log length: %d bytes. Sanitizing...\n", len(rawLog))

	// 4. Sanitize
	sanitizedLog := githublogs.SanitizeLogForAI(rawLog)

	// 5. Output
	fmt.Println("---------------------------------------------------")
	fmt.Println("SANITIZED LOG OUTPUT START")
	fmt.Println("---------------------------------------------------")
	fmt.Println(sanitizedLog)
	fmt.Println("---------------------------------------------------")
	fmt.Println("SANITIZED LOG OUTPUT END")
	fmt.Println("---------------------------------------------------")

	// Create JobContext struct as requested (demonstration)
	jobCtx := githublogs.JobContext{
		RepoName:     repo,
		Conclusion:   "unknown", // We didn't fetch the job status in this simplified flow
		SanitizedLog: sanitizedLog,
	}

	// Just to show we populated it
	_ = jobCtx
}
