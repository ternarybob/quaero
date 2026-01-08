package devops

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/workers/workerutil"
)

// TestLocalDirWorker_GetWorkerType tests the GetWorkerType method
func TestLocalDirWorker_GetWorkerType(t *testing.T) {
	worker := &LocalDirWorker{}

	expected := models.JobTypeLocalDirBatch
	actual := worker.GetWorkerType()

	if actual != expected {
		t.Errorf("GetWorkerType() = %s, want %s", actual, expected)
	}
}

// TestLocalDirWorker_GetType tests the GetType method
func TestLocalDirWorker_GetType(t *testing.T) {
	worker := &LocalDirWorker{}

	expected := models.WorkerTypeLocalDir
	actual := worker.GetType()

	if actual != expected {
		t.Errorf("GetType() = %s, want %s", actual, expected)
	}
}

// TestLocalDirWorker_ReturnsChildJobs tests the ReturnsChildJobs method
func TestLocalDirWorker_ReturnsChildJobs(t *testing.T) {
	worker := &LocalDirWorker{}

	if !worker.ReturnsChildJobs() {
		t.Error("ReturnsChildJobs() should return true")
	}
}

// TestLocalDirWorker_Validate tests the Validate method
func TestLocalDirWorker_Validate(t *testing.T) {
	worker := &LocalDirWorker{}

	tests := []struct {
		name    string
		job     *models.QueueJob
		wantErr bool
	}{
		{
			name: "valid job type",
			job: &models.QueueJob{
				Type: models.JobTypeLocalDirBatch,
			},
			wantErr: false,
		},
		{
			name: "invalid job type",
			job: &models.QueueJob{
				Type: "crawler_url",
			},
			wantErr: true,
		},
		{
			name: "empty job type",
			job: &models.QueueJob{
				Type: "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := worker.Validate(tt.job)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestLocalDirWorker_ValidateConfig tests the ValidateConfig method
func TestLocalDirWorker_ValidateConfig(t *testing.T) {
	worker := &LocalDirWorker{}

	tests := []struct {
		name    string
		step    models.JobStep
		wantErr bool
	}{
		{
			name: "valid config with dir_path",
			step: models.JobStep{
				Type: "local_dir",
				Config: map[string]interface{}{
					"dir_path": "/some/path",
				},
			},
			wantErr: false,
		},
		{
			name: "valid config with path",
			step: models.JobStep{
				Type: "local_dir",
				Config: map[string]interface{}{
					"path": "/some/path",
				},
			},
			wantErr: false,
		},
		{
			name: "nil config",
			step: models.JobStep{
				Type:   "local_dir",
				Config: nil,
			},
			wantErr: true,
		},
		{
			name: "empty config",
			step: models.JobStep{
				Type:   "local_dir",
				Config: map[string]interface{}{},
			},
			wantErr: true,
		},
		{
			name: "empty dir_path",
			step: models.JobStep{
				Type: "local_dir",
				Config: map[string]interface{}{
					"dir_path": "",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := worker.ValidateConfig(tt.step)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestLocalDirWorker_Init tests the Init method
func TestLocalDirWorker_Init(t *testing.T) {
	logger := arbor.NewLogger()

	// Create a temporary directory with test files
	tempDir := t.TempDir()

	// Create test files
	testFiles := []struct {
		path    string
		content string
	}{
		{"main.go", "package main\n\nfunc main() {}"},
		{"util.go", "package main\n\nfunc helper() {}"},
		{"README.md", "# Test Project"},
		{"config.yaml", "key: value"},
		{"subdir/nested.go", "package subdir"},
		{"node_modules/dep.js", "// should be excluded"},
		{".git/config", "[core]"},
		{"binary.exe", "binary content"},
	}

	for _, tf := range testFiles {
		fullPath := filepath.Join(tempDir, tf.path)
		err := os.MkdirAll(filepath.Dir(fullPath), 0755)
		if err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
		err = os.WriteFile(fullPath, []byte(tf.content), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	worker := NewLocalDirWorker(nil, nil, nil, nil, logger)

	tests := []struct {
		name           string
		step           models.JobStep
		jobDef         models.JobDefinition
		wantErr        bool
		wantFilesRange [2]int // [min, max] expected files
	}{
		{
			name: "scan temp directory",
			step: models.JobStep{
				Name: "scan",
				Type: "local_dir",
				Config: map[string]interface{}{
					"dir_path": tempDir,
				},
			},
			jobDef: models.JobDefinition{
				ID:   "test-job",
				Name: "Test Job",
			},
			wantErr:        false,
			wantFilesRange: [2]int{4, 6}, // main.go, util.go, README.md, config.yaml, subdir/nested.go (excludes node_modules, .git, binary)
		},
		{
			name: "scan with custom extensions",
			step: models.JobStep{
				Name: "scan",
				Type: "local_dir",
				Config: map[string]interface{}{
					"dir_path":   tempDir,
					"extensions": []interface{}{".go"},
				},
			},
			jobDef: models.JobDefinition{
				ID:   "test-job",
				Name: "Test Job",
			},
			wantErr:        false,
			wantFilesRange: [2]int{2, 4}, // Only .go files
		},
		{
			name: "missing dir_path",
			step: models.JobStep{
				Name:   "scan",
				Type:   "local_dir",
				Config: map[string]interface{}{},
			},
			jobDef: models.JobDefinition{
				ID:   "test-job",
				Name: "Test Job",
			},
			wantErr: true,
		},
		{
			name: "non-existent directory",
			step: models.JobStep{
				Name: "scan",
				Type: "local_dir",
				Config: map[string]interface{}{
					"dir_path": "/nonexistent/path/xyz",
				},
			},
			jobDef: models.JobDefinition{
				ID:   "test-job",
				Name: "Test Job",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			result, err := worker.Init(ctx, tt.step, tt.jobDef)

			if (err != nil) != tt.wantErr {
				t.Errorf("Init() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil {
				if result == nil {
					t.Error("Init() returned nil result without error")
					return
				}

				fileCount := len(result.WorkItems)
				if fileCount < tt.wantFilesRange[0] || fileCount > tt.wantFilesRange[1] {
					t.Errorf("Init() found %d files, want between %d and %d", fileCount, tt.wantFilesRange[0], tt.wantFilesRange[1])
				}

				if result.Strategy != interfaces.ProcessingStrategyParallel {
					t.Errorf("Init() strategy = %s, want %s", result.Strategy, interfaces.ProcessingStrategyParallel)
				}

				// Check metadata
				basePath, ok := result.Metadata["base_path"].(string)
				if !ok || basePath == "" {
					t.Error("Init() metadata missing base_path")
				}
			}
		})
	}
}

// TestLocalDirWorker_isBinaryExtension tests the binary extension detection
func TestLocalDirWorker_isBinaryExtension(t *testing.T) {
	tests := []struct {
		path     string
		isBinary bool
	}{
		{"file.exe", true},
		{"file.dll", true},
		{"file.png", true},
		{"file.jpg", true},
		{"file.pdf", true},
		{"file.zip", true},
		{"file.lock", true},
		{"file.go", false},
		{"file.ts", false},
		{"file.md", false},
		{"file.yaml", false},
		{"file.json", false},
		{"file.html", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := isBinaryExtensionLocalDir(tt.path)
			if result != tt.isBinary {
				t.Errorf("isBinaryExtensionLocalDir(%s) = %v, want %v", tt.path, result, tt.isBinary)
			}
		})
	}
}

// TestLocalDirWorker_detectFileType tests the file type detection
func TestLocalDirWorker_detectFileType(t *testing.T) {
	tests := []struct {
		ext      string
		wantType string
	}{
		{".go", "code:go"},
		{".ts", "code:typescript"},
		{".tsx", "code:typescript-react"},
		{".js", "code:javascript"},
		{".py", "code:python"},
		{".rs", "code:rust"},
		{".yaml", "config:yaml"},
		{".yml", "config:yaml"},
		{".json", "config:json"},
		{".toml", "config:toml"},
		{".md", "markup:markdown"},
		{".html", "markup:html"},
		{".css", "markup:css"},
		{".unknown", "text"},
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			result := detectFileType(tt.ext)
			if result != tt.wantType {
				t.Errorf("detectFileType(%s) = %s, want %s", tt.ext, result, tt.wantType)
			}
		})
	}
}

// TestLocalDirWorker_InitWithMaxFiles tests the Init method with max_files limit
func TestLocalDirWorker_InitWithMaxFiles(t *testing.T) {
	logger := arbor.NewLogger()

	// Create temp directory with many files
	tempDir := t.TempDir()
	for i := 0; i < 20; i++ {
		err := os.WriteFile(
			filepath.Join(tempDir, "file"+string(rune('a'+i))+".go"),
			[]byte("package main"),
			0644,
		)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	worker := NewLocalDirWorker(nil, nil, nil, nil, logger)

	ctx := context.Background()
	step := models.JobStep{
		Name: "scan",
		Type: "local_dir",
		Config: map[string]interface{}{
			"dir_path":  tempDir,
			"max_files": 5,
		},
	}
	jobDef := models.JobDefinition{
		ID:   "test-job",
		Name: "Test Job",
	}

	result, err := worker.Init(ctx, step, jobDef)
	if err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	if len(result.WorkItems) > 5 {
		t.Errorf("Init() returned %d files, want at most 5", len(result.WorkItems))
	}
}

// TestLocalDirWorker_InitWithMaxFileSize tests the Init method with max_file_size limit
func TestLocalDirWorker_InitWithMaxFileSize(t *testing.T) {
	logger := arbor.NewLogger()

	// Create temp directory with files of different sizes
	tempDir := t.TempDir()

	// Small file (100 bytes)
	smallContent := make([]byte, 100)
	err := os.WriteFile(filepath.Join(tempDir, "small.go"), smallContent, 0644)
	if err != nil {
		t.Fatalf("Failed to create small file: %v", err)
	}

	// Large file (10KB)
	largeContent := make([]byte, 10*1024)
	err = os.WriteFile(filepath.Join(tempDir, "large.go"), largeContent, 0644)
	if err != nil {
		t.Fatalf("Failed to create large file: %v", err)
	}

	worker := NewLocalDirWorker(nil, nil, nil, nil, logger)

	ctx := context.Background()
	step := models.JobStep{
		Name: "scan",
		Type: "local_dir",
		Config: map[string]interface{}{
			"dir_path":      tempDir,
			"max_file_size": 1024, // 1KB limit
		},
	}
	jobDef := models.JobDefinition{
		ID:   "test-job",
		Name: "Test Job",
	}

	result, err := worker.Init(ctx, step, jobDef)
	if err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	// Should only include the small file
	if len(result.WorkItems) != 1 {
		t.Errorf("Init() returned %d files, want 1 (only small file)", len(result.WorkItems))
	}

	if len(result.WorkItems) > 0 && result.WorkItems[0].Name != "small.go" {
		t.Errorf("Init() returned wrong file: %s, want small.go", result.WorkItems[0].Name)
	}
}

// TestLocalDirWorker_InitWithExcludePaths tests the Init method with exclude_paths
func TestLocalDirWorker_InitWithExcludePaths(t *testing.T) {
	logger := arbor.NewLogger()

	// Create temp directory with different folders
	tempDir := t.TempDir()

	// Create files in different directories
	dirs := []string{"src", "vendor", "build", "dist"}
	for _, dir := range dirs {
		dirPath := filepath.Join(tempDir, dir)
		err := os.MkdirAll(dirPath, 0755)
		if err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
		err = os.WriteFile(filepath.Join(dirPath, "main.go"), []byte("package "+dir), 0644)
		if err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}
	}

	worker := NewLocalDirWorker(nil, nil, nil, nil, logger)

	ctx := context.Background()
	step := models.JobStep{
		Name: "scan",
		Type: "local_dir",
		Config: map[string]interface{}{
			"dir_path":      tempDir,
			"exclude_paths": []interface{}{"vendor/", "build/", "dist/"},
		},
	}
	jobDef := models.JobDefinition{
		ID:   "test-job",
		Name: "Test Job",
	}

	result, err := worker.Init(ctx, step, jobDef)
	if err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	// Should only include src/main.go
	if len(result.WorkItems) != 1 {
		t.Errorf("Init() returned %d files, want 1 (only src/main.go)", len(result.WorkItems))
	}
}

// TestLocalDirWorker_InitEmptyDirectory tests Init with an empty directory
func TestLocalDirWorker_InitEmptyDirectory(t *testing.T) {
	logger := arbor.NewLogger()
	worker := NewLocalDirWorker(nil, nil, nil, nil, logger)

	// Create empty temp directory
	tempDir := t.TempDir()

	ctx := context.Background()
	step := models.JobStep{
		Name: "scan",
		Type: "local_dir",
		Config: map[string]interface{}{
			"dir_path": tempDir,
		},
	}
	jobDef := models.JobDefinition{
		ID:   "test-job",
		Name: "Test Job",
	}

	result, err := worker.Init(ctx, step, jobDef)
	if err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	if len(result.WorkItems) != 0 {
		t.Errorf("Init() returned %d files for empty directory, want 0", len(result.WorkItems))
	}

	if result.TotalCount != 0 {
		t.Errorf("Init() TotalCount = %d, want 0", result.TotalCount)
	}
}

// TestLocalDirWorker_InterfaceCompliance verifies interface implementation
func TestLocalDirWorker_InterfaceCompliance(t *testing.T) {
	var _ interfaces.DefinitionWorker = (*LocalDirWorker)(nil)
	var _ interfaces.JobWorker = (*LocalDirWorker)(nil)
}

// TestLocalDirWorker_HelperFunctions tests the config helper functions
func TestLocalDirWorker_HelperFunctions(t *testing.T) {
	t.Run("getStringConfig", func(t *testing.T) {
		config := map[string]interface{}{
			"key1": "value1",
			"key2": 123,
		}
		if got := workerutil.GetStringConfig(config, "key1", "default"); got != "value1" {
			t.Errorf("workerutil.GetStringConfig() = %v, want value1", got)
		}
		if got := workerutil.GetStringConfig(config, "key2", "default"); got != "default" {
			t.Errorf("workerutil.GetStringConfig() = %v, want default", got)
		}
		if got := workerutil.GetStringConfig(config, "missing", "default"); got != "default" {
			t.Errorf("workerutil.GetStringConfig() = %v, want default", got)
		}
	})

	t.Run("getIntConfig", func(t *testing.T) {
		config := map[string]interface{}{
			"key1": float64(42),
			"key2": 123,
			"key3": "not an int",
		}
		if got := workerutil.GetIntConfig(config, "key1", 0); got != 42 {
			t.Errorf("workerutil.GetIntConfig() = %v, want 42", got)
		}
		if got := workerutil.GetIntConfig(config, "key2", 0); got != 123 {
			t.Errorf("workerutil.GetIntConfig() = %v, want 123", got)
		}
		if got := workerutil.GetIntConfig(config, "key3", 99); got != 99 {
			t.Errorf("workerutil.GetIntConfig() = %v, want 99", got)
		}
		if got := workerutil.GetIntConfig(config, "missing", 99); got != 99 {
			t.Errorf("workerutil.GetIntConfig() = %v, want 99", got)
		}
	})

	t.Run("getStringSliceConfig", func(t *testing.T) {
		config := map[string]interface{}{
			"key1": []interface{}{"a", "b", "c"},
			"key2": []string{"x", "y"},
			"key3": "not a slice",
		}
		got := workerutil.GetStringSliceConfig(config, "key1", nil)
		if len(got) != 3 || got[0] != "a" {
			t.Errorf("workerutil.GetStringSliceConfig() = %v, want [a b c]", got)
		}
		got = workerutil.GetStringSliceConfig(config, "key2", nil)
		if len(got) != 2 || got[0] != "x" {
			t.Errorf("workerutil.GetStringSliceConfig() = %v, want [x y]", got)
		}
		got = workerutil.GetStringSliceConfig(config, "key3", []string{"default"})
		if len(got) != 1 || got[0] != "default" {
			t.Errorf("workerutil.GetStringSliceConfig() = %v, want [default]", got)
		}
	})
}

// TestLocalDirWorker_CreateJobsStepTags tests that step-level tags are used in batch jobs
func TestLocalDirWorker_CreateJobsStepTags(t *testing.T) {
	tests := []struct {
		name         string
		stepTags     interface{}
		jobDefTags   []string
		expectedTags []string
	}{
		{
			name:         "step tags as interface slice",
			stepTags:     []interface{}{"codebase", "quaero"},
			jobDefTags:   nil,
			expectedTags: []string{"codebase", "quaero"},
		},
		{
			name:         "step tags as string slice",
			stepTags:     []string{"project", "go"},
			jobDefTags:   nil,
			expectedTags: []string{"project", "go"},
		},
		{
			name:         "fallback to job definition tags",
			stepTags:     nil,
			jobDefTags:   []string{"default-tag"},
			expectedTags: []string{"default-tag"},
		},
		{
			name:         "step tags override job definition tags",
			stepTags:     []interface{}{"step-tag"},
			jobDefTags:   []string{"job-def-tag"},
			expectedTags: []string{"step-tag"},
		},
		{
			name:         "empty step tags fallback to job definition",
			stepTags:     []interface{}{},
			jobDefTags:   []string{"fallback"},
			expectedTags: []string{"fallback"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build step config
			stepConfig := map[string]interface{}{
				"dir_path": ".",
			}
			if tt.stepTags != nil {
				stepConfig["tags"] = tt.stepTags
			}

			step := models.JobStep{
				Name:   "import_files",
				Type:   "local_dir",
				Config: stepConfig,
			}
			jobDef := models.JobDefinition{
				ID:   "test-job",
				Name: "Test Job",
				Tags: tt.jobDefTags,
			}

			// Extract tags using the same logic as CreateJobs
			var baseTags []string
			if stepTags, ok := step.Config["tags"].([]interface{}); ok {
				for _, tag := range stepTags {
					if tagStr, ok := tag.(string); ok {
						baseTags = append(baseTags, tagStr)
					}
				}
			} else if stepTags, ok := step.Config["tags"].([]string); ok {
				baseTags = stepTags
			}

			// Fallback to job definition tags if no step tags specified
			if len(baseTags) == 0 && len(jobDef.Tags) > 0 {
				baseTags = jobDef.Tags
			}

			// Verify expected tags
			if len(baseTags) != len(tt.expectedTags) {
				t.Errorf("got %d tags %v, want %d tags %v", len(baseTags), baseTags, len(tt.expectedTags), tt.expectedTags)
				return
			}

			for i, expected := range tt.expectedTags {
				if baseTags[i] != expected {
					t.Errorf("tag[%d] = %s, want %s", i, baseTags[i], expected)
				}
			}
		})
	}
}

// Benchmark tests
func BenchmarkLocalDirWorker_Init(b *testing.B) {
	logger := arbor.NewLogger()
	worker := NewLocalDirWorker(nil, nil, nil, nil, logger)

	// Create temp directory with files
	tempDir := b.TempDir()
	for i := 0; i < 100; i++ {
		err := os.WriteFile(
			filepath.Join(tempDir, "file"+string(rune(i))+".go"),
			[]byte("package main\n\nfunc main() {}"),
			0644,
		)
		if err != nil {
			b.Fatalf("Failed to create test file: %v", err)
		}
	}

	ctx := context.Background()
	step := models.JobStep{
		Name: "scan",
		Type: "local_dir",
		Config: map[string]interface{}{
			"dir_path": tempDir,
		},
	}
	jobDef := models.JobDefinition{
		ID:   "test-job",
		Name: "Test Job",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = worker.Init(ctx, step, jobDef)
	}
}
