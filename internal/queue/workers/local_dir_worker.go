// -----------------------------------------------------------------------
// Local Directory Worker - Index local filesystem directories
// Scans a directory, indexes files as documents for AI processing
// -----------------------------------------------------------------------

package workers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
)

// LocalDirWorker handles local directory indexing
// Implements both DefinitionWorker and JobWorker interfaces
type LocalDirWorker struct {
	jobManager      *queue.Manager
	queueMgr        interfaces.QueueManager
	documentStorage interfaces.DocumentStorage
	eventService    interfaces.EventService
	logger          arbor.ILogger
}

// Compile-time assertions: LocalDirWorker implements both interfaces
var _ interfaces.DefinitionWorker = (*LocalDirWorker)(nil)
var _ interfaces.JobWorker = (*LocalDirWorker)(nil)

// NewLocalDirWorker creates a new local directory worker
func NewLocalDirWorker(
	jobManager *queue.Manager,
	queueMgr interfaces.QueueManager,
	documentStorage interfaces.DocumentStorage,
	eventService interfaces.EventService,
	logger arbor.ILogger,
) *LocalDirWorker {
	return &LocalDirWorker{
		jobManager:      jobManager,
		queueMgr:        queueMgr,
		documentStorage: documentStorage,
		eventService:    eventService,
		logger:          logger,
	}
}

// GetWorkerType returns the job type this worker handles (batch jobs)
func (w *LocalDirWorker) GetWorkerType() string {
	return models.JobTypeLocalDirBatch
}

// Validate validates that the queue job is compatible with this worker
func (w *LocalDirWorker) Validate(job *models.QueueJob) error {
	if job.Type != models.JobTypeLocalDirBatch {
		return fmt.Errorf("invalid job type: expected %s, got %s", models.JobTypeLocalDirBatch, job.Type)
	}
	return nil
}

// Execute processes a batch of files from a local_dir_batch queue job.
// Each batch contains up to 1000 files to read and store as documents.
func (w *LocalDirWorker) Execute(ctx context.Context, job *models.QueueJob) error {
	// Extract config from batch job
	basePath, _ := job.Config["base_path"].(string)
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
	if basePath == "" {
		return fmt.Errorf("base_path is required in batch job config")
	}
	if len(filesRaw) == 0 {
		w.logger.Warn().
			Str("job_id", job.ID).
			Msg("[run] Batch job has no files to process")
		return nil
	}

	w.logger.Info().
		Str("job_id", job.ID).
		Str("base_path", basePath).
		Int("batch_idx", int(batchIdx)).
		Int("files_in_batch", len(filesRaw)).
		Msg("[run] Processing local directory batch job")

	// Process each file in the batch
	var savedCount, errorCount int

	for _, fileRaw := range filesRaw {
		fileMap, ok := fileRaw.(map[string]interface{})
		if !ok {
			errorCount++
			continue
		}

		relPath, _ := fileMap["path"].(string)
		folder, _ := fileMap["folder"].(string)
		absPath, _ := fileMap["absolute_path"].(string)
		extension, _ := fileMap["extension"].(string)
		fileSize, _ := fileMap["file_size"].(float64)
		fileType, _ := fileMap["file_type"].(string)

		if relPath == "" || absPath == "" {
			errorCount++
			continue
		}

		// Read file content
		content, err := os.ReadFile(absPath)
		if err != nil {
			w.logger.Debug().
				Err(err).
				Str("path", absPath).
				Msg("[run] Failed to read file")
			errorCount++
			continue
		}

		// Generate document ID
		docID := uuid.New().String()

		// Build document title from path
		title := filepath.Base(relPath)

		// Create document metadata
		metadata := map[string]interface{}{
			"base_path":     basePath,
			"file_path":     relPath,
			"absolute_path": absPath,
			"folder":        folder,
			"extension":     extension,
			"file_size":     int64(fileSize),
			"file_type":     fileType,
			"type":          "local_file",
		}

		// Create document
		doc := &models.Document{
			ID:              docID,
			SourceType:      models.SourceTypeLocalDir,
			SourceID:        relPath,
			Title:           title,
			ContentMarkdown: string(content),
			URL:             "file://" + absPath,
			Tags:            tags,
			Metadata:        metadata,
		}

		// Save document
		if err := w.documentStorage.SaveDocument(doc); err != nil {
			w.logger.Debug().
				Err(err).
				Str("path", relPath).
				Msg("[run] Failed to save document")
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
				Msg("[run] Batch progress")
		}
	}

	// Log final batch results
	w.jobManager.AddJobLogWithPhase(ctx, job.ID, "info", fmt.Sprintf("Batch complete: %d saved, %d errors", savedCount, errorCount), "", "run")

	w.logger.Info().
		Str("job_id", job.ID).
		Int("batch_idx", int(batchIdx)).
		Int("saved", savedCount).
		Int("errors", errorCount).
		Msg("[run] Local directory batch job completed")

	return nil
}

// ============================================================================
// DEFINITIONWORKER INTERFACE METHODS (for job definition step handling)
// ============================================================================

// GetType returns WorkerTypeLocalDir for the DefinitionWorker interface
func (w *LocalDirWorker) GetType() models.WorkerType {
	return models.WorkerTypeLocalDir
}

// Init performs the initialization/setup phase for a local directory step.
// This is where we:
//   - Validate configuration and extract directory path
//   - Walk the directory and identify files to process
//   - Return file list as work items
func (w *LocalDirWorker) Init(ctx context.Context, step models.JobStep, jobDef models.JobDefinition) (*interfaces.WorkerInitResult, error) {
	stepConfig := step.Config
	if stepConfig == nil {
		stepConfig = make(map[string]interface{})
	}

	// Extract config
	dirPath := getStringConfig(stepConfig, "dir_path", "")
	if dirPath == "" {
		dirPath = getStringConfig(stepConfig, "path", "")
	}

	// Validate required config
	if dirPath == "" {
		return nil, fmt.Errorf("dir_path is required")
	}

	// Resolve to absolute path
	absPath, err := filepath.Abs(dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	// Check that directory exists
	info, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("directory does not exist: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("path is not a directory: %s", absPath)
	}

	// Extract optional config with defaults
	extensions := getStringSliceConfig(stepConfig, "extensions", []string{
		".go", ".ts", ".tsx", ".js", ".jsx", ".py", ".rs", ".java",
		".md", ".yaml", ".yml", ".toml", ".json", ".xml",
		".html", ".css", ".scss", ".less",
		".sh", ".bash", ".zsh",
		".sql", ".graphql",
		".dockerfile", ".env.example",
	})
	excludePaths := getStringSliceConfig(stepConfig, "exclude_paths", []string{
		"vendor/", "node_modules/", ".git/", "dist/", "build/",
		"__pycache__/", ".venv/", "venv/", ".tox/",
		"target/", ".gradle/", ".mvn/",
		".idea/", ".vscode/", ".vs/",
	})
	maxFiles := getIntConfig(stepConfig, "max_files", 10000)
	maxFileSize := getIntConfig(stepConfig, "max_file_size", 1024*1024) // 1MB default

	w.logger.Info().
		Str("step_name", step.Name).
		Str("dir_path", absPath).
		Int("max_files", maxFiles).
		Msg("[init] Initializing local directory worker - scanning directory")

	// Build extension map for quick lookup
	extMap := make(map[string]bool)
	for _, ext := range extensions {
		extMap[strings.ToLower(ext)] = true
	}

	// Walk the directory and collect files
	type fileInfo struct {
		Path      string
		AbsPath   string
		Folder    string
		Extension string
		FileSize  int64
		ModTime   time.Time
		FileType  string
	}
	var files []fileInfo

	// Counters for detailed logging
	var totalFilesScanned int
	var excludedByPath int
	var excludedByExtension int
	var excludedBySize int
	var excludedByBinary int

	err = filepath.Walk(absPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		if info.IsDir() {
			// Get relative path for directory
			relPath, _ := filepath.Rel(absPath, path)
			// Skip .git directory
			if relPath == ".git" || strings.HasPrefix(relPath, ".git"+string(os.PathSeparator)) {
				return filepath.SkipDir
			}
			for _, exclude := range excludePaths {
				excludeClean := strings.TrimSuffix(exclude, "/")
				if relPath == excludeClean || strings.HasPrefix(relPath, excludeClean+string(os.PathSeparator)) {
					return filepath.SkipDir
				}
			}
			return nil
		}

		// Count all files
		totalFilesScanned++

		// Get relative path
		relPath, err := filepath.Rel(absPath, path)
		if err != nil {
			return nil
		}

		// Check exclude paths for files
		shouldExclude := false
		for _, exclude := range excludePaths {
			excludeClean := strings.TrimSuffix(exclude, "/")
			if strings.Contains(relPath, excludeClean+string(os.PathSeparator)) || strings.HasPrefix(relPath, excludeClean+string(os.PathSeparator)) {
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
		if isBinaryExtensionLocalDir(relPath) {
			excludedByBinary++
			return nil
		}

		// Check file size
		if info.Size() > int64(maxFileSize) {
			excludedBySize++
			return nil
		}

		// Determine file type based on extension
		fileType := detectFileType(ext)

		files = append(files, fileInfo{
			Path:      relPath,
			AbsPath:   path,
			Folder:    filepath.Dir(relPath),
			Extension: ext,
			FileSize:  info.Size(),
			ModTime:   info.ModTime(),
			FileType:  fileType,
		})

		// Check max files limit
		if len(files) >= maxFiles {
			return filepath.SkipAll
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	matchedFiles := len(files)

	w.logger.Info().
		Str("dir_path", absPath).
		Int("total_files_in_dir", totalFilesScanned).
		Int("excluded_by_path", excludedByPath).
		Int("excluded_by_extension", excludedByExtension).
		Int("excluded_by_size", excludedBySize).
		Int("excluded_by_binary", excludedByBinary).
		Int("matched_files", matchedFiles).
		Msg("[init] Directory scanned - file analysis complete")

	// Create work items from files
	workItems := make([]interfaces.WorkItem, matchedFiles)
	for i, file := range files {
		workItems[i] = interfaces.WorkItem{
			ID:   file.Path,
			Name: filepath.Base(file.Path),
			Type: "file",
			Config: map[string]interface{}{
				"path":          file.Path,
				"absolute_path": file.AbsPath,
				"folder":        file.Folder,
				"extension":     file.Extension,
				"file_size":     file.FileSize,
				"file_type":     file.FileType,
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
		SuggestedConcurrency: batchCount,
		Metadata: map[string]interface{}{
			"base_path":             absPath,
			"total_files_scanned":   totalFilesScanned,
			"excluded_by_path":      excludedByPath,
			"excluded_by_extension": excludedByExtension,
			"excluded_by_size":      excludedBySize,
			"excluded_by_binary":    excludedByBinary,
			"matched_files":         matchedFiles,
			"batch_size":            batchSize,
			"batch_count":           batchCount,
		},
	}, nil
}

// CreateJobs creates batched queue jobs for processing files from the local directory.
// Each batch contains up to 1000 files and is processed as a separate queue job.
func (w *LocalDirWorker) CreateJobs(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string, initResult *interfaces.WorkerInitResult) (string, error) {
	// Call Init if not provided
	if initResult == nil {
		var err error
		initResult, err = w.Init(ctx, step, jobDef)
		if err != nil {
			return "", fmt.Errorf("failed to initialize local_dir worker: %w", err)
		}
	}

	// Extract metadata from init result
	basePath, _ := initResult.Metadata["base_path"].(string)
	totalFilesScanned, _ := initResult.Metadata["total_files_scanned"].(int)
	excludedByPath, _ := initResult.Metadata["excluded_by_path"].(int)
	excludedByExtension, _ := initResult.Metadata["excluded_by_extension"].(int)
	excludedBySize, _ := initResult.Metadata["excluded_by_size"].(int)
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
		Str("base_path", basePath).
		Int("total_files", totalFiles).
		Int("batch_size", batchSize).
		Int("batch_count", batchCount).
		Msg("[run] Creating batched queue jobs")

	// Add step logs for UI visibility
	w.jobManager.AddJobLogWithPhase(ctx, stepID, "info", fmt.Sprintf("Scanning '%s'", basePath), "", "run")
	w.jobManager.AddJobLogWithPhase(ctx, stepID, "info", fmt.Sprintf("Scanned %d files: %d excluded by path, %d by extension, %d by size, %d by binary",
		totalFilesScanned, excludedByPath, excludedByExtension, excludedBySize, excludedByBinary), "", "run")
	w.jobManager.AddJobLogWithPhase(ctx, stepID, "info", fmt.Sprintf("Matched %d files, creating %d batch jobs (%d files per batch)",
		matchedFiles, batchCount, batchSize), "", "run")

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

		// Convert WorkItems to file info for the batch config
		filePaths := make([]map[string]interface{}, len(batchWorkItems))
		for i, item := range batchWorkItems {
			filePaths[i] = map[string]interface{}{
				"path":          item.Config["path"],
				"absolute_path": item.Config["absolute_path"],
				"folder":        item.Config["folder"],
				"extension":     item.Config["extension"],
				"file_size":     item.Config["file_size"],
				"file_type":     item.Config["file_type"],
			}
		}

		// Create batch job
		batchJob := models.NewQueueJob(
			models.JobTypeLocalDirBatch,
			fmt.Sprintf("Batch %d/%d: %s", batchIdx+1, batchCount, filepath.Base(basePath)),
			map[string]interface{}{
				"base_path":  basePath,
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
				Msg("[run] Failed to serialize batch job")
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
				Msg("[run] Failed to create batch job record")
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
				Msg("[run] Failed to enqueue batch job")
			continue
		}

		jobIDs = append(jobIDs, batchJob.ID)

		w.logger.Debug().
			Str("job_id", batchJob.ID).
			Int("batch_idx", batchIdx).
			Int("files_in_batch", len(batchWorkItems)).
			Msg("[run] Batch job created and enqueued")
	}

	if len(jobIDs) == 0 {
		return "", fmt.Errorf("failed to create any batch jobs")
	}

	w.jobManager.AddJobLogWithPhase(ctx, stepID, "info", fmt.Sprintf("Created %d batch jobs for %d files", len(jobIDs), totalFiles), "", "run")

	w.logger.Info().
		Str("step_name", step.Name).
		Int("batches_created", len(jobIDs)).
		Int("total_files", totalFiles).
		Msg("[run] Batch jobs created and enqueued")

	// Return the step ID - orchestrator will monitor child job completion
	return stepID, nil
}

// ReturnsChildJobs returns true - we create batched child jobs for queue processing
func (w *LocalDirWorker) ReturnsChildJobs() bool {
	return true
}

// ValidateConfig validates step configuration for local directory type
func (w *LocalDirWorker) ValidateConfig(step models.JobStep) error {
	if step.Config == nil {
		return fmt.Errorf("local_dir step requires config")
	}

	// Validate dir_path
	dirPath, hasDirPath := step.Config["dir_path"].(string)
	path, hasPath := step.Config["path"].(string)

	if (!hasDirPath || dirPath == "") && (!hasPath || path == "") {
		return fmt.Errorf("local_dir step requires 'dir_path' or 'path' in config")
	}

	return nil
}

// isBinaryExtensionLocalDir checks if a file is likely binary based on extension
func isBinaryExtensionLocalDir(path string) bool {
	binaryExts := map[string]bool{
		".exe": true, ".dll": true, ".so": true, ".dylib": true, ".a": true, ".o": true,
		".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".ico": true, ".svg": true, ".webp": true,
		".pdf": true, ".doc": true, ".docx": true, ".xls": true, ".xlsx": true, ".ppt": true, ".pptx": true,
		".zip": true, ".tar": true, ".gz": true, ".rar": true, ".7z": true, ".bz2": true,
		".woff": true, ".woff2": true, ".ttf": true, ".eot": true, ".otf": true,
		".mp3": true, ".mp4": true, ".wav": true, ".avi": true, ".mov": true, ".mkv": true, ".webm": true,
		".pyc": true, ".pyo": true, ".class": true,
		".lock": true,
		".bin":  true, ".dat": true,
		".db": true, ".sqlite": true, ".sqlite3": true,
	}
	ext := strings.ToLower(filepath.Ext(path))
	return binaryExts[ext]
}

// detectFileType returns a human-readable file type based on extension
func detectFileType(ext string) string {
	ext = strings.ToLower(ext)

	codeExts := map[string]string{
		".go": "go", ".ts": "typescript", ".tsx": "typescript-react", ".js": "javascript", ".jsx": "javascript-react",
		".py": "python", ".rs": "rust", ".java": "java", ".kt": "kotlin", ".scala": "scala",
		".c": "c", ".cpp": "cpp", ".h": "c-header", ".hpp": "cpp-header",
		".rb": "ruby", ".php": "php", ".swift": "swift", ".cs": "csharp",
		".r": "r", ".jl": "julia", ".lua": "lua", ".pl": "perl",
		".sh": "shell", ".bash": "bash", ".zsh": "zsh", ".fish": "fish",
		".sql": "sql", ".graphql": "graphql",
	}
	if fileType, ok := codeExts[ext]; ok {
		return "code:" + fileType
	}

	configExts := map[string]string{
		".yaml": "yaml", ".yml": "yaml", ".toml": "toml", ".json": "json", ".xml": "xml",
		".ini": "ini", ".cfg": "config", ".conf": "config", ".env": "env",
	}
	if fileType, ok := configExts[ext]; ok {
		return "config:" + fileType
	}

	markupExts := map[string]string{
		".md": "markdown", ".mdx": "mdx", ".rst": "restructuredtext", ".adoc": "asciidoc",
		".html": "html", ".htm": "html", ".css": "css", ".scss": "scss", ".less": "less", ".sass": "sass",
	}
	if fileType, ok := markupExts[ext]; ok {
		return "markup:" + fileType
	}

	// Check for special filenames
	switch ext {
	case ".dockerfile", "dockerfile":
		return "config:dockerfile"
	case ".makefile", "makefile":
		return "build:makefile"
	case ".gitignore", ".dockerignore", ".npmignore":
		return "config:ignore"
	}

	return "text"
}
