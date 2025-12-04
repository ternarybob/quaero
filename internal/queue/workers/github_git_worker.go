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

// GetWorkerType returns the job type this worker handles (batch jobs)
func (w *GitHubGitWorker) GetWorkerType() string {
	return models.JobTypeGitHubGitBatch
}

// Validate validates that the queue job is compatible with this worker
func (w *GitHubGitWorker) Validate(job *models.QueueJob) error {
	if job.Type != models.JobTypeGitHubGitBatch {
		return fmt.Errorf("invalid job type: expected %s, got %s", models.JobTypeGitHubGitBatch, job.Type)
	}
	return nil
}

// Execute processes a batch of files from a github_git_batch queue job.
// Each batch contains up to 1000 files to read from the cloned repository.
func (w *GitHubGitWorker) Execute(ctx context.Context, job *models.QueueJob) error {
	// Extract config from batch job
	owner, _ := job.Config["owner"].(string)
	repo, _ := job.Config["repo"].(string)
	branch, _ := job.Config["branch"].(string)
	cloneDir, _ := job.Config["clone_dir"].(string)
	batchIdx, _ := job.Config["batch_idx"].(float64) // JSON numbers are float64
	filesRaw, _ := job.Config["files"].([]interface{})
	tagsRaw, _ := job.Config["tags"].([]interface{})

	// Convert tags to string slice
	var tags []string
	for _, t := range tagsRaw {
		if s, ok := t.(string); ok {
			tags = append(tags, s)
		}
	}

	// Validate required fields
	if cloneDir == "" {
		return fmt.Errorf("clone_dir is required in batch job config")
	}
	if len(filesRaw) == 0 {
		w.logger.Warn().
			Str("job_id", job.ID).
			Msg("Batch job has no files to process")
		return nil
	}

	w.logger.Info().
		Str("job_id", job.ID).
		Str("owner", owner).
		Str("repo", repo).
		Int("batch_idx", int(batchIdx)).
		Int("files_in_batch", len(filesRaw)).
		Msg("Processing batch job")

	// Process each file in the batch
	var savedCount, errorCount int

	for _, fileRaw := range filesRaw {
		fileMap, ok := fileRaw.(map[string]interface{})
		if !ok {
			errorCount++
			continue
		}

		filePath, _ := fileMap["path"].(string)
		folder, _ := fileMap["folder"].(string)
		if filePath == "" {
			errorCount++
			continue
		}

		// Read file content from cloned repository
		fullPath := filepath.Join(cloneDir, filePath)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			w.logger.Debug().
				Err(err).
				Str("path", filePath).
				Msg("Failed to read file")
			errorCount++
			continue
		}

		// Generate document ID
		docID := uuid.New().String()

		// Build document title from path
		title := filepath.Base(filePath)

		// Build source URL for GitHub
		sourceURL := fmt.Sprintf("https://github.com/%s/%s/blob/%s/%s", owner, repo, branch, filePath)

		// Create document metadata
		metadata := map[string]interface{}{
			"owner":      owner,
			"repo":       repo,
			"branch":     branch,
			"path":       filePath,
			"folder":     folder,
			"source_url": sourceURL,
			"type":       "github_file",
		}

		// Create document
		doc := &models.Document{
			ID:              docID,
			SourceType:      "github_git",
			SourceID:        filePath,
			Title:           title,
			ContentMarkdown: string(content),
			URL:             sourceURL,
			Tags:            tags,
			Metadata:        metadata,
		}

		// Save document
		if err := w.documentStorage.SaveDocument(doc); err != nil {
			w.logger.Debug().
				Err(err).
				Str("path", filePath).
				Msg("Failed to save document")
			errorCount++
			continue
		}

		savedCount++

		// Log progress periodically (every 100 files)
		if savedCount%100 == 0 {
			w.logger.Debug().
				Int("saved", savedCount).
				Int("errors", errorCount).
				Int("total", len(filesRaw)).
				Msg("Batch progress")
		}
	}

	// Log final batch results
	w.jobManager.AddJobLog(ctx, job.ID, "info", fmt.Sprintf("Batch complete: %d saved, %d errors", savedCount, errorCount))

	w.logger.Info().
		Str("job_id", job.ID).
		Int("batch_idx", int(batchIdx)).
		Int("saved", savedCount).
		Int("errors", errorCount).
		Msg("Batch job completed")

	return nil
}

// ============================================================================
// DEFINITIONWORKER INTERFACE METHODS (for job definition step handling)
// ============================================================================

// GetType returns WorkerTypeGitHubGit for the DefinitionWorker interface
func (w *GitHubGitWorker) GetType() models.WorkerType {
	return models.WorkerTypeGitHubGit
}

// Init performs the initialization/setup phase for a GitHub git step.
// This is where we:
//   - Validate configuration and extract repo details
//   - Clone the repository (shallow clone for speed)
//   - Walk the directory and identify files to process
//
// The Init phase creates a temporary clone and returns file list.
// The cloneDir is stored in metadata for CreateJobs to use.
func (w *GitHubGitWorker) Init(ctx context.Context, step models.JobStep, jobDef models.JobDefinition) (*interfaces.WorkerInitResult, error) {
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

	// Validate required config
	if connectorID == "" && connectorName == "" {
		return nil, fmt.Errorf("connector_id or connector_name is required")
	}
	if owner == "" {
		return nil, fmt.Errorf("owner is required (provide in config or via trigger_url)")
	}
	if repo == "" {
		return nil, fmt.Errorf("repo is required (provide in config or via trigger_url)")
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

	w.logger.Info().
		Str("step_name", step.Name).
		Str("owner", owner).
		Str("repo", repo).
		Str("branch", branch).
		Int("max_files", maxFiles).
		Msg("Initializing GitHub git worker - assessing repository")

	// Get connector for authentication
	var connector *models.Connector
	var err error
	if connectorID != "" {
		connector, err = w.connectorService.GetConnector(ctx, connectorID)
		if err != nil {
			return nil, fmt.Errorf("failed to get connector by ID: %w", err)
		}
	} else {
		connector, err = w.connectorService.GetConnectorByName(ctx, connectorName)
		if err != nil {
			return nil, fmt.Errorf("failed to get connector by name '%s': %w", connectorName, err)
		}
		connectorID = connector.ID
	}

	// Extract token from connector config (Config is json.RawMessage)
	var gitHubConfig models.GitHubConnectorConfig
	if err := json.Unmarshal(connector.Config, &gitHubConfig); err != nil {
		return nil, fmt.Errorf("failed to parse GitHub connector config: %w", err)
	}
	if gitHubConfig.Token == "" {
		return nil, fmt.Errorf("GitHub token not found in connector config")
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
		Msg("Cloning repository to assess content")

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
		return nil, fmt.Errorf("failed to clone repository: %w - git output: %s", err, errOutput)
	}

	// Build extension map for quick lookup
	extMap := make(map[string]bool)
	for _, ext := range extensions {
		extMap[strings.ToLower(ext)] = true
	}

	// Walk the directory and collect files with detailed counting
	type fileInfo struct {
		Path   string
		Folder string
	}
	var files []fileInfo

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

		files = append(files, fileInfo{
			Path:   relPath,
			Folder: filepath.Dir(relPath),
		})

		return nil
	})

	if err != nil {
		os.RemoveAll(cloneDir)
		return nil, fmt.Errorf("failed to walk repository: %w", err)
	}

	// All matched files will be processed (no limit in Init - batching happens in CreateJobs)
	matchedFiles := len(files)

	w.logger.Info().
		Str("owner", owner).
		Str("repo", repo).
		Str("branch", branch).
		Int("total_files_in_repo", totalFilesScanned).
		Int("excluded_by_path", excludedByPath).
		Int("excluded_by_extension", excludedByExtension).
		Int("excluded_by_binary", excludedByBinary).
		Int("matched_files", matchedFiles).
		Msg("Repository assessed - file analysis complete")

	// Create work items from ALL files (batching happens in CreateJobs)
	workItems := make([]interfaces.WorkItem, matchedFiles)
	for i, file := range files {
		workItems[i] = interfaces.WorkItem{
			ID:   file.Path,
			Name: filepath.Base(file.Path),
			Type: "file",
			Config: map[string]interface{}{
				"path":   file.Path,
				"folder": file.Folder,
			},
		}
	}

	// Calculate batch count for logging
	batchSize := 1000
	batchCount := (matchedFiles + batchSize - 1) / batchSize

	return &interfaces.WorkerInitResult{
		WorkItems:            workItems,
		TotalCount:           matchedFiles,
		Strategy:             interfaces.ProcessingStrategyParallel, // Batched queue jobs
		SuggestedConcurrency: batchCount,                            // One job per batch
		Metadata: map[string]interface{}{
			"owner":                 owner,
			"repo":                  repo,
			"branch":                branch,
			"connector_id":          connectorID,
			"clone_dir":             cloneDir,
			"total_files_scanned":   totalFilesScanned,
			"excluded_by_path":      excludedByPath,
			"excluded_by_extension": excludedByExtension,
			"excluded_by_binary":    excludedByBinary,
			"matched_files":         matchedFiles,
			"batch_size":            batchSize,
			"batch_count":           batchCount,
		},
	}, nil
}

// CreateJobs creates batched queue jobs for processing files from the repository.
// Each batch contains up to 1000 files and is processed as a separate queue job.
func (w *GitHubGitWorker) CreateJobs(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string, initResult *interfaces.WorkerInitResult) (string, error) {
	// Call Init if not provided
	if initResult == nil {
		var err error
		initResult, err = w.Init(ctx, step, jobDef)
		if err != nil {
			return "", fmt.Errorf("failed to initialize github_git worker: %w", err)
		}
	}

	// Extract metadata from init result
	owner, _ := initResult.Metadata["owner"].(string)
	repo, _ := initResult.Metadata["repo"].(string)
	branch, _ := initResult.Metadata["branch"].(string)
	cloneDir, _ := initResult.Metadata["clone_dir"].(string)
	totalFilesScanned, _ := initResult.Metadata["total_files_scanned"].(int)
	excludedByPath, _ := initResult.Metadata["excluded_by_path"].(int)
	excludedByExtension, _ := initResult.Metadata["excluded_by_extension"].(int)
	excludedByBinary, _ := initResult.Metadata["excluded_by_binary"].(int)
	matchedFiles, _ := initResult.Metadata["matched_files"].(int)
	batchSize, _ := initResult.Metadata["batch_size"].(int)
	if batchSize == 0 {
		batchSize = 1000
	}

	workItems := initResult.WorkItems
	totalFiles := len(workItems)

	// Calculate batch count
	batchCount := (totalFiles + batchSize - 1) / batchSize

	w.logger.Info().
		Str("step_name", step.Name).
		Str("owner", owner).
		Str("repo", repo).
		Str("branch", branch).
		Int("total_files", totalFiles).
		Int("batch_size", batchSize).
		Int("batch_count", batchCount).
		Msg("Creating batched queue jobs")

	// Add step logs for UI visibility
	w.jobManager.AddJobLog(ctx, stepID, "info", fmt.Sprintf("Cloned '%s/%s@%s'", owner, repo, branch))
	w.jobManager.AddJobLog(ctx, stepID, "info", fmt.Sprintf("Scanned %d files: %d excluded by path, %d by extension, %d by binary",
		totalFilesScanned, excludedByPath, excludedByExtension, excludedByBinary))
	w.jobManager.AddJobLog(ctx, stepID, "info", fmt.Sprintf("Matched %d files, creating %d batch jobs (%d files per batch)",
		matchedFiles, batchCount, batchSize))

	// Get tags for documents
	baseTags := jobDef.Tags
	if baseTags == nil {
		baseTags = []string{}
	}

	// Create batched queue jobs
	jobIDs := make([]string, 0, batchCount)

	for batchIdx := 0; batchIdx < batchCount; batchIdx++ {
		startIdx := batchIdx * batchSize
		endIdx := startIdx + batchSize
		if endIdx > totalFiles {
			endIdx = totalFiles
		}

		batchWorkItems := workItems[startIdx:endIdx]

		// Convert WorkItems to file paths for the batch config
		filePaths := make([]map[string]interface{}, len(batchWorkItems))
		for i, item := range batchWorkItems {
			filePaths[i] = map[string]interface{}{
				"path":   item.Config["path"],
				"folder": item.Config["folder"],
			}
		}

		// Create batch job
		batchJob := models.NewQueueJob(
			models.JobTypeGitHubGitBatch,
			fmt.Sprintf("Batch %d/%d: %s/%s", batchIdx+1, batchCount, owner, repo),
			map[string]interface{}{
				"owner":      owner,
				"repo":       repo,
				"branch":     branch,
				"clone_dir":  cloneDir,
				"batch_idx":  batchIdx,
				"batch_size": len(batchWorkItems),
				"files":      filePaths,
				"tags":       baseTags,
			},
			map[string]interface{}{
				"job_definition_id": jobDef.ID,
				"step_name":         step.Name,
			},
		)
		batchJob.ParentID = &stepID

		// Serialize and enqueue
		payloadBytes, err := batchJob.ToJSON()
		if err != nil {
			w.logger.Error().Err(err).
				Int("batch_idx", batchIdx).
				Msg("Failed to serialize batch job")
			continue
		}

		// Create job record
		if err := w.jobManager.CreateJobRecord(ctx, &queue.Job{
			ID:              batchJob.ID,
			ParentID:        batchJob.ParentID,
			Type:            batchJob.Type,
			Name:            batchJob.Name,
			Phase:           "execution",
			Status:          "pending",
			CreatedAt:       batchJob.CreatedAt,
			ProgressCurrent: 0,
			ProgressTotal:   len(batchWorkItems),
			Payload:         string(payloadBytes),
		}); err != nil {
			w.logger.Error().Err(err).
				Int("batch_idx", batchIdx).
				Msg("Failed to create batch job record")
			continue
		}

		// Enqueue to queue manager
		msg := models.QueueMessage{
			JobID:   batchJob.ID,
			Type:    batchJob.Type,
			Payload: payloadBytes,
		}
		if err := w.queueMgr.Enqueue(ctx, msg); err != nil {
			w.logger.Error().Err(err).
				Int("batch_idx", batchIdx).
				Msg("Failed to enqueue batch job")
			continue
		}

		jobIDs = append(jobIDs, batchJob.ID)

		w.logger.Debug().
			Str("job_id", batchJob.ID).
			Int("batch_idx", batchIdx).
			Int("files_in_batch", len(batchWorkItems)).
			Msg("Batch job created and enqueued")
	}

	if len(jobIDs) == 0 {
		return "", fmt.Errorf("failed to create any batch jobs")
	}

	w.jobManager.AddJobLog(ctx, stepID, "info", fmt.Sprintf("Created %d batch jobs for %d files", len(jobIDs), totalFiles))

	w.logger.Info().
		Str("step_name", step.Name).
		Int("batches_created", len(jobIDs)).
		Int("total_files", totalFiles).
		Msg("Batch jobs created and enqueued")

	// Return the step ID - orchestrator will monitor child job completion
	return stepID, nil
}

// ReturnsChildJobs returns true - we create batched child jobs for queue processing
func (w *GitHubGitWorker) ReturnsChildJobs() bool {
	return true
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
