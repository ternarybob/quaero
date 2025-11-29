package jobs

import (
	"testing"
)

func TestParseTOML_StepFormat(t *testing.T) {
	tomlContent := `
id = "places-nearby-restaurants"
name = "Nearby Restaurants"
type = "places"
job_type = "user"
description = "Search for restaurants"

[step.search_nearby_restaurants]
action = "places_search"
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

	jobDef := jobFile.ToJobDefinition()

	if len(jobDef.Steps) != 1 {
		t.Fatalf("Expected 1 step, got %d", len(jobDef.Steps))
	}

	step := jobDef.Steps[0]
	if step.Name != "search_nearby_restaurants" {
		t.Errorf("Expected step name 'search_nearby_restaurants', got '%s'", step.Name)
	}

	if step.Action != "places_search" {
		t.Errorf("Expected action 'places_search', got '%s'", step.Action)
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
