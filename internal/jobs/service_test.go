package jobs

import (
	"testing"

	"github.com/ternarybob/quaero/internal/models"
)

func TestParseTOML_StepFormat(t *testing.T) {
	tomlContent := `
id = "places-nearby-restaurants"
name = "Nearby Restaurants"
type = "places"
job_type = "user"
description = "Search for restaurants"

[step.search_nearby_restaurants]
type = "places_search"
on_error = "fail"
search_query = "restaurants near Wheelers Hill"
search_type = "nearby_search"
max_results = 20
`

	jobFile, err := ParseTOML([]byte(tomlContent))
	if err != nil {
		t.Fatalf("Failed to parse TOML: %v", err)
	}

	// Debug: Check what's in the parsed jobFile
	t.Logf("Parsed jobFile.Step: %+v", jobFile.Step)
	for name, stepData := range jobFile.Step {
		t.Logf("Step %s: data=%+v", name, stepData)
	}

	jobDef, err := jobFile.ToJobDefinition()
	if err != nil {
		t.Fatalf("Failed to convert to JobDefinition: %v", err)
	}

	if len(jobDef.Steps) != 1 {
		t.Fatalf("Expected 1 step, got %d", len(jobDef.Steps))
	}

	step := jobDef.Steps[0]
	if step.Name != "search_nearby_restaurants" {
		t.Errorf("Expected step name 'search_nearby_restaurants', got '%s'", step.Name)
	}

	if step.Type != models.StepTypePlacesSearch {
		t.Errorf("Expected type 'places_search', got '%s'", step.Type)
	}

	// Check that search_query is in the config
	searchQuery, ok := step.Config["search_query"].(string)
	if !ok {
		t.Errorf("search_query not found in step config, config: %+v", step.Config)
	} else if searchQuery != "restaurants near Wheelers Hill" {
		t.Errorf("Expected search_query 'restaurants near Wheelers Hill', got '%s'", searchQuery)
	}

	// Check max_results
	maxResults, ok := step.Config["max_results"]
	if !ok {
		t.Errorf("max_results not found in step config")
	} else {
		t.Logf("max_results type: %T, value: %v", maxResults, maxResults)
	}
}

// TestParseTOML_WithTypeField tests parsing with new 'type' field
func TestParseTOML_WithTypeField(t *testing.T) {
	tomlContent := `
id = "test-agent-job"
name = "Test Agent Job"
type = "agent"
job_type = "user"
description = "Test agent job with type field"

[step.process_documents]
type = "agent"
description = "Process documents with AI agent"
on_error = "continue"
agent_type = "keyword_extractor"
filter_tags = ["technical"]
`

	jobFile, err := ParseTOML([]byte(tomlContent))
	if err != nil {
		t.Fatalf("Failed to parse TOML: %v", err)
	}

	jobDef, err := jobFile.ToJobDefinition()
	if err != nil {
		t.Fatalf("Failed to convert to JobDefinition: %v", err)
	}

	if len(jobDef.Steps) != 1 {
		t.Fatalf("Expected 1 step, got %d", len(jobDef.Steps))
	}

	step := jobDef.Steps[0]

	// Verify Type field is set correctly
	if step.Type != models.StepTypeAgent {
		t.Errorf("Expected step type 'agent', got '%s'", step.Type)
	}

	// Verify Description field is set
	if step.Description != "Process documents with AI agent" {
		t.Errorf("Expected description 'Process documents with AI agent', got '%s'", step.Description)
	}

	// Verify config fields are parsed correctly
	agentType, ok := step.Config["agent_type"].(string)
	if !ok || agentType != "keyword_extractor" {
		t.Errorf("Expected agent_type 'keyword_extractor', got '%v'", step.Config["agent_type"])
	}
}

// TestParseTOML_MissingType tests validation when type field is missing
func TestParseTOML_MissingType(t *testing.T) {
	tomlContent := `
id = "test-job"
name = "Test Job"
type = "custom"
job_type = "user"

[step.test_step]
on_error = "continue"
some_config = "value"
`

	jobFile, err := ParseTOML([]byte(tomlContent))
	if err != nil {
		t.Fatalf("Failed to parse TOML: %v", err)
	}

	_, err = jobFile.ToJobDefinition()
	if err == nil {
		t.Fatal("Expected error when type field is missing, but got success")
	}

	expectedError := "'type' field is required"
	if !contains(err.Error(), expectedError) {
		t.Errorf("Expected error containing '%s', got '%s'", expectedError, err.Error())
	}
}

// TestParseTOML_InvalidType tests validation with invalid type value
func TestParseTOML_InvalidType(t *testing.T) {
	tomlContent := `
id = "test-job"
name = "Test Job"
type = "custom"
job_type = "user"

[step.test_step]
type = "invalid_type"
on_error = "continue"
`

	jobFile, err := ParseTOML([]byte(tomlContent))
	if err != nil {
		t.Fatalf("Failed to parse TOML: %v", err)
	}

	_, err = jobFile.ToJobDefinition()
	if err == nil {
		t.Fatal("Expected error for invalid type, but got success")
	}

	expectedError := "invalid type"
	if !contains(err.Error(), expectedError) {
		t.Errorf("Expected error containing '%s', got '%s'", expectedError, err.Error())
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && indexString(s, substr) >= 0))
}

func indexString(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
