// -----------------------------------------------------------------------
// Code Map Worker - Hierarchical code structure analysis
// Creates a lightweight map of codebases for efficient AI processing
// Stores structure and metadata, not full file contents
// -----------------------------------------------------------------------

package devops

import (
	"bufio"
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
	"github.com/ternarybob/quaero/internal/workers/workerutil"
)

// CodeMapWorker creates hierarchical code structure maps
// Optimized for large codebases (2GB+, 10k+ files)
// Instead of storing full content, stores:
// - Project-level summary document
// - Directory documents with aggregated stats
// - File metadata documents (no content, just structure)
type CodeMapWorker struct {
	jobManager      *queue.Manager
	queueMgr        interfaces.QueueManager
	documentStorage interfaces.DocumentStorage
	agentService    interfaces.AgentService // For AI summarization
	eventService    interfaces.EventService
	logger          arbor.ILogger
}

// Compile-time assertions
var _ interfaces.DefinitionWorker = (*CodeMapWorker)(nil)
var _ interfaces.JobWorker = (*CodeMapWorker)(nil)

// NewCodeMapWorker creates a new code map worker
func NewCodeMapWorker(
	jobManager *queue.Manager,
	queueMgr interfaces.QueueManager,
	documentStorage interfaces.DocumentStorage,
	agentService interfaces.AgentService,
	eventService interfaces.EventService,
	logger arbor.ILogger,
) *CodeMapWorker {
	return &CodeMapWorker{
		jobManager:      jobManager,
		queueMgr:        queueMgr,
		documentStorage: documentStorage,
		agentService:    agentService,
		eventService:    eventService,
		logger:          logger,
	}
}

// GetWorkerType returns the job type this worker handles
func (w *CodeMapWorker) GetWorkerType() string {
	return models.JobTypeCodeMapStructure
}

// Validate validates that the queue job is compatible with this worker
func (w *CodeMapWorker) Validate(job *models.QueueJob) error {
	if job.Type != models.JobTypeCodeMapStructure && job.Type != models.JobTypeCodeMapSummary {
		return fmt.Errorf("invalid job type: expected %s or %s, got %s",
			models.JobTypeCodeMapStructure, models.JobTypeCodeMapSummary, job.Type)
	}
	return nil
}

// Execute processes a code map job
func (w *CodeMapWorker) Execute(ctx context.Context, job *models.QueueJob) error {
	switch job.Type {
	case models.JobTypeCodeMapStructure:
		return w.executeStructureJob(ctx, job)
	case models.JobTypeCodeMapSummary:
		return w.executeSummaryJob(ctx, job)
	default:
		return fmt.Errorf("unknown job type: %s", job.Type)
	}
}

// executeStructureJob creates structure documents for a directory subtree
func (w *CodeMapWorker) executeStructureJob(ctx context.Context, job *models.QueueJob) error {
	basePath, _ := job.Config["base_path"].(string)
	dirPath, _ := job.Config["dir_path"].(string)
	projectName, _ := job.Config["project_name"].(string)
	tagsRaw, _ := job.Config["tags"].([]interface{})

	var tags []string
	for _, t := range tagsRaw {
		if s, ok := t.(string); ok {
			tags = append(tags, s)
		}
	}

	if dirPath == "" {
		dirPath = basePath
	}

	w.logger.Info().
		Str("job_id", job.ID).
		Str("base_path", basePath).
		Str("dir_path", dirPath).
		Msg("[run] Processing code map structure job")

	// Build the directory tree
	tree, err := w.buildDirectoryTree(ctx, basePath, dirPath)
	if err != nil {
		return fmt.Errorf("failed to build directory tree: %w", err)
	}

	// Create documents for the tree
	docsCreated, err := w.createTreeDocuments(ctx, tree, basePath, projectName, tags)
	if err != nil {
		return fmt.Errorf("failed to create tree documents: %w", err)
	}

	w.jobManager.AddJobLogWithPhase(ctx, job.ID, "info",
		fmt.Sprintf("Created %d structure documents for %s", docsCreated, dirPath), "", "run")

	return nil
}

// executeSummaryJob runs AI summarization on code map documents
func (w *CodeMapWorker) executeSummaryJob(ctx context.Context, job *models.QueueJob) error {
	// This would use the AgentService to generate summaries
	// For now, this is a placeholder for future AI integration
	w.logger.Info().
		Str("job_id", job.ID).
		Msg("[run] Code map summary job - AI summarization not yet implemented")

	return nil
}

// ============================================================================
// DEFINITIONWORKER INTERFACE METHODS
// ============================================================================

// GetType returns WorkerTypeCodeMap
func (w *CodeMapWorker) GetType() models.WorkerType {
	return models.WorkerTypeCodeMap
}

// Init performs initialization for a code map step
func (w *CodeMapWorker) Init(ctx context.Context, step models.JobStep, jobDef models.JobDefinition) (*interfaces.WorkerInitResult, error) {
	stepConfig := step.Config
	if stepConfig == nil {
		stepConfig = make(map[string]interface{})
	}

	// Extract config
	dirPath := workerutil.GetStringConfig(stepConfig, "dir_path", "")
	if dirPath == "" {
		dirPath = workerutil.GetStringConfig(stepConfig, "path", "")
	}
	if dirPath == "" {
		return nil, fmt.Errorf("dir_path is required")
	}

	projectName := workerutil.GetStringConfig(stepConfig, "project_name", filepath.Base(dirPath))
	maxDepth := workerutil.GetIntConfig(stepConfig, "max_depth", 10)
	skipSummarization := getBoolConfig(stepConfig, "skip_summarization", false)

	// Resolve to absolute path
	absPath, err := filepath.Abs(dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	// Check directory exists
	info, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("directory does not exist: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("path is not a directory: %s", absPath)
	}

	w.logger.Info().
		Str("step_name", step.Name).
		Str("dir_path", absPath).
		Str("project_name", projectName).
		Int("max_depth", maxDepth).
		Msg("[init] Initializing code map worker - analyzing directory structure")

	// Quick scan to count directories for batch planning
	dirCount := 0
	filepath.Walk(absPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			relPath, _ := filepath.Rel(absPath, path)
			// Skip excluded directories
			if shouldExcludeDir(relPath) {
				return filepath.SkipDir
			}
			// Check depth
			depth := strings.Count(relPath, string(os.PathSeparator))
			if relPath != "." && depth < maxDepth {
				dirCount++
			}
		}
		return nil
	})

	// Create work items - one for root structure, optionally one for summarization
	workItems := []interfaces.WorkItem{
		{
			ID:   "structure",
			Name: "Build code structure",
			Type: "structure",
			Config: map[string]interface{}{
				"dir_path":     absPath,
				"project_name": projectName,
				"max_depth":    maxDepth,
			},
		},
	}

	if !skipSummarization {
		workItems = append(workItems, interfaces.WorkItem{
			ID:   "summarize",
			Name: "Generate AI summaries",
			Type: "summary",
			Config: map[string]interface{}{
				"dir_path":     absPath,
				"project_name": projectName,
			},
		})
	}

	return &interfaces.WorkerInitResult{
		WorkItems:            workItems,
		TotalCount:           len(workItems),
		Strategy:             interfaces.ProcessingStrategySequential,
		SuggestedConcurrency: 1,
		Metadata: map[string]interface{}{
			"base_path":          absPath,
			"project_name":       projectName,
			"directory_count":    dirCount,
			"max_depth":          maxDepth,
			"skip_summarization": skipSummarization,
		},
	}, nil
}

// CreateJobs creates code map jobs
func (w *CodeMapWorker) CreateJobs(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string, initResult *interfaces.WorkerInitResult) (string, error) {
	if initResult == nil {
		var err error
		initResult, err = w.Init(ctx, step, jobDef)
		if err != nil {
			return "", fmt.Errorf("failed to initialize code_map worker: %w", err)
		}
	}

	basePath, _ := initResult.Metadata["base_path"].(string)
	projectName, _ := initResult.Metadata["project_name"].(string)
	dirCount, _ := initResult.Metadata["directory_count"].(int)

	w.logger.Info().
		Str("step_name", step.Name).
		Str("base_path", basePath).
		Int("directories", dirCount).
		Msg("[run] Creating code map jobs")

	w.jobManager.AddJobLogWithPhase(ctx, stepID, "info",
		fmt.Sprintf("Analyzing '%s' (%d directories)", projectName, dirCount), "", "run")

	// Get tags
	baseTags := jobDef.Tags
	if baseTags == nil {
		baseTags = []string{}
	}

	jobIDs := make([]string, 0, len(initResult.WorkItems))

	for _, workItem := range initResult.WorkItems {
		var jobType string
		if workItem.Type == "structure" {
			jobType = models.JobTypeCodeMapStructure
		} else {
			jobType = models.JobTypeCodeMapSummary
		}

		job := models.NewQueueJob(
			jobType,
			fmt.Sprintf("Code Map: %s (%s)", projectName, workItem.Type),
			map[string]interface{}{
				"base_path":    basePath,
				"dir_path":     workItem.Config["dir_path"],
				"project_name": projectName,
				"tags":         baseTags,
			},
			map[string]interface{}{
				"job_definition_id": jobDef.ID,
				"step_name":         step.Name,
			},
		)
		job.ParentID = &stepID

		payloadBytes, err := job.ToJSON()
		if err != nil {
			w.logger.Error().Err(err).Msg("[run] Failed to serialize job")
			continue
		}

		if err := w.jobManager.CreateJobRecord(ctx, &queue.Job{
			ID:        job.ID,
			ParentID:  job.ParentID,
			Type:      job.Type,
			Name:      job.Name,
			Phase:     "execution",
			Status:    "pending",
			CreatedAt: job.CreatedAt,
			Payload:   string(payloadBytes),
		}); err != nil {
			w.logger.Error().Err(err).Msg("[run] Failed to create job record")
			continue
		}

		msg := models.QueueMessage{
			JobID:   job.ID,
			Type:    job.Type,
			Payload: payloadBytes,
		}
		if err := w.queueMgr.Enqueue(ctx, msg); err != nil {
			w.logger.Error().Err(err).Msg("[run] Failed to enqueue job")
			continue
		}

		jobIDs = append(jobIDs, job.ID)
	}

	if len(jobIDs) == 0 {
		return "", fmt.Errorf("failed to create any code map jobs")
	}

	w.jobManager.AddJobLogWithPhase(ctx, stepID, "info",
		fmt.Sprintf("Created %d code map jobs", len(jobIDs)), "", "run")

	return stepID, nil
}

// ReturnsChildJobs returns true
func (w *CodeMapWorker) ReturnsChildJobs() bool {
	return true
}

// ValidateConfig validates step configuration
func (w *CodeMapWorker) ValidateConfig(step models.JobStep) error {
	if step.Config == nil {
		return fmt.Errorf("code_map step requires config")
	}

	dirPath, hasDirPath := step.Config["dir_path"].(string)
	path, hasPath := step.Config["path"].(string)

	if (!hasDirPath || dirPath == "") && (!hasPath || path == "") {
		return fmt.Errorf("code_map step requires 'dir_path' or 'path' in config")
	}

	return nil
}

// ============================================================================
// DIRECTORY TREE BUILDING
// ============================================================================

// dirNode represents a node in the directory tree
type dirNode struct {
	Path      string
	Name      string
	IsDir     bool
	Size      int64
	ModTime   time.Time
	Children  []*dirNode
	FileCount int
	DirCount  int
	LOC       int
	Languages map[string]int // language -> LOC
	Extension string
	Exports   []string
	Imports   []string
	HasTests  bool
}

// buildDirectoryTree builds an in-memory tree of the directory structure
func (w *CodeMapWorker) buildDirectoryTree(ctx context.Context, basePath, startPath string) (*dirNode, error) {
	info, err := os.Stat(startPath)
	if err != nil {
		return nil, err
	}

	root := &dirNode{
		Path:      startPath,
		Name:      filepath.Base(startPath),
		IsDir:     info.IsDir(),
		Size:      info.Size(),
		ModTime:   info.ModTime(),
		Languages: make(map[string]int),
	}

	if !info.IsDir() {
		w.analyzeFile(root, startPath)
		return root, nil
	}

	// Read directory entries
	entries, err := os.ReadDir(startPath)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		childPath := filepath.Join(startPath, entry.Name())
		relPath, _ := filepath.Rel(basePath, childPath)

		// Skip excluded paths
		if entry.IsDir() && shouldExcludeDir(relPath) {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		child := &dirNode{
			Path:      childPath,
			Name:      entry.Name(),
			IsDir:     entry.IsDir(),
			Size:      info.Size(),
			ModTime:   info.ModTime(),
			Languages: make(map[string]int),
		}

		if entry.IsDir() {
			// Recursively process subdirectory
			subTree, err := w.buildDirectoryTree(ctx, basePath, childPath)
			if err != nil {
				w.logger.Debug().Err(err).Str("path", childPath).Msg("Failed to process subdirectory")
				continue
			}
			child = subTree
			root.DirCount += 1 + child.DirCount
			root.FileCount += child.FileCount
			root.LOC += child.LOC
			for lang, loc := range child.Languages {
				root.Languages[lang] += loc
			}
		} else {
			// Analyze file
			if !isBinaryExtensionCodeMap(childPath) {
				w.analyzeFile(child, childPath)
				root.FileCount++
				root.LOC += child.LOC
				if child.Extension != "" {
					lang := extensionToLanguage(child.Extension)
					root.Languages[lang] += child.LOC
				}
			}
		}

		root.Size += child.Size
		root.Children = append(root.Children, child)
	}

	return root, nil
}

// analyzeFile extracts metadata from a file without storing full content
func (w *CodeMapWorker) analyzeFile(node *dirNode, path string) {
	node.Extension = strings.ToLower(filepath.Ext(path))

	// Count lines
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineCount := 0
	var imports []string
	var exports []string

	// Simple pattern matching for common languages
	importPatterns := map[string]*regexp.Regexp{
		".go":  regexp.MustCompile(`^import\s+(?:\(|")`),
		".ts":  regexp.MustCompile(`^import\s+`),
		".tsx": regexp.MustCompile(`^import\s+`),
		".js":  regexp.MustCompile(`^(?:import|require)\s*\(`),
		".jsx": regexp.MustCompile(`^(?:import|require)\s*\(`),
		".py":  regexp.MustCompile(`^(?:import|from)\s+`),
	}

	exportPatterns := map[string]*regexp.Regexp{
		".go":  regexp.MustCompile(`^func\s+([A-Z]\w*)|^type\s+([A-Z]\w*)`),
		".ts":  regexp.MustCompile(`^export\s+(?:function|class|const|interface|type)\s+(\w+)`),
		".tsx": regexp.MustCompile(`^export\s+(?:function|class|const|interface|type)\s+(\w+)`),
		".js":  regexp.MustCompile(`^export\s+(?:function|class|const)\s+(\w+)`),
		".jsx": regexp.MustCompile(`^export\s+(?:function|class|const)\s+(\w+)`),
		".py":  regexp.MustCompile(`^(?:def|class)\s+(\w+)`),
	}

	importRe := importPatterns[node.Extension]
	exportRe := exportPatterns[node.Extension]

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		lineCount++

		// Extract imports (first 50 lines typically)
		if lineCount <= 50 && importRe != nil && importRe.MatchString(line) {
			imports = append(imports, line)
		}

		// Extract exports
		if exportRe != nil {
			if matches := exportRe.FindStringSubmatch(line); len(matches) > 1 {
				for _, m := range matches[1:] {
					if m != "" {
						exports = append(exports, m)
					}
				}
			}
		}
	}

	node.LOC = lineCount
	node.Imports = imports
	node.Exports = exports

	// Detect test files
	name := strings.ToLower(node.Name)
	node.HasTests = strings.Contains(name, "_test") ||
		strings.Contains(name, ".test.") ||
		strings.Contains(name, ".spec.") ||
		strings.HasPrefix(name, "test_")

	// Compute MD5 for change detection (on small files only)
	if node.Size < 1024*1024 { // < 1MB
		content, err := os.ReadFile(path)
		if err == nil {
			hash := md5.Sum(content)
			node.Extension = hex.EncodeToString(hash[:])
		}
	}
}

// createTreeDocuments creates document entries for the directory tree
func (w *CodeMapWorker) createTreeDocuments(ctx context.Context, tree *dirNode, basePath, projectName string, tags []string) (int, error) {
	count := 0

	// Determine node type
	nodeType := "file"
	if tree.IsDir {
		relPath, _ := filepath.Rel(basePath, tree.Path)
		if relPath == "." || tree.Path == basePath {
			nodeType = "project"
		} else {
			nodeType = "directory"
		}
	}

	// Build metadata
	relPath, _ := filepath.Rel(basePath, tree.Path)
	if relPath == "." {
		relPath = ""
	}

	parentPath := ""
	if relPath != "" {
		parentPath = filepath.Dir(relPath)
		if parentPath == "." {
			parentPath = ""
		}
	}

	// Get main language
	mainLang := ""
	maxLOC := 0
	for lang, loc := range tree.Languages {
		if loc > maxLOC {
			maxLOC = loc
			mainLang = lang
		}
	}

	// Convert languages map to sorted slice
	var languages []string
	for lang := range tree.Languages {
		languages = append(languages, lang)
	}
	sort.Strings(languages)

	// Get child paths
	var childPaths []string
	for _, child := range tree.Children {
		childRelPath, _ := filepath.Rel(basePath, child.Path)
		childPaths = append(childPaths, childRelPath)
	}

	metadata := map[string]interface{}{
		"code_map": map[string]interface{}{
			"base_path":     basePath,
			"node_type":     nodeType,
			"rel_path":      relPath,
			"parent_path":   parentPath,
			"project_name":  projectName,
			"child_count":   len(tree.Children),
			"file_count":    tree.FileCount,
			"dir_count":     tree.DirCount,
			"total_size":    tree.Size,
			"total_loc":     tree.LOC,
			"languages":     languages,
			"main_language": mainLang,
			"child_paths":   childPaths,
			"extension":     tree.Extension,
			"file_size":     tree.Size,
			"loc":           tree.LOC,
			"language":      extensionToLanguage(tree.Extension),
			"exports":       tree.Exports,
			"imports":       tree.Imports,
			"has_tests":     tree.HasTests,
			"mod_time":      tree.ModTime.Format(time.RFC3339),
			"indexed":       true,
			"last_indexed":  time.Now().Format(time.RFC3339),
		},
	}

	// Build content markdown (lightweight summary, not full content)
	var content strings.Builder
	content.WriteString(fmt.Sprintf("# %s\n\n", tree.Name))

	if tree.IsDir {
		content.WriteString(fmt.Sprintf("**Type:** %s\n", nodeType))
		content.WriteString(fmt.Sprintf("**Path:** `%s`\n", relPath))
		content.WriteString(fmt.Sprintf("**Files:** %d | **Directories:** %d | **LOC:** %d\n\n", tree.FileCount, tree.DirCount, tree.LOC))

		if len(languages) > 0 {
			content.WriteString("**Languages:**\n")
			for _, lang := range languages {
				content.WriteString(fmt.Sprintf("- %s: %d LOC\n", lang, tree.Languages[lang]))
			}
			content.WriteString("\n")
		}

		if len(tree.Children) > 0 {
			content.WriteString("**Contents:**\n")
			for _, child := range tree.Children {
				if child.IsDir {
					content.WriteString(fmt.Sprintf("- `%s/` (%d files, %d LOC)\n", child.Name, child.FileCount, child.LOC))
				} else {
					content.WriteString(fmt.Sprintf("- `%s` (%d LOC)\n", child.Name, child.LOC))
				}
			}
		}
	} else {
		content.WriteString(fmt.Sprintf("**Type:** file\n"))
		content.WriteString(fmt.Sprintf("**Language:** %s\n", extensionToLanguage(tree.Extension)))
		content.WriteString(fmt.Sprintf("**LOC:** %d\n\n", tree.LOC))

		if len(tree.Exports) > 0 {
			content.WriteString("**Exports:**\n")
			for _, exp := range tree.Exports {
				content.WriteString(fmt.Sprintf("- `%s`\n", exp))
			}
			content.WriteString("\n")
		}

		if len(tree.Imports) > 0 && len(tree.Imports) <= 10 {
			content.WriteString("**Imports:**\n")
			for _, imp := range tree.Imports {
				content.WriteString(fmt.Sprintf("- %s\n", imp))
			}
		}
	}

	// Create document
	docID := uuid.New().String()
	sourceID := fmt.Sprintf("%s:%s", projectName, relPath)
	if relPath == "" {
		sourceID = projectName
	}

	doc := &models.Document{
		ID:              docID,
		SourceType:      models.SourceTypeCodeMap,
		SourceID:        sourceID,
		Title:           fmt.Sprintf("[%s] %s", nodeType, tree.Name),
		ContentMarkdown: content.String(),
		URL:             "file://" + tree.Path,
		Tags:            tags,
		Metadata:        metadata,
	}

	if err := w.documentStorage.SaveDocument(doc); err != nil {
		return count, fmt.Errorf("failed to save document: %w", err)
	}
	count++

	// Recursively create documents for children (directories only at first level)
	for _, child := range tree.Children {
		if child.IsDir {
			childCount, err := w.createTreeDocuments(ctx, child, basePath, projectName, tags)
			if err != nil {
				w.logger.Debug().Err(err).Str("path", child.Path).Msg("Failed to create child documents")
			}
			count += childCount
		}
	}

	return count, nil
}

// ============================================================================
// HELPERS
// ============================================================================

func shouldExcludeDir(relPath string) bool {
	excludeDirs := []string{
		".git", "node_modules", "vendor", "__pycache__", ".venv", "venv",
		"dist", "build", "target", ".gradle", ".mvn", ".idea", ".vscode",
		".vs", "bin", "obj", "out", ".next", ".nuxt", "coverage",
	}
	parts := strings.Split(relPath, string(os.PathSeparator))
	for _, part := range parts {
		for _, exclude := range excludeDirs {
			if part == exclude {
				return true
			}
		}
	}
	return false
}

func isBinaryExtensionCodeMap(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	binaryExts := map[string]bool{
		".exe": true, ".dll": true, ".so": true, ".dylib": true, ".a": true, ".o": true,
		".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".ico": true, ".svg": true,
		".pdf": true, ".doc": true, ".docx": true, ".zip": true, ".tar": true, ".gz": true,
		".woff": true, ".woff2": true, ".ttf": true, ".eot": true,
		".mp3": true, ".mp4": true, ".wav": true, ".avi": true,
		".pyc": true, ".pyo": true, ".class": true, ".lock": true,
		".bin": true, ".db": true, ".sqlite": true,
	}
	return binaryExts[ext]
}

func extensionToLanguage(ext string) string {
	langMap := map[string]string{
		".go":    "Go",
		".ts":    "TypeScript",
		".tsx":   "TypeScript",
		".js":    "JavaScript",
		".jsx":   "JavaScript",
		".py":    "Python",
		".rs":    "Rust",
		".java":  "Java",
		".kt":    "Kotlin",
		".swift": "Swift",
		".c":     "C",
		".cpp":   "C++",
		".h":     "C",
		".hpp":   "C++",
		".cs":    "C#",
		".rb":    "Ruby",
		".php":   "PHP",
		".scala": "Scala",
		".r":     "R",
		".sql":   "SQL",
		".sh":    "Shell",
		".bash":  "Shell",
		".md":    "Markdown",
		".yaml":  "YAML",
		".yml":   "YAML",
		".json":  "JSON",
		".toml":  "TOML",
		".xml":   "XML",
		".html":  "HTML",
		".css":   "CSS",
		".scss":  "SCSS",
	}
	if lang, ok := langMap[ext]; ok {
		return lang
	}
	return "Other"
}

func getBoolConfig(config map[string]interface{}, key string, defaultValue bool) bool {
	if v, ok := config[key].(bool); ok {
		return v
	}
	return defaultValue
}
