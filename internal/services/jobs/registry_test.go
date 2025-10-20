// -----------------------------------------------------------------------
// Last Modified: Monday, 21st October 2025 5:50:00 pm
// Modified By: Claude Code
// -----------------------------------------------------------------------

package jobs

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/models"
)

// Test helper - createTestRegistry creates a registry for testing
func createTestRegistry() *JobTypeRegistry {
	logger := arbor.NewLogger()
	return NewJobTypeRegistry(logger)
}

// Test helper - mockActionHandler is a mock action handler for testing
func mockActionHandler(ctx context.Context, step models.JobStep, sources []*models.SourceConfig) error {
	return nil
}

// Test helper - mockFailingActionHandler is a mock action handler that returns an error
func mockFailingActionHandler(ctx context.Context, step models.JobStep, sources []*models.SourceConfig) error {
	return fmt.Errorf("mock action failed")
}

// TestNewJobTypeRegistry tests registry initialization
func TestNewJobTypeRegistry(t *testing.T) {
	registry := createTestRegistry()

	if registry == nil {
		t.Fatal("Expected non-nil registry")
	}

	if registry.actions == nil {
		t.Error("Expected actions map to be initialized")
	}

	if registry.logger == nil {
		t.Error("Expected logger to be set")
	}
}

// TestRegisterAction tests action registration
func TestRegisterAction(t *testing.T) {
	tests := []struct {
		name        string
		jobType     models.JobType
		actionName  string
		handler     ActionHandler
		setupFunc   func(*JobTypeRegistry) // Optional setup before test
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid registration",
			jobType:     models.JobTypeCrawler,
			actionName:  "crawl",
			handler:     mockActionHandler,
			expectError: false,
		},
		{
			name:       "multiple actions for same type",
			jobType:    models.JobTypeCrawler,
			actionName: "transform",
			handler:    mockActionHandler,
			setupFunc: func(r *JobTypeRegistry) {
				r.RegisterAction(models.JobTypeCrawler, "crawl", mockActionHandler)
			},
			expectError: false,
		},
		{
			name:       "different job types",
			jobType:    models.JobTypeSummarizer,
			actionName: "scan",
			handler:    mockActionHandler,
			setupFunc: func(r *JobTypeRegistry) {
				r.RegisterAction(models.JobTypeCrawler, "crawl", mockActionHandler)
			},
			expectError: false,
		},
		{
			name:        "empty action name",
			jobType:     models.JobTypeCrawler,
			actionName:  "",
			handler:     mockActionHandler,
			expectError: true,
			errorMsg:    "action name cannot be empty",
		},
		{
			name:        "nil handler",
			jobType:     models.JobTypeCrawler,
			actionName:  "crawl",
			handler:     nil,
			expectError: true,
			errorMsg:    "handler cannot be nil",
		},
		{
			name:        "invalid job type",
			jobType:     models.JobType("invalid"),
			actionName:  "crawl",
			handler:     mockActionHandler,
			expectError: true,
			errorMsg:    "invalid job type",
		},
		{
			name:       "duplicate registration",
			jobType:    models.JobTypeCrawler,
			actionName: "crawl",
			handler:    mockActionHandler,
			setupFunc: func(r *JobTypeRegistry) {
				r.RegisterAction(models.JobTypeCrawler, "crawl", mockActionHandler)
			},
			expectError: true,
			errorMsg:    "already registered",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := createTestRegistry()

			// Run setup if provided
			if tt.setupFunc != nil {
				tt.setupFunc(registry)
			}

			// Call RegisterAction
			err := registry.RegisterAction(tt.jobType, tt.actionName, tt.handler)

			// Check error expectation
			if tt.expectError && err == nil {
				t.Errorf("Expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
			if tt.expectError && err != nil && tt.errorMsg != "" {
				// Check error message contains expected text
				if !contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error to contain %q, got: %v", tt.errorMsg, err)
				}
			}

			// If success expected, verify action was registered
			if !tt.expectError {
				handler, err := registry.GetAction(tt.jobType, tt.actionName)
				if err != nil {
					t.Errorf("Failed to retrieve registered action: %v", err)
				}
				if handler == nil {
					t.Error("Expected non-nil handler")
				}
			}
		})
	}
}

// TestGetAction tests action retrieval
func TestGetAction(t *testing.T) {
	// Setup: Create registry and register several actions
	registry := createTestRegistry()
	registry.RegisterAction(models.JobTypeCrawler, "crawl", mockActionHandler)
	registry.RegisterAction(models.JobTypeCrawler, "transform", mockActionHandler)
	registry.RegisterAction(models.JobTypeSummarizer, "scan", mockActionHandler)
	registry.RegisterAction(models.JobTypeSummarizer, "summarize", mockActionHandler)

	tests := []struct {
		name        string
		jobType     models.JobType
		actionName  string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "existing action",
			jobType:     models.JobTypeCrawler,
			actionName:  "crawl",
			expectError: false,
		},
		{
			name:        "non-existent action",
			jobType:     models.JobTypeCrawler,
			actionName:  "invalid",
			expectError: true,
			errorMsg:    "not found",
		},
		{
			name:        "non-existent job type",
			jobType:     models.JobTypeCustom,
			actionName:  "crawl",
			expectError: true,
			errorMsg:    "no actions registered",
		},
		{
			name:        "cross-type lookup",
			jobType:     models.JobTypeCrawler,
			actionName:  "scan",
			expectError: true,
			errorMsg:    "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, err := registry.GetAction(tt.jobType, tt.actionName)

			// Check error expectation
			if tt.expectError && err == nil {
				t.Errorf("Expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
			if tt.expectError && err != nil && tt.errorMsg != "" {
				if !contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error to contain %q, got: %v", tt.errorMsg, err)
				}
			}

			// If success expected, assert handler is not nil and can be called
			if !tt.expectError {
				if handler == nil {
					t.Error("Expected non-nil handler")
				} else {
					// Test that handler can be called
					ctx := context.Background()
					step := models.JobStep{Name: "test", Action: tt.actionName}
					sources := []*models.SourceConfig{}
					err := handler(ctx, step, sources)
					if err != nil {
						t.Errorf("Handler execution failed: %v", err)
					}
				}
			}
		})
	}
}

// TestListActions tests listing actions for a job type
func TestListActions(t *testing.T) {
	tests := []struct {
		name         string
		setupFunc    func(*JobTypeRegistry)
		jobType      models.JobType
		expectedList []string
	}{
		{
			name:         "empty registry",
			setupFunc:    func(r *JobTypeRegistry) {},
			jobType:      models.JobTypeCrawler,
			expectedList: []string{},
		},
		{
			name: "single action",
			setupFunc: func(r *JobTypeRegistry) {
				r.RegisterAction(models.JobTypeCrawler, "crawl", mockActionHandler)
			},
			jobType:      models.JobTypeCrawler,
			expectedList: []string{"crawl"},
		},
		{
			name: "multiple actions",
			setupFunc: func(r *JobTypeRegistry) {
				r.RegisterAction(models.JobTypeCrawler, "crawl", mockActionHandler)
				r.RegisterAction(models.JobTypeCrawler, "transform", mockActionHandler)
				r.RegisterAction(models.JobTypeCrawler, "embed", mockActionHandler)
			},
			jobType:      models.JobTypeCrawler,
			expectedList: []string{"crawl", "embed", "transform"}, // Sorted
		},
		{
			name: "non-existent job type",
			setupFunc: func(r *JobTypeRegistry) {
				r.RegisterAction(models.JobTypeCrawler, "crawl", mockActionHandler)
			},
			jobType:      models.JobTypeSummarizer,
			expectedList: []string{},
		},
		{
			name: "multiple job types",
			setupFunc: func(r *JobTypeRegistry) {
				r.RegisterAction(models.JobTypeCrawler, "crawl", mockActionHandler)
				r.RegisterAction(models.JobTypeCrawler, "transform", mockActionHandler)
				r.RegisterAction(models.JobTypeSummarizer, "scan", mockActionHandler)
				r.RegisterAction(models.JobTypeSummarizer, "summarize", mockActionHandler)
			},
			jobType:      models.JobTypeCrawler,
			expectedList: []string{"crawl", "transform"}, // Only crawler actions
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := createTestRegistry()
			tt.setupFunc(registry)

			actions := registry.ListActions(tt.jobType)

			// Check length
			if len(actions) != len(tt.expectedList) {
				t.Errorf("Expected %d actions, got %d", len(tt.expectedList), len(actions))
			}

			// Check contents
			for i, expected := range tt.expectedList {
				if i >= len(actions) {
					t.Errorf("Missing action at index %d: expected %q", i, expected)
					continue
				}
				if actions[i] != expected {
					t.Errorf("At index %d: expected %q, got %q", i, expected, actions[i])
				}
			}

			// Verify alphabetical sorting (if multiple actions)
			if len(actions) > 1 {
				for i := 1; i < len(actions); i++ {
					if actions[i-1] > actions[i] {
						t.Errorf("Actions not sorted: %q comes after %q", actions[i-1], actions[i])
					}
				}
			}
		})
	}
}

// TestGetAllJobTypes tests retrieving all registered job types
func TestGetAllJobTypes(t *testing.T) {
	t.Run("empty registry", func(t *testing.T) {
		registry := createTestRegistry()
		jobTypes := registry.GetAllJobTypes()

		if len(jobTypes) != 0 {
			t.Errorf("Expected empty slice, got %d job types", len(jobTypes))
		}
	})

	t.Run("multiple job types", func(t *testing.T) {
		registry := createTestRegistry()
		registry.RegisterAction(models.JobTypeCrawler, "crawl", mockActionHandler)
		registry.RegisterAction(models.JobTypeSummarizer, "scan", mockActionHandler)

		jobTypes := registry.GetAllJobTypes()

		if len(jobTypes) != 2 {
			t.Errorf("Expected 2 job types, got %d", len(jobTypes))
		}

		// Check both types are present
		foundCrawler := false
		foundSummarizer := false
		for _, jt := range jobTypes {
			if jt == models.JobTypeCrawler {
				foundCrawler = true
			}
			if jt == models.JobTypeSummarizer {
				foundSummarizer = true
			}
		}

		if !foundCrawler {
			t.Error("Expected to find JobTypeCrawler")
		}
		if !foundSummarizer {
			t.Error("Expected to find JobTypeSummarizer")
		}
	})
}

// TestConcurrentAccess tests thread safety of registry operations
func TestConcurrentAccess(t *testing.T) {
	registry := createTestRegistry()

	// Register initial actions
	registry.RegisterAction(models.JobTypeCrawler, "crawl", mockActionHandler)
	registry.RegisterAction(models.JobTypeSummarizer, "scan", mockActionHandler)

	var wg sync.WaitGroup
	// Buffered channel to collect errors from goroutines
	errChan := make(chan error, 15) // 10 read + 5 write goroutines

	// Launch multiple goroutines for read operations
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			// GetAction (read operation)
			_, err := registry.GetAction(models.JobTypeCrawler, "crawl")
			if err != nil {
				errChan <- fmt.Errorf("concurrent GetAction failed: %v", err)
			}
			// ListActions (read operation)
			_ = registry.ListActions(models.JobTypeCrawler)
		}(i)
	}

	// Launch goroutines for write operations
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			// RegisterAction (write operation) with unique action names
			actionName := fmt.Sprintf("action_%d", idx)
			err := registry.RegisterAction(models.JobTypeCrawler, actionName, mockActionHandler)
			if err != nil {
				errChan <- fmt.Errorf("concurrent RegisterAction failed: %v", err)
			}
		}(i)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(errChan)

	// Report any collected errors
	for err := range errChan {
		t.Error(err)
	}

	// Verify final state: all registered actions are retrievable
	actions := registry.ListActions(models.JobTypeCrawler)
	expectedMinCount := 6 // 1 initial + 5 concurrent registrations
	if len(actions) < expectedMinCount {
		t.Errorf("Expected at least %d actions after concurrent writes, got %d", expectedMinCount, len(actions))
	}
}

// TestActionHandlerExecution tests that registered handlers can be retrieved and executed
func TestActionHandlerExecution(t *testing.T) {
	t.Run("successful handler execution", func(t *testing.T) {
		registry := createTestRegistry()
		registry.RegisterAction(models.JobTypeCrawler, "crawl", mockActionHandler)

		handler, err := registry.GetAction(models.JobTypeCrawler, "crawl")
		if err != nil {
			t.Fatalf("Failed to get action: %v", err)
		}

		// Execute handler
		ctx := context.Background()
		step := models.JobStep{Name: "test", Action: "crawl"}
		sources := []*models.SourceConfig{}
		err = handler(ctx, step, sources)

		if err != nil {
			t.Errorf("Handler execution failed: %v", err)
		}
	})

	t.Run("failing handler execution", func(t *testing.T) {
		registry := createTestRegistry()
		registry.RegisterAction(models.JobTypeCrawler, "fail", mockFailingActionHandler)

		handler, err := registry.GetAction(models.JobTypeCrawler, "fail")
		if err != nil {
			t.Fatalf("Failed to get action: %v", err)
		}

		// Execute failing handler
		ctx := context.Background()
		step := models.JobStep{Name: "test", Action: "fail"}
		sources := []*models.SourceConfig{}
		err = handler(ctx, step, sources)

		if err == nil {
			t.Error("Expected error from failing handler, got nil")
		}
	})
}

// TestRegistryIsolation tests that multiple registry instances are independent
func TestRegistryIsolation(t *testing.T) {
	registry1 := createTestRegistry()
	registry2 := createTestRegistry()

	// Register action in registry1 only
	err := registry1.RegisterAction(models.JobTypeCrawler, "crawl", mockActionHandler)
	if err != nil {
		t.Fatalf("Failed to register action in registry1: %v", err)
	}

	// Verify registry1 has the action
	_, err = registry1.GetAction(models.JobTypeCrawler, "crawl")
	if err != nil {
		t.Errorf("registry1 should have action: %v", err)
	}

	// Verify registry2 does not have the action
	_, err = registry2.GetAction(models.JobTypeCrawler, "crawl")
	if err == nil {
		t.Error("registry2 should not have action, but GetAction succeeded")
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
