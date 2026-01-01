package workers

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
)

// TestSummaryWorker_GetType tests the GetType method
func TestSummaryWorker_GetType(t *testing.T) {
	worker := &SummaryWorker{}

	expected := models.WorkerTypeSummary
	actual := worker.GetType()

	if actual != expected {
		t.Errorf("GetType() = %s, want %s", actual, expected)
	}
}

// TestSummaryWorker_ReturnsChildJobs tests that summary worker executes synchronously
func TestSummaryWorker_ReturnsChildJobs(t *testing.T) {
	worker := &SummaryWorker{}

	if worker.ReturnsChildJobs() {
		t.Error("ReturnsChildJobs() should return false for summary worker")
	}
}

// TestSummaryWorker_ValidateConfig tests the ValidateConfig method
func TestSummaryWorker_ValidateConfig(t *testing.T) {
	worker := &SummaryWorker{}

	tests := []struct {
		name    string
		step    models.JobStep
		wantErr bool
	}{
		{
			name: "valid config with prompt and filter_tags (interface slice)",
			step: models.JobStep{
				Type: "summary",
				Config: map[string]interface{}{
					"prompt":      "Summarize the code architecture",
					"filter_tags": []interface{}{"codebase", "project"},
					"api_key":     "test-key",
				},
			},
			wantErr: false,
		},
		{
			name: "valid config with prompt and filter_tags (string slice)",
			step: models.JobStep{
				Type: "summary",
				Config: map[string]interface{}{
					"prompt":      "Summarize the code architecture",
					"filter_tags": []string{"codebase", "project"},
					"api_key":     "test-key",
				},
			},
			wantErr: false,
		},
		{
			name: "nil config",
			step: models.JobStep{
				Type:   "summary",
				Config: nil,
			},
			wantErr: true,
		},
		{
			name: "empty config",
			step: models.JobStep{
				Type:   "summary",
				Config: map[string]interface{}{},
			},
			wantErr: true,
		},
		{
			name: "missing prompt",
			step: models.JobStep{
				Type: "summary",
				Config: map[string]interface{}{
					"filter_tags": []interface{}{"codebase"},
				},
			},
			wantErr: true,
		},
		{
			name: "empty prompt",
			step: models.JobStep{
				Type: "summary",
				Config: map[string]interface{}{
					"prompt":      "",
					"filter_tags": []interface{}{"codebase"},
				},
			},
			wantErr: true,
		},
		{
			name: "missing filter_tags",
			step: models.JobStep{
				Type: "summary",
				Config: map[string]interface{}{
					"prompt": "Summarize the code",
				},
			},
			wantErr: true,
		},
		{
			name: "empty filter_tags",
			step: models.JobStep{
				Type: "summary",
				Config: map[string]interface{}{
					"prompt":      "Summarize the code",
					"filter_tags": []interface{}{},
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

// MockSearchService is a mock implementation of interfaces.SearchService for testing
type MockSearchService struct {
	mock.Mock
}

func (m *MockSearchService) Search(ctx context.Context, query string, opts interfaces.SearchOptions) ([]*models.Document, error) {
	args := m.Called(ctx, query, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Document), args.Error(1)
}

func (m *MockSearchService) GetByID(ctx context.Context, id string) (*models.Document, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Document), args.Error(1)
}

func (m *MockSearchService) SearchByReference(ctx context.Context, reference string, opts interfaces.SearchOptions) ([]*models.Document, error) {
	args := m.Called(ctx, reference, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Document), args.Error(1)
}

// MockKVStorage is a mock implementation of interfaces.KeyValueStorage for testing
type MockKVStorage struct {
	mock.Mock
}

func (m *MockKVStorage) Get(ctx context.Context, key string) (string, error) {
	args := m.Called(ctx, key)
	return args.String(0), args.Error(1)
}

func (m *MockKVStorage) GetPair(ctx context.Context, key string) (*interfaces.KeyValuePair, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*interfaces.KeyValuePair), args.Error(1)
}

func (m *MockKVStorage) Set(ctx context.Context, key, value, description string) error {
	args := m.Called(ctx, key, value, description)
	return args.Error(0)
}

func (m *MockKVStorage) Upsert(ctx context.Context, key, value, description string) (bool, error) {
	args := m.Called(ctx, key, value, description)
	return args.Bool(0), args.Error(1)
}

func (m *MockKVStorage) Delete(ctx context.Context, key string) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}

func (m *MockKVStorage) DeleteAll(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockKVStorage) List(ctx context.Context) ([]interfaces.KeyValuePair, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]interfaces.KeyValuePair), args.Error(1)
}

func (m *MockKVStorage) GetAll(ctx context.Context) (map[string]string, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]string), args.Error(1)
}

// TestSummaryWorker_Init tests the Init method
func TestSummaryWorker_Init(t *testing.T) {
	logger := arbor.NewLogger()

	// Create mock services
	mockSearch := new(MockSearchService)
	mockKV := new(MockKVStorage)

	// Create test documents
	testDocs := []*models.Document{
		{
			ID:              "doc_1",
			Title:           "main.go",
			ContentMarkdown: "package main\n\nfunc main() {}",
			Tags:            []string{"codebase"},
			CreatedAt:       time.Now(),
		},
		{
			ID:              "doc_2",
			Title:           "utils.go",
			ContentMarkdown: "package main\n\nfunc helper() string { return \"help\" }",
			Tags:            []string{"codebase"},
			CreatedAt:       time.Now(),
		},
	}

	tests := []struct {
		name         string
		step         models.JobStep
		jobDef       models.JobDefinition
		setupMocks   func()
		wantErr      bool
		wantDocCount int
	}{
		{
			name: "successful init with documents",
			step: models.JobStep{
				Name: "summarize",
				Type: "summary",
				Config: map[string]interface{}{
					"prompt":      "Provide an architectural summary",
					"filter_tags": []interface{}{"codebase"},
					"api_key":     "test-api-key",
				},
			},
			jobDef: models.JobDefinition{
				ID:   "test-job",
				Name: "Test Summary Job",
			},
			setupMocks: func() {
				mockSearch.On("Search", mock.Anything, "", mock.MatchedBy(func(opts interfaces.SearchOptions) bool {
					return len(opts.Tags) > 0 && opts.Tags[0] == "codebase"
				})).Return(testDocs, nil).Once()
			},
			wantErr:      false,
			wantDocCount: 2,
		},
		{
			name: "missing prompt",
			step: models.JobStep{
				Name: "summarize",
				Type: "summary",
				Config: map[string]interface{}{
					"filter_tags": []interface{}{"codebase"},
					"api_key":     "test-api-key",
				},
			},
			jobDef: models.JobDefinition{
				ID:   "test-job",
				Name: "Test Summary Job",
			},
			setupMocks: func() {},
			wantErr:    true,
		},
		{
			name: "missing filter_tags",
			step: models.JobStep{
				Name: "summarize",
				Type: "summary",
				Config: map[string]interface{}{
					"prompt":  "Summarize this",
					"api_key": "test-api-key",
				},
			},
			jobDef: models.JobDefinition{
				ID:   "test-job",
				Name: "Test Summary Job",
			},
			setupMocks: func() {},
			wantErr:    true,
		},
		// Note: api_key test case removed - api_key is no longer required in Init
		// It's resolved by provider factory during CreateJobs
		{
			name: "nil config",
			step: models.JobStep{
				Name:   "summarize",
				Type:   "summary",
				Config: nil,
			},
			jobDef: models.JobDefinition{
				ID:   "test-job",
				Name: "Test Summary Job",
			},
			setupMocks: func() {},
			wantErr:    true,
		},
		{
			name: "no documents found",
			step: models.JobStep{
				Name: "summarize",
				Type: "summary",
				Config: map[string]interface{}{
					"prompt":      "Summarize this",
					"filter_tags": []interface{}{"nonexistent-tag"},
					"api_key":     "test-api-key",
				},
			},
			jobDef: models.JobDefinition{
				ID:   "test-job",
				Name: "Test Summary Job",
			},
			setupMocks: func() {
				mockSearch.On("Search", mock.Anything, "", mock.Anything).Return([]*models.Document{}, nil).Once()
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset mocks
			mockSearch.ExpectedCalls = nil
			mockSearch.Calls = nil
			mockKV.ExpectedCalls = nil
			mockKV.Calls = nil

			// Setup mocks for this test
			tt.setupMocks()

			worker := NewSummaryWorker(mockSearch, nil, nil, mockKV, logger, nil, nil)
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

				if len(result.WorkItems) != tt.wantDocCount {
					t.Errorf("Init() found %d documents, want %d", len(result.WorkItems), tt.wantDocCount)
				}

				// Verify strategy is inline (synchronous)
				if result.Strategy != interfaces.ProcessingStrategyInline {
					t.Errorf("Init() strategy = %s, want %s", result.Strategy, interfaces.ProcessingStrategyInline)
				}

				// Verify metadata
				if result.Metadata == nil {
					t.Error("Init() metadata should not be nil")
				} else {
					if _, ok := result.Metadata["prompt"].(string); !ok {
						t.Error("Init() metadata missing prompt")
					}
					if _, ok := result.Metadata["filter_tags"].([]string); !ok {
						t.Error("Init() metadata missing filter_tags")
					}
					// Note: api_key is no longer in metadata - it's resolved by provider factory
					if _, ok := result.Metadata["documents"].([]*models.Document); !ok {
						t.Error("Init() metadata missing documents")
					}
				}
			}
		})
	}
}

// TestSummaryWorker_InterfaceCompliance verifies interface implementation
func TestSummaryWorker_InterfaceCompliance(t *testing.T) {
	var _ interfaces.DefinitionWorker = (*SummaryWorker)(nil)
}

// TestSummaryWorker_createDocument tests the createDocument method
func TestSummaryWorker_createDocument(t *testing.T) {
	logger := arbor.NewLogger()
	worker := NewSummaryWorker(nil, nil, nil, nil, logger, nil, nil)

	// Test documents
	docs := []*models.Document{
		{ID: "doc_1", Title: "file1.go"},
		{ID: "doc_2", Title: "file2.go"},
	}

	jobDef := &models.JobDefinition{
		ID:   "job-123",
		Name: "My Summary Job",
		Tags: []string{"project", "analysis"},
	}

	doc, err := worker.createDocument(
		context.Background(),
		"# Summary\n\nThis is the generated summary.",
		"Summarize the code architecture",
		docs,
		jobDef,
		"parent-job-id",
		nil,
		nil,
	)

	assert.NoError(t, err)
	assert.NotNil(t, doc)

	// Verify document fields
	assert.Contains(t, doc.ID, "doc_")
	assert.Equal(t, "Summary: My Summary Job", doc.Title)
	assert.Equal(t, "summary", doc.SourceType)
	assert.Equal(t, "# Summary\n\nThis is the generated summary.", doc.ContentMarkdown)
	assert.Equal(t, models.DetailLevelFull, doc.DetailLevel)

	// Verify tags
	assert.Contains(t, doc.Tags, "summary")
	assert.Contains(t, doc.Tags, "my-summary-job") // sanitized job name
	assert.Contains(t, doc.Tags, "project")
	assert.Contains(t, doc.Tags, "analysis")

	// Verify metadata
	assert.Equal(t, "Summarize the code architecture", doc.Metadata["prompt"])
	assert.Equal(t, 2, doc.Metadata["source_document_count"])
	assert.Equal(t, "parent-job-id", doc.Metadata["parent_job_id"])
	assert.Equal(t, "My Summary Job", doc.Metadata["job_name"])
	assert.Equal(t, "job-123", doc.Metadata["job_id"])
	assert.ElementsMatch(t, []string{"doc_1", "doc_2"}, doc.Metadata["source_document_ids"])
}

// TestSummaryWorker_createDocument_WithoutJobDef tests createDocument without job definition
func TestSummaryWorker_createDocument_WithoutJobDef(t *testing.T) {
	logger := arbor.NewLogger()
	worker := NewSummaryWorker(nil, nil, nil, nil, logger, nil, nil)

	docs := []*models.Document{
		{ID: "doc_1", Title: "file1.go"},
	}

	doc, err := worker.createDocument(
		context.Background(),
		"# Summary content",
		"Test prompt",
		docs,
		nil,
		"parent-id",
		nil,
		nil,
	)

	assert.NoError(t, err)
	assert.NotNil(t, doc)
	assert.Equal(t, "Summary", doc.Title)
	assert.Contains(t, doc.Tags, "summary")
	assert.NotContains(t, doc.Metadata, "job_name")
}

// TestNewSummaryWorker tests the constructor
func TestNewSummaryWorker(t *testing.T) {
	logger := arbor.NewLogger()
	mockSearch := new(MockSearchService)
	mockKV := new(MockKVStorage)

	worker := NewSummaryWorker(mockSearch, nil, nil, mockKV, logger, nil, nil)

	assert.NotNil(t, worker)
	assert.Equal(t, mockSearch, worker.searchService)
	assert.Equal(t, mockKV, worker.kvStorage)
	assert.Equal(t, logger, worker.logger)
}

// Note: TestSummaryWorker_InitWithAPIKeyPlaceholder removed
// API key resolution is now handled by provider factory in CreateJobs, not Init
