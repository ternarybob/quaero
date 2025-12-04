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

// GetWorkerType returns the job type this worker handles
func (w *GitHubGitWorker) GetWorkerType() string {
	return models.JobTypeGitHubGitFile
}

// Validate validates that the queue job is compatible with this worker
func (w *GitHubGitWorker) Validate(job *models.QueueJob) error {
	if job.Type != models.JobTypeGitHubGitFile {
		return fmt.Errorf("invalid job type: expected %s, got %s", models.JobTypeGitHubGitFile, job.Type)
	}

	requiredFields := []string{"owner", "repo", "branch", "path"}
	for _, field := range requiredFields {
		if _, ok := job.GetConfigString(field); !ok {
			return fmt.Errorf("missing required config field: %s", field)
		}
	}
	return nil
}

// Execute processes a GitHub git file job (reads from cloned repo)
func (w *GitHubGitWorker) Execute(ctx context.Context, job *models.QueueJob) error {
	w.logger.Debug().Str("job_id", job.ID).Msg("Processing GitHub git file job")

	// Update job status to running
	if err := w.jobManager.UpdateJobStatus(ctx, job.ID, string(models.JobStatusRunning)); err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}

	// Extract configuration from job
	owner, _ := job.GetConfigString("owner")
	repo, _ := job.GetConfigString("repo")
	branch, _ := job.GetConfigString("branch")
	path, _ := job.GetConfigString("path")
	folder, _ := job.GetConfigString("folder")
	cloneDir, _ := job.GetConfigString("clone_dir")

	// Get tags from metadata
	baseTags := getTagsFromMetadata(job.Metadata)

	// Read file content from cloned repo
	filePath := filepath.Join(cloneDir, path)
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", path, err)
	}

	// Create document
	doc := &models.Document{
		ID:              fmt.Sprintf("doc_%s", uuid.New().String()),
		SourceType:      models.SourceTypeGitHubGit,
		SourceID:        fmt.Sprintf("%s/%s/%s/%s", owner, repo, branch, path),
		Title:           filepath.Base(path),
		ContentMarkdown: string(content),
		URL:             fmt.Sprintf("https://github.com/%s/%s/blob/%s/%s", owner, repo, branch, path),
		Tags:            mergeTags(baseTags, []string{"github", repo, branch}),
		Metadata: map[string]interface{}{
			"owner":       owner,
			"repo":        repo,
			"branch":      branch,
			"folder":      folder,
			"path":        path,
			"file_type":   filepath.Ext(path),
			"clone_based": true,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Save document
	if err := w.documentStorage.SaveDocument(doc); err != nil {
		return fmt.Errorf("failed to save document: %w", err)
	}

	// Log document saved
	w.logDocumentSaved(ctx, job, doc.ID, doc.Title, path)

	// Update job status to completed
	if err := w.jobManager.UpdateJobStatus(ctx, job.ID, string(models.JobStatusCompleted)); err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}

	// Update progress (completed=1, failed=0)
	if err := w.jobManager.UpdateJobProgress(ctx, job.ID, 1, 0); err != nil {
		w.logger.Warn().Err(err).Msg("Failed to update job progress")
	}

	w.logger.Debug().
		Str("job_id", job.ID).
		Str("path", path).
		Msg("GitHub git file job completed successfully")

	return nil
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

	// Suppress output to avoid leaking token
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to clone repository: %w (ensure git is installed and token has access)", err)
	}

	// Build extension map for quick lookup
	extMap := make(map[string]bool)
	for _, ext := range extensions {
		extMap[strings.ToLower(ext)] = true
	}

	// Walk the directory and collect files
	var files []struct {
		Path   string
		Folder string
	}

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

		// Get relative path
		relPath, err := filepath.Rel(cloneDir, path)
		if err != nil {
			return nil
		}

		// Skip .git directory
		if strings.HasPrefix(relPath, ".git") {
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
			return nil
		}

		// Check extension filter
		ext := strings.ToLower(filepath.Ext(relPath))
		if len(extensions) > 0 && !extMap[ext] {
			return nil
		}

		// Skip binary files by extension
		if isBinaryExtensionGit(relPath) {
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

	// Create parent job
	parentJob := models.NewQueueJob(
		"github_git_parent",
		fmt.Sprintf("GitHub Git: %s/%s", owner, repo),
		map[string]interface{}{
			"owner":     owner,
			"repo":      repo,
			"branch":    branch,
			"clone_dir": cloneDir,
		},
		map[string]interface{}{
			"connector_id":      connectorID,
			"job_definition_id": jobDef.ID,
			"tags":              jobDef.Tags,
		},
	)
	parentJob.ParentID = &stepID

	// Serialize parent job to JSON
	parentPayloadBytes, err := parentJob.ToJSON()
	if err != nil {
		os.RemoveAll(cloneDir)
		return "", fmt.Errorf("failed to serialize parent job: %w", err)
	}

	// Create parent job record in database
	if err := w.jobManager.CreateJobRecord(ctx, &queue.Job{
		ID:              parentJob.ID,
		ParentID:        parentJob.ParentID,
		Type:            parentJob.Type,
		Name:            parentJob.Name,
		Phase:           "execution",
		Status:          "pending",
		CreatedAt:       parentJob.CreatedAt,
		ProgressCurrent: 0,
		ProgressTotal:   0,
		Payload:         string(parentPayloadBytes),
	}); err != nil {
		os.RemoveAll(cloneDir)
		return "", fmt.Errorf("failed to create parent job record: %w", err)
	}

	// Enqueue child jobs for files
	totalFilesEnqueued := 0

	for _, file := range files {
		if totalFilesEnqueued >= maxFiles {
			w.logger.Warn().
				Int("max_files", maxFiles).
				Msg("Reached max_files limit, stopping file enumeration")
			break
		}

		// Create child job for each file
		childJob := models.NewQueueJobChild(
			parentJob.ID,
			models.JobTypeGitHubGitFile,
			fmt.Sprintf("Git: %s@%s:%s", repo, branch, file.Path),
			map[string]interface{}{
				"owner":     owner,
				"repo":      repo,
				"branch":    branch,
				"path":      file.Path,
				"folder":    file.Folder,
				"clone_dir": cloneDir,
			},
			map[string]interface{}{
				"connector_id":   connectorID,
				"tags":           jobDef.Tags,
				"root_parent_id": stepID,
			},
			parentJob.Depth+1,
		)

		// Serialize child job to JSON
		childPayloadBytes, err := childJob.ToJSON()
		if err != nil {
			w.logger.Warn().Err(err).
				Str("file", file.Path).
				Msg("Failed to serialize child job, skipping")
			continue
		}

		// Create child job record in database
		if err := w.jobManager.CreateJobRecord(ctx, &queue.Job{
			ID:              childJob.ID,
			ParentID:        childJob.ParentID,
			Type:            childJob.Type,
			Name:            childJob.Name,
			Phase:           "execution",
			Status:          "pending",
			CreatedAt:       childJob.CreatedAt,
			ProgressCurrent: 0,
			ProgressTotal:   1,
			Payload:         string(childPayloadBytes),
		}); err != nil {
			w.logger.Warn().Err(err).
				Str("file", file.Path).
				Msg("Failed to create child job record, skipping")
			continue
		}

		// Create queue message and enqueue
		queueMsg := models.QueueMessage{
			JobID:   childJob.ID,
			Type:    childJob.Type,
			Payload: childPayloadBytes,
		}

		if err := w.queueMgr.Enqueue(ctx, queueMsg); err != nil {
			w.logger.Warn().Err(err).
				Str("file", file.Path).
				Msg("Failed to enqueue file job, skipping")
			continue
		}

		totalFilesEnqueued++
	}

	w.logger.Info().
		Str("parent_job_id", parentJob.ID).
		Str("owner", owner).
		Str("repo", repo).
		Str("branch", branch).
		Int("files_enqueued", totalFilesEnqueued).
		Str("clone_dir", cloneDir).
		Msg("GitHub git clone completed, child jobs enqueued")

	// Update parent job with total count
	if err := w.jobManager.UpdateJobProgress(ctx, parentJob.ID, 0, totalFilesEnqueued); err != nil {
		w.logger.Warn().Err(err).Msg("Failed to update parent job progress")
	}

	// Note: Clone directory cleanup should happen after all child jobs complete
	// This requires a cleanup mechanism (e.g., post-job hook or scheduled cleanup)

	return parentJob.ID, nil
}

// ReturnsChildJobs returns true since GitHub git creates child jobs for each file
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
