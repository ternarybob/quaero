package search

import (
	"testing"
)

func TestMatchesMetadata_NestedKey(t *testing.T) {
	// Test nested key access with dot notation
	metadata := map[string]interface{}{
		"rule_classifier": map[string]interface{}{
			"category":    "source",
			"subcategory": "implementation",
		},
		"flat_key": "flat_value",
	}

	tests := []struct {
		name    string
		filters map[string]string
		want    bool
	}{
		{
			name:    "nested key match",
			filters: map[string]string{"rule_classifier.category": "source"},
			want:    true,
		},
		{
			name:    "nested key no match",
			filters: map[string]string{"rule_classifier.category": "test"},
			want:    false,
		},
		{
			name:    "nested subcategory match",
			filters: map[string]string{"rule_classifier.subcategory": "implementation"},
			want:    true,
		},
		{
			name:    "flat key match",
			filters: map[string]string{"flat_key": "flat_value"},
			want:    true,
		},
		{
			name:    "missing key",
			filters: map[string]string{"nonexistent.key": "value"},
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesMetadata(metadata, tt.filters)
			if got != tt.want {
				t.Errorf("matchesMetadata() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMatchesMetadata_MultiValue(t *testing.T) {
	// Test multi-value matching with comma-separated values
	metadata := map[string]interface{}{
		"rule_classifier": map[string]interface{}{
			"category": "build",
		},
	}

	tests := []struct {
		name    string
		filters map[string]string
		want    bool
	}{
		{
			name:    "single value match",
			filters: map[string]string{"rule_classifier.category": "build"},
			want:    true,
		},
		{
			name:    "multi-value first match",
			filters: map[string]string{"rule_classifier.category": "build,config,docs"},
			want:    true,
		},
		{
			name:    "multi-value middle match",
			filters: map[string]string{"rule_classifier.category": "source,build,test"},
			want:    true,
		},
		{
			name:    "multi-value no match",
			filters: map[string]string{"rule_classifier.category": "source,test,ci"},
			want:    false,
		},
		{
			name:    "multi-value with spaces",
			filters: map[string]string{"rule_classifier.category": "source, build, test"},
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesMetadata(metadata, tt.filters)
			if got != tt.want {
				t.Errorf("matchesMetadata() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetNestedValue(t *testing.T) {
	data := map[string]interface{}{
		"level1": map[string]interface{}{
			"level2": map[string]interface{}{
				"level3": "deep_value",
			},
			"value": "level2_value",
		},
		"flat": "flat_value",
	}

	tests := []struct {
		name string
		key  string
		want interface{}
	}{
		{"flat key", "flat", "flat_value"},
		{"one level deep", "level1.value", "level2_value"},
		{"two levels deep", "level1.level2.level3", "deep_value"},
		{"missing key", "nonexistent", nil},
		{"missing nested key", "level1.nonexistent", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getNestedValue(data, tt.key)
			if got != tt.want {
				t.Errorf("getNestedValue() = %v, want %v", got, tt.want)
			}
		})
	}
}
