package github

import (
	"context"
	"encoding/base64"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/go-github/v57/github"
)

// RepoFile represents a file from a GitHub repository
type RepoFile struct {
	Path        string // Full path: src/components/Button.tsx
	Folder      string // Parent folder: src/components/
	Name        string // File name: Button.tsx
	SHA         string // File SHA
	Size        int    // File size in bytes
	Content     string // Decoded content (for text files)
	URL         string // GitHub URL
	DownloadURL string // Raw download URL
}

// RepoBranch represents branch info
type RepoBranch struct {
	Name      string
	CommitSHA string
	Protected bool
}

// WorkflowRun represents a GitHub Actions workflow run
type WorkflowRun struct {
	ID           int64
	Name         string
	WorkflowName string
	Status       string // queued, in_progress, completed
	Conclusion   string // success, failure, cancelled, skipped
	Branch       string
	CommitSHA    string
	RunStartedAt time.Time
	RunAttempt   int
	URL          string
}

// ListBranches returns all branches for a repo
func (c *Connector) ListBranches(ctx context.Context, owner, repo string) ([]RepoBranch, error) {
	var allBranches []RepoBranch

	opts := &github.BranchListOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}

	for {
		branches, resp, err := c.client.Repositories.ListBranches(ctx, owner, repo, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list branches: %w", err)
		}

		for _, b := range branches {
			branch := RepoBranch{
				Name:      b.GetName(),
				Protected: b.GetProtected(),
			}
			if b.Commit != nil {
				branch.CommitSHA = b.Commit.GetSHA()
			}
			allBranches = append(allBranches, branch)
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allBranches, nil
}

// ListFiles returns all files in a repo for a given branch
// Filters by extension (e.g., ".go", ".ts", ".md")
// Excludes binary files and specified paths
func (c *Connector) ListFiles(ctx context.Context, owner, repo, branch string, extensions []string, excludePaths []string) ([]RepoFile, error) {
	// Get the tree recursively
	tree, _, err := c.client.Git.GetTree(ctx, owner, repo, branch, true)
	if err != nil {
		return nil, fmt.Errorf("failed to get tree: %w", err)
	}

	var files []RepoFile

	// Build extension map for quick lookup
	extMap := make(map[string]bool)
	for _, ext := range extensions {
		extMap[strings.ToLower(ext)] = true
	}

	for _, entry := range tree.Entries {
		// Skip directories and submodules
		if entry.GetType() != "blob" {
			continue
		}

		path := entry.GetPath()

		// Check exclude paths
		if shouldExclude(path, excludePaths) {
			continue
		}

		// Check extension filter (if provided)
		if len(extensions) > 0 {
			ext := strings.ToLower(filepath.Ext(path))
			if !extMap[ext] {
				continue
			}
		}

		// Skip likely binary files by extension
		if isBinaryExtension(path) {
			continue
		}

		file := RepoFile{
			Path:   path,
			Folder: filepath.Dir(path),
			Name:   filepath.Base(path),
			SHA:    entry.GetSHA(),
			Size:   entry.GetSize(),
			URL:    fmt.Sprintf("https://github.com/%s/%s/blob/%s/%s", owner, repo, branch, path),
		}

		files = append(files, file)
	}

	return files, nil
}

// GetFileContent fetches the content of a single file
func (c *Connector) GetFileContent(ctx context.Context, owner, repo, branch, path string) (*RepoFile, error) {
	content, _, _, err := c.client.Repositories.GetContents(ctx, owner, repo, path, &github.RepositoryContentGetOptions{
		Ref: branch,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get file content: %w", err)
	}

	if content == nil {
		return nil, fmt.Errorf("file not found: %s", path)
	}

	file := &RepoFile{
		Path:        content.GetPath(),
		Folder:      filepath.Dir(content.GetPath()),
		Name:        content.GetName(),
		SHA:         content.GetSHA(),
		Size:        content.GetSize(),
		URL:         content.GetHTMLURL(),
		DownloadURL: content.GetDownloadURL(),
	}

	// Decode content (base64)
	if content.Content != nil {
		decoded, err := base64.StdEncoding.DecodeString(*content.Content)
		if err != nil {
			return nil, fmt.Errorf("failed to decode content: %w", err)
		}
		file.Content = string(decoded)
	}

	return file, nil
}

// ListWorkflowRuns returns recent workflow runs for a repo
func (c *Connector) ListWorkflowRuns(ctx context.Context, owner, repo string, limit int, statusFilter, branchFilter string) ([]WorkflowRun, error) {
	opts := &github.ListWorkflowRunsOptions{
		ListOptions: github.ListOptions{PerPage: min(limit, 100)},
	}

	if statusFilter != "" {
		opts.Status = statusFilter
	}
	if branchFilter != "" {
		opts.Branch = branchFilter
	}

	runs, _, err := c.client.Actions.ListRepositoryWorkflowRuns(ctx, owner, repo, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list workflow runs: %w", err)
	}

	var result []WorkflowRun
	for _, r := range runs.WorkflowRuns {
		if len(result) >= limit {
			break
		}

		run := WorkflowRun{
			ID:           r.GetID(),
			Name:         r.GetName(),
			WorkflowName: r.GetName(),
			Status:       r.GetStatus(),
			Conclusion:   r.GetConclusion(),
			Branch:       r.GetHeadBranch(),
			CommitSHA:    r.GetHeadSHA(),
			RunAttempt:   r.GetRunAttempt(),
			URL:          r.GetHTMLURL(),
		}

		if r.RunStartedAt != nil {
			run.RunStartedAt = r.RunStartedAt.Time
		}

		result = append(result, run)
	}

	return result, nil
}

// GetWorkflowRunLogs fetches logs for a specific workflow run
func (c *Connector) GetWorkflowRunLogs(ctx context.Context, owner, repo string, runID int64) (string, error) {
	url, _, err := c.client.Actions.GetWorkflowRunLogs(ctx, owner, repo, runID, 10)
	if err != nil {
		return "", fmt.Errorf("failed to get workflow run logs URL: %w", err)
	}

	// Fetch the log content from the signed URL
	resp, err := c.client.Client().Get(url.String())
	if err != nil {
		return "", fmt.Errorf("failed to download logs: %w", err)
	}
	defer resp.Body.Close()

	// Read the response body
	// Note: GitHub returns a ZIP file, but for now we'll handle plain text
	// In production, you might want to unzip and concatenate log files
	buf := make([]byte, 10*1024*1024) // 10MB max
	n, _ := resp.Body.Read(buf)

	return string(buf[:n]), nil
}

// shouldExclude checks if a path should be excluded
func shouldExclude(path string, excludePaths []string) bool {
	for _, exclude := range excludePaths {
		// Handle directory exclusion (e.g., "vendor/")
		if strings.HasSuffix(exclude, "/") {
			if strings.HasPrefix(path, exclude) || strings.Contains(path, "/"+exclude) {
				return true
			}
		} else if strings.Contains(path, exclude) {
			return true
		}
	}
	return false
}

// isBinaryExtension checks if a file is likely binary based on extension
func isBinaryExtension(path string) bool {
	binaryExts := map[string]bool{
		".exe": true, ".dll": true, ".so": true, ".dylib": true,
		".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".ico": true, ".svg": true,
		".pdf": true, ".doc": true, ".docx": true, ".xls": true, ".xlsx": true,
		".zip": true, ".tar": true, ".gz": true, ".rar": true, ".7z": true,
		".woff": true, ".woff2": true, ".ttf": true, ".eot": true,
		".mp3": true, ".mp4": true, ".wav": true, ".avi": true, ".mov": true,
		".pyc": true, ".pyo": true, ".class": true, ".o": true, ".a": true,
		".lock": true, // package locks are often large and not useful
	}
	ext := strings.ToLower(filepath.Ext(path))
	return binaryExts[ext]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
