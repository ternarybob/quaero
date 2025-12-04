// -----------------------------------------------------------------------
// GitHub Git Worker - Clone repository via git instead of API
// Faster for bulk file downloads, uses git command (requires git installed)
// -----------------------------------------------------------------------

package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
)

// GitHubGitWorker handles GitHub repository cloning via git command
// Implements both DefinitionWorker and JobWorker interfaces
type GitHubGitWorker struct {
	connectorService interfaces.ConnectorService
	jobManager       *queue.Manager
	queueMgr         interfaces.QueueManager
	documentStorage  interfaces.DocumentStorage
	eventService     interfaces.EventService
	logger           arbor.ILogger
	tempDir          string // Base directory for temporary clones
}

// Compile-time assertions: GitHubGitWorker implements both interfaces
var _ interfaces.DefinitionWorker = (*GitHubGitWorker)(nil)
var _ interfaces.JobWorker = (*GitHubGitWorker)(nil)

// NewGitHubGitWorker creates a new GitHub git worker
func NewGitHubGitWorker(
	connectorService interfaces.ConnectorService,
	jobManager *queue.Manager,
	queueMgr interfaces.QueueManager,
	documentStorage interfaces.DocumentStorage,
	eventService interfaces.EventService,
	logger arbor.ILogger,
) *GitHubGitWorker {
	// Use system temp directory for clones
	tempDir := filepath.Join(os.TempDir(), "quaero-git-clones")
	os.MkdirAll(tempDir, 0755)

	return &GitHubGitWorker{
		connectorService: connectorService,
		jobManager:       jobManager,
		queueMgr:         queueMgr,
		documentStorage:  documentStorage,
		eventService:     eventService,
		logger:           logger,
		tempDir:          tempDir,
	}
}

// GetWorkerType returns the job type this worker handles (not used for inline processing)
func (w *GitHubGitWorker) GetWorkerType() string {
	return models.JobTypeGitHubGitFile
}

// Validate validates that the queue job is compatible with this worker
func (w *GitHubGitWorker) Validate(job *models.QueueJob) error {
	// This worker now processes files inline, so this is rarely called
	if job.Type != models.JobTypeGitHubGitFile {
		return fmt.Errorf("invalid job type: expected %s, got %s", models.JobTypeGitHubGitFile, job.Type)
	}
	return nil
}

// Execute is kept for compatibility but the main work is done inline in CreateJobs
func (w *GitHubGitWorker) Execute(ctx context.Context, job *models.QueueJob) error {
	// This method is no longer the primary path - files are processed inline in CreateJobs
	// Kept for interface compatibility
	return fmt.Errorf("github_git worker processes files inline - this method should not be called")
}

// ============================================================================
// DEFINITIONWORKER INTERFACE METHODS (for job definition step handling)
// ============================================================================

// GetType returns WorkerTypeGitHubGit for the DefinitionWorker interface
func (w *GitHubGitWorker) GetType() models.WorkerType {
	return models.WorkerTypeGitHubGit
}

// CreateJobs clones the repository using git command and creates child jobs for each file
func (w *GitHubGitWorker) CreateJobs(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string) (string, error) {
	stepConfig := step.Config
	if stepConfig == nil {
		stepConfig = make(map[string]interface{})
	}

	// Extract config
	connectorID := getStringConfig(stepConfig, "connector_id", "")
	connectorName := getStringConfig(stepConfig, "connector_name", "")
	owner := getStringConfig(stepConfig, "owner", "")
	repo := getStringConfig(stepConfig, "repo", "")
	triggerURL := getStringConfig(stepConfig, "trigger_url", "")

	// If owner/repo not provided, extract from trigger_url (e.g., from Chrome extension)
	if (owner == "" || repo == "") && triggerURL != "" {
		extractedOwner, extractedRepo, extractedBranch := w.parseGitHubURL(triggerURL)
		if owner == "" {
			owner = extractedOwner
		}
		if repo == "" {
			repo = extractedRepo
		}
		// Also use extracted branch if not explicitly configured
		if extractedBranch != "" && getStringConfig(stepConfig, "branch", "") == "" {
			stepConfig["branch"] = extractedBranch
		}
		w.logger.Debug().
			Str("trigger_url", triggerURL).
			Str("extracted_owner", extractedOwner).
			Str("extracted_repo", extractedRepo).
			Str("extracted_branch", extractedBranch).
			Msg("Extracted owner/repo from trigger URL")
	}

	if connectorID == "" && connectorName == "" {
		return "", fmt.Errorf("connector_id or connector_name is required")
	}
	if owner == "" {
		return "", fmt.Errorf("owner is required (provide in config or via trigger_url)")
	}
	if repo == "" {
		return "", fmt.Errorf("repo is required (provide in config or via trigger_url)")
	}

	// Extract optional config with defaults
	branch := getStringConfig(stepConfig, "branch", "main")
	extensions := getStringSliceConfig(stepConfig, "extensions", []string{".go", ".ts", ".tsx", ".js", ".jsx", ".md", ".yaml", ".yml", ".toml", ".json"})
	excludePaths := getStringSliceConfig(stepConfig, "exclude_paths", []string{"vendor/", "node_modules/", ".git/", "dist/", "build/"})
	maxFiles := getIntConfig(stepConfig, "max_files", 1000)

	// Get git executable path with OS-appropriate default
	defaultGitPath := "git" // Linux/macOS default (assumes git is in PATH)
	if runtime.GOOS == "windows" {
		defaultGitPath = "C:\\Program Files\\Git\\bin\\git.exe"
	}
	gitPath := getStringConfig(stepConfig, "git_path", defaultGitPath)

	w.logger.Debug().
		Str("step_name", step.Name).
		Str("connector_id", connectorID).
		Str("connector_name", connectorName).
		Str("owner", owner).
		Str("repo", repo).
		Str("branch", branch).
		Str("git_path", gitPath).
		Int("max_files", maxFiles).
		Msg("Creating GitHub git clone parent job")

	// Get connector for authentication
	var connector *models.Connector
	var err error
	if connectorID != "" {
		connector, err = w.connectorService.GetConnector(ctx, connectorID)
		if err != nil {
			return "", fmt.Errorf("failed to get connector by ID: %w", err)
		}
	} else {
		connector, err = w.connectorService.GetConnectorByName(ctx, connectorName)
		if err != nil {
			return "", fmt.Errorf("failed to get connector by name '%s': %w", connectorName, err)
		}
		connectorID = connector.ID
	}

	// Extract token from connector config (Config is json.RawMessage)
	var gitHubConfig models.GitHubConnectorConfig
	if err := json.Unmarshal(connector.Config, &gitHubConfig); err != nil {
		return "", fmt.Errorf("failed to parse GitHub connector config: %w", err)
	}
	if gitHubConfig.Token == "" {
		return "", fmt.Errorf("GitHub token not found in connector config")
	}
	token := gitHubConfig.Token

	// Create unique clone directory
	cloneDir := filepath.Join(w.tempDir, fmt.Sprintf("%s-%s-%d", owner, repo, time.Now().UnixNano()))

	// Clone the repository using git command
	// Format: https://oauth2:TOKEN@github.com/owner/repo.git
	repoURL := fmt.Sprintf("https://oauth2:%s@github.com/%s/%s.git", token, owner, repo)

	w.logger.Info().
		Str("owner", owner).
		Str("repo", repo).
		Str("branch", branch).
		Str("clone_dir", cloneDir).
		Msg("Cloning repository via git command")

	// Run git clone with depth 1 (shallow clone) for speed
	cmd := exec.CommandContext(ctx, gitPath, "clone",
		"--depth", "1",
		"--branch", branch,
		"--single-branch",
		repoURL,
		cloneDir,
	)

	// Capture stderr for error reporting (sanitize to avoid leaking token)
	var stderr strings.Builder
	cmd.Stdout = nil
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// Sanitize error output to remove token
		errOutput := stderr.String()
		errOutput = strings.ReplaceAll(errOutput, token, "[REDACTED]")
		// Also redact any oauth2:token patterns
		if idx := strings.Index(errOutput, "oauth2:"); idx != -1 {
			endIdx := strings.Index(errOutput[idx:], "@")
			if endIdx != -1 {
				errOutput = errOutput[:idx] + "oauth2:[REDACTED]" + errOutput[idx+endIdx:]
			}
		}
		w.logger.Error().
			Str("git_path", gitPath).
			Str("owner", owner).
			Str("repo", repo).
			Str("branch", branch).
			Str("error_output", errOutput).
			Msg("Git clone failed")
		return "", fmt.Errorf("failed to clone repository: %w - git output: %s", err, errOutput)
	}

	// Build extension map for quick lookup
	extMap := make(map[string]bool)
	for _, ext := range extensions {
		extMap[strings.ToLower(ext)] = true
	}

	// Walk the directory and collect files with detailed counting
	var files []struct {
		Path   string
		Folder string
	}

	// Counters for detailed logging
	var totalFilesScanned int
	var excludedByPath int
	var excludedByExtension int
	var excludedByBinary int

	err = filepath.Walk(cloneDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		if info.IsDir() {
			// Check if directory should be excluded
			relPath, _ := filepath.Rel(cloneDir, path)
			// Skip .git directory
			if relPath == ".git" || strings.HasPrefix(relPath, ".git/") {
				return filepath.SkipDir
			}
			for _, exclude := range excludePaths {
				if strings.Contains(relPath+"/", exclude) {
					return filepath.SkipDir
				}
			}
			return nil
		}

		// Count all files
		totalFilesScanned++

		// Get relative path
		relPath, err := filepath.Rel(cloneDir, path)
		if err != nil {
			return nil
		}

		// Skip .git directory
		if strings.HasPrefix(relPath, ".git") {
			excludedByPath++
			return nil
		}

		// Check exclude paths
		shouldExclude := false
		for _, exclude := range excludePaths {
			if strings.Contains(relPath, exclude) {
				shouldExclude = true
				break
			}
		}
		if shouldExclude {
			excludedByPath++
			return nil
		}

		// Check extension filter
		ext := strings.ToLower(filepath.Ext(relPath))
		if len(extensions) > 0 && !extMap[ext] {
			excludedByExtension++
			return nil
		}

		// Skip binary files by extension
		if isBinaryExtensionGit(relPath) {
			excludedByBinary++
			return nil
		}

		files = append(files, struct {
			Path   string
			Folder string
		}{
			Path:   relPath,
			Folder: filepath.Dir(relPath),
		})

		return nil
	})

	if err != nil {
		os.RemoveAll(cloneDir)
		return "", fmt.Errorf("failed to walk repository: %w", err)
	}

	// Calculate files to schedule
	matchedFiles := len(files)
	filesToSchedule := matchedFiles
	excludedByLimit := 0
	if filesToSchedule > maxFiles {
		excludedByLimit = filesToSchedule - maxFiles
		filesToSchedule = maxFiles
	}

	// Log detailed file counts (similar to git output style)
	w.logger.Info().
		Str("owner", owner).
		Str("repo", repo).
		Str("branch", branch).
		Int("total_files_in_repo", totalFilesScanned).
		Int("excluded_by_path", excludedByPath).
		Int("excluded_by_extension", excludedByExtension).
		Int("excluded_by_binary", excludedByBinary).
		Int("matched_files", matchedFiles).
		Int("excluded_by_limit", excludedByLimit).
		Int("files_to_download", filesToSchedule).
		Msg("Repository scanned - file analysis complete")

	// Add detailed step logs for UI visibility (git-style output)
	w.jobManager.AddJobLog(ctx, stepID, "info", fmt.Sprintf("Cloning into '%s/%s@%s'...", owner, repo, branch))
	w.jobManager.AddJobLog(ctx, stepID, "info", fmt.Sprintf("Enumerating files: %d total files in repository", totalFilesScanned))
	w.jobManager.AddJobLog(ctx, stepID, "info", fmt.Sprintf("Filtering files: %d excluded by path, %d excluded by extension, %d excluded by binary type",
		excludedByPath, excludedByExtension, excludedByBinary))
	w.jobManager.AddJobLog(ctx, stepID, "info", fmt.Sprintf("Matched files: %d (limit: %d)", matchedFiles, maxFiles))
	if excludedByLimit > 0 {
		w.jobManager.AddJobLog(ctx, stepID, "warn", fmt.Sprintf("Limit reached: %d files excluded by max_files limit", excludedByLimit))
	}
	w.jobManager.AddJobLog(ctx, stepID, "info", fmt.Sprintf("Processing %d files inline (no child jobs)", filesToSchedule))

	// Get tags for documents
	baseTags := jobDef.Tags
	if baseTags == nil {
		baseTags = []string{}
	}

	// Process files inline - no child jobs needed
	// This is much more efficient than creating 1000+ separate jobs
	processedCount := 0
	failedCount := 0
	startProcessing := time.Now()

	for i, file := range files {
		if processedCount >= maxFiles {
			w.logger.Warn().
				Int("max_files", maxFiles).
				Msg("Reached max_files limit, stopping file processing")
			break
		}

		fileStartTime := time.Now()

		// Read file content from cloned repo
		filePath := filepath.Join(cloneDir, file.Path)
		content, err := os.ReadFile(filePath)
		if err != nil {
			w.logger.Warn().Err(err).
				Str("file", file.Path).
				Msg("Failed to read file, skipping")
			failedCount++
			continue
		}

		// Create document
		doc := &models.Document{
			ID:              fmt.Sprintf("doc_%s", uuid.New().String()),
			SourceType:      models.SourceTypeGitHubGit,
			SourceID:        fmt.Sprintf("%s/%s/%s/%s", owner, repo, branch, file.Path),
			Title:           filepath.Base(file.Path),
			ContentMarkdown: string(content),
			URL:             fmt.Sprintf("https://github.com/%s/%s/blob/%s/%s", owner, repo, branch, file.Path),
			Tags:            mergeTags(baseTags, []string{"github", repo, branch}),
			Metadata: map[string]interface{}{
				"owner":       owner,
				"repo":        repo,
				"branch":      branch,
				"folder":      file.Folder,
				"path":        file.Path,
				"file_type":   filepath.Ext(file.Path),
				"clone_based": true,
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		// Save document
		if err := w.documentStorage.SaveDocument(doc); err != nil {
			w.logger.Warn().Err(err).
				Str("file", file.Path).
				Msg("Failed to save document, skipping")
			failedCount++
			continue
		}

		processedCount++
		fileElapsed := time.Since(fileStartTime)

		// Log every file processed
		w.logger.Debug().
			Str("file", file.Path).
			Str("doc_id", doc.ID[:12]).
			Dur("duration", fileElapsed).
			Msg("File processed")

		// Add job log for UI visibility (every 10 files or last file)
		if processedCount%10 == 0 || i == len(files)-1 || processedCount == filesToSchedule {
			w.jobManager.AddJobLog(ctx, stepID, "info",
				fmt.Sprintf("Progress: %d/%d files processed (%.1f%%)",
					processedCount, filesToSchedule, float64(processedCount)/float64(filesToSchedule)*100))
		}
	}

	totalElapsed := time.Since(startProcessing)

	// Clean up clone directory immediately since we're done with it
	if err := os.RemoveAll(cloneDir); err != nil {
		w.logger.Warn().Err(err).Str("clone_dir", cloneDir).Msg("Failed to clean up clone directory")
	} else {
		w.logger.Debug().Str("clone_dir", cloneDir).Msg("Clone directory cleaned up")
	}

	// Log completion
	w.logger.Info().
		Str("owner", owner).
		Str("repo", repo).
		Str("branch", branch).
		Int("files_processed", processedCount).
		Int("files_failed", failedCount).
		Int("total_files_scanned", totalFilesScanned).
		Int("files_matched", matchedFiles).
		Dur("processing_time", totalElapsed).
		Msg("GitHub git clone completed - all files processed inline")

	// Add final step logs for UI visibility
	w.jobManager.AddJobLog(ctx, stepID, "info", fmt.Sprintf("Completed: %d files imported, %d failed (time: %s)",
		processedCount, failedCount, totalElapsed.Round(time.Millisecond)))
	w.jobManager.AddJobLog(ctx, stepID, "info", fmt.Sprintf("Summary: scanned %d, matched %d, rejected %d",
		totalFilesScanned, matchedFiles, excludedByPath+excludedByExtension+excludedByBinary))

	// Return empty string since we don't create child jobs anymore
	// The step will be marked complete by the caller
	return "", nil
}

// ReturnsChildJobs returns false - files are now processed inline for efficiency
func (w *GitHubGitWorker) ReturnsChildJobs() bool {
	return false
}

// ValidateConfig validates step configuration for GitHub git type
func (w *GitHubGitWorker) ValidateConfig(step models.JobStep) error {
	if step.Config == nil {
		return fmt.Errorf("github_git step requires config")
	}

	// Validate connector_id or connector_name
	connectorID, hasConnectorID := step.Config["connector_id"].(string)
	connectorName, hasConnectorName := step.Config["connector_name"].(string)

	if (!hasConnectorID || connectorID == "") && (!hasConnectorName || connectorName == "") {
		return fmt.Errorf("github_git step requires either 'connector_id' or 'connector_name' in config")
	}

	// Note: owner and repo are optional - they can be extracted from trigger_url at runtime
	// when the job is triggered from the Chrome extension visiting a GitHub page

	return nil
}

// isBinaryExtensionGit checks if a file is likely binary based on extension
func isBinaryExtensionGit(path string) bool {
	binaryExts := map[string]bool{
		".exe": true, ".dll": true, ".so": true, ".dylib": true,
		".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".ico": true, ".svg": true,
		".pdf": true, ".doc": true, ".docx": true, ".xls": true, ".xlsx": true,
		".zip": true, ".tar": true, ".gz": true, ".rar": true, ".7z": true,
		".woff": true, ".woff2": true, ".ttf": true, ".eot": true,
		".mp3": true, ".mp4": true, ".wav": true, ".avi": true, ".mov": true,
		".pyc": true, ".pyo": true, ".class": true, ".o": true, ".a": true,
		".lock": true,
	}
	ext := strings.ToLower(filepath.Ext(path))
	return binaryExts[ext]
}

// logDocumentSaved logs a document saved event
func (w *GitHubGitWorker) logDocumentSaved(ctx context.Context, job *models.QueueJob, docID, title, path string) {
	message := fmt.Sprintf("Document saved: %s (ID: %s)", title, docID[:8])
	if path != "" {
		message = fmt.Sprintf("Document saved: %s - %s (ID: %s)", path, title, docID[:8])
	}
	w.jobManager.AddJobLog(ctx, job.ID, "info", message)
}

// parseGitHubURL extracts owner, repo, and optionally branch from a GitHub URL
// Supports various GitHub URL formats:
// - https://github.com/owner/repo
// - https://github.com/owner/repo/tree/branch
// - https://github.com/owner/repo/blob/branch/path
// - https://github.com/owner/repo/commit/sha
func (w *GitHubGitWorker) parseGitHubURL(githubURL string) (owner, repo, branch string) {
	parsed, err := url.Parse(githubURL)
	if err != nil {
		return "", "", ""
	}

	// Remove leading slash and split path
	path := strings.TrimPrefix(parsed.Path, "/")
	parts := strings.Split(path, "/")

	if len(parts) < 2 {
		return "", "", ""
	}

	owner = parts[0]
	repo = parts[1]

	// Extract branch if present in URL
	// Formats: /owner/repo/tree/branch, /owner/repo/blob/branch/file
	if len(parts) >= 4 {
		switch parts[2] {
		case "tree", "blob":
			branch = parts[3]
		case "commit":
			// For commit URLs, we could use the SHA but typically want default branch
			// Leave branch empty to use configured default
		}
	}

	return owner, repo, branch
}
