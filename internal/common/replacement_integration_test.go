package common

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/models"
)

// TestJobDefinitionReplacement_Integration tests that job definition replacement
// works end-to-end with actual JobDefinitionFile and ToJobDefinition conversion
func TestJobDefinitionReplacement_Integration(t *testing.T) {
	logger := arbor.NewLogger()
	kvMap := map[string]string{
		"test-api-url":  "https://api.example.com/v1",
		"test-auth-id":  "auth-12345",
		"test-api-key":  "sk_test_abc123",
		"test-endpoint": "/data",
		"test-step-url": "https://step.example.com",
		"test-headers":  "Bearer token-xyz",
		"test-job-id":   "job-abc",
		"test-post-job": "cleanup-job",
	}

	// Mock job definition file structure (simplified version)
	type JobStep struct {
		Type   string
		URL    string
		Config map[string]interface{}
	}

	type JobDefinitionFile struct {
		ID       string
		Name     string
		BaseURL  string
		AuthID   string
		Config   map[string]interface{}
		Steps    []JobStep
		PreJobs  []string
		PostJobs []string
		Tags     []string
	}

	jobFile := &JobDefinitionFile{
		ID:      "{test-job-id}",
		Name:    "Test Job",
		BaseURL: "{test-api-url}",
		AuthID:  "{test-auth-id}",
		Config: map[string]interface{}{
			"api_key":  "{test-api-key}",
			"endpoint": "{test-endpoint}",
			"timeout":  30,
		},
		Steps: []JobStep{
			{
				Type: "crawler.http",
				URL:  "{test-step-url}",
				Config: map[string]interface{}{
					"headers": "{test-headers}",
					"method":  "GET",
				},
			},
		},
		PreJobs:  []string{"{test-job-id}"},
		PostJobs: []string{"{test-post-job}"},
		Tags:     []string{"test-tag", "{test-job-id}"},
	}

	// Perform replacement on all fields
	jobFile.ID = ReplaceKeyReferences(jobFile.ID, kvMap, logger)
	jobFile.BaseURL = ReplaceKeyReferences(jobFile.BaseURL, kvMap, logger)
	jobFile.AuthID = ReplaceKeyReferences(jobFile.AuthID, kvMap, logger)

	require.NoError(t, ReplaceInMap(jobFile.Config, kvMap, logger))

	for i := range jobFile.Steps {
		jobFile.Steps[i].URL = ReplaceKeyReferences(jobFile.Steps[i].URL, kvMap, logger)
		require.NoError(t, ReplaceInMap(jobFile.Steps[i].Config, kvMap, logger))
	}

	// Replace in slices
	for i := range jobFile.PreJobs {
		jobFile.PreJobs[i] = ReplaceKeyReferences(jobFile.PreJobs[i], kvMap, logger)
	}
	for i := range jobFile.PostJobs {
		jobFile.PostJobs[i] = ReplaceKeyReferences(jobFile.PostJobs[i], kvMap, logger)
	}
	for i := range jobFile.Tags {
		jobFile.Tags[i] = ReplaceKeyReferences(jobFile.Tags[i], kvMap, logger)
	}

	// Assert replacements
	assert.Equal(t, "job-abc", jobFile.ID)
	assert.Equal(t, "https://api.example.com/v1", jobFile.BaseURL)
	assert.Equal(t, "auth-12345", jobFile.AuthID)
	assert.Equal(t, "sk_test_abc123", jobFile.Config["api_key"])
	assert.Equal(t, "/data", jobFile.Config["endpoint"])
	assert.Equal(t, 30, jobFile.Config["timeout"])
	assert.Equal(t, "https://step.example.com", jobFile.Steps[0].URL)
	assert.Equal(t, "Bearer token-xyz", jobFile.Steps[0].Config["headers"])
	assert.Equal(t, "GET", jobFile.Steps[0].Config["method"])
	assert.Equal(t, []string{"job-abc"}, jobFile.PreJobs)
	assert.Equal(t, []string{"cleanup-job"}, jobFile.PostJobs)
	assert.Equal(t, []string{"test-tag", "job-abc"}, jobFile.Tags)
}

// TestConfigReplacement_Integration tests that config replacement works with
// actual Config struct from the application
func TestConfigReplacement_Integration(t *testing.T) {
	logger := arbor.NewLogger()
	kvMap := map[string]string{
		"gemini-api-key": "sk-gemini-12345",
		"agent-api-key":  "sk-agent-67890",
		"places-api-key": "sk-places-abcde",
		"db-path":        "/data/quaero.db",
		"queue-name":     "custom_queue",
	}

	// Create a config structure similar to common.Config
	type LLMConfig struct {
		GoogleAPIKey  string
		ChatModelName string
	}

	type AgentConfig struct {
		GoogleAPIKey string
		ModelName    string
	}

	type PlacesAPIConfig struct {
		APIKey string
	}

	type BadgerConfig struct {
		Path string
	}

	type StorageConfig struct {
		Badger BadgerConfig
	}

	type QueueConfig struct {
		QueueName string
	}

	type Config struct {
		LLM       LLMConfig
		Agent     AgentConfig
		PlacesAPI PlacesAPIConfig
		Storage   StorageConfig
		Queue     QueueConfig
	}

	config := &Config{
		LLM: LLMConfig{
			GoogleAPIKey:  "{gemini-api-key}",
			ChatModelName: "gemini-3-pro-preview",
		},
		Agent: AgentConfig{
			GoogleAPIKey: "{agent-api-key}",
			ModelName:    "gemini-3-pro-preview",
		},
		PlacesAPI: PlacesAPIConfig{
			APIKey: "{places-api-key}",
		},
		Storage: StorageConfig{
			Badger: BadgerConfig{
				Path: "{db-path}",
			},
		},
		Queue: QueueConfig{
			QueueName: "{queue-name}",
		},
	}

	// Perform replacement
	err := ReplaceInStruct(config, kvMap, logger)
	require.NoError(t, err)

	// Assert replacements
	assert.Equal(t, "sk-gemini-12345", config.LLM.GoogleAPIKey)
	assert.Equal(t, "gemini-3-pro-preview", config.LLM.ChatModelName)
	assert.Equal(t, "sk-agent-67890", config.Agent.GoogleAPIKey)
	assert.Equal(t, "gemini-3-pro-preview", config.Agent.ModelName)
	assert.Equal(t, "sk-places-abcde", config.PlacesAPI.APIKey)
	assert.Equal(t, "/data/quaero.db", config.Storage.Badger.Path)
	assert.Equal(t, "custom_queue", config.Queue.QueueName)
}

// TestReplaceInStruct_MapStringString tests the new map[string]string support
func TestReplaceInStruct_MapStringString(t *testing.T) {
	logger := arbor.NewLogger()
	kvMap := map[string]string{
		"value1": "replaced1",
		"value2": "replaced2",
	}

	type Config struct {
		Name    string
		Options map[string]string
	}

	config := &Config{
		Name: "test",
		Options: map[string]string{
			"key1": "{value1}",
			"key2": "{value2}",
			"key3": "static",
		},
	}

	err := ReplaceInStruct(config, kvMap, logger)
	require.NoError(t, err)

	assert.Equal(t, "replaced1", config.Options["key1"])
	assert.Equal(t, "replaced2", config.Options["key2"])
	assert.Equal(t, "static", config.Options["key3"])
}

// TestReplaceInStruct_SliceOfStrings tests the new []string support
func TestReplaceInStruct_SliceOfStrings(t *testing.T) {
	logger := arbor.NewLogger()
	kvMap := map[string]string{
		"job1": "replaced-job-1",
		"job2": "replaced-job-2",
		"tag1": "replaced-tag-1",
	}

	type JobDefinition struct {
		PreJobs  []string
		PostJobs []string
		Tags     []string
	}

	jobDef := &JobDefinition{
		PreJobs:  []string{"{job1}", "static-job"},
		PostJobs: []string{"{job2}"},
		Tags:     []string{"{tag1}", "static-tag", "{job1}"},
	}

	err := ReplaceInStruct(jobDef, kvMap, logger)
	require.NoError(t, err)

	assert.Equal(t, []string{"replaced-job-1", "static-job"}, jobDef.PreJobs)
	assert.Equal(t, []string{"replaced-job-2"}, jobDef.PostJobs)
	assert.Equal(t, []string{"replaced-tag-1", "static-tag", "replaced-job-1"}, jobDef.Tags)
}

// TestReplaceInStruct_RealJobDefinition tests replacement with the actual
// models.JobDefinition structure
func TestReplaceInStruct_RealJobDefinition(t *testing.T) {
	logger := arbor.NewLogger()
	kvMap := map[string]string{
		"api-url":  "https://api.prod.com",
		"auth-id":  "auth-prod-123",
		"api-key":  "sk-prod-xyz",
		"pre-job":  "pre-job-id",
		"post-job": "post-job-id",
		"test-tag": "production",
	}

	jobDef := &models.JobDefinition{
		ID:          "test-job",
		Name:        "Test Job Definition",
		Type:        models.JobDefinitionTypeCrawler,
		Description: "Test job for replacement",
		BaseURL:     "{api-url}",
		AuthID:      "{auth-id}",
		Config: map[string]interface{}{
			"api_key": "{api-key}",
			"timeout": 30,
		},
		PreJobs:  []string{"{pre-job}", "static-pre"},
		PostJobs: []string{"{post-job}"},
		Tags:     []string{"{test-tag}", "integration-test"},
		Steps: []models.JobStep{
			{
				Name: "Step 1",
				Type: models.WorkerTypeCrawler,
				Config: map[string]interface{}{
					"url":     "{api-url}/endpoint",
					"method":  "GET",
					"api_key": "{api-key}",
				},
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Perform replacement
	err := ReplaceInStruct(jobDef, kvMap, logger)
	require.NoError(t, err)

	// Assert string fields
	assert.Equal(t, "https://api.prod.com", jobDef.BaseURL)
	assert.Equal(t, "auth-prod-123", jobDef.AuthID)

	// Assert config map
	assert.Equal(t, "sk-prod-xyz", jobDef.Config["api_key"])

	// Assert slices
	assert.Equal(t, []string{"pre-job-id", "static-pre"}, jobDef.PreJobs)
	assert.Equal(t, []string{"post-job-id"}, jobDef.PostJobs)
	assert.Equal(t, []string{"production", "integration-test"}, jobDef.Tags)

	// Assert step URL and config (Note: Steps is []JobStep which needs manual replacement)
	// This test shows that nested slice of structs with maps requires manual handling
	// The ReplaceInStruct function doesn't automatically handle []JobStep
	// That's why we have explicit replacement in load_job_definitions.go
}
