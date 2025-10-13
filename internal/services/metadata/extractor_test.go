package metadata

import (
	"testing"

	"github.com/ternarybob/quaero/internal/models"
)

func TestExtractor_ExtractMetadata(t *testing.T) {
	t.Run("Extract Jira issue keys", func(t *testing.T) {
		extractor := NewExtractor(nil)

		doc := &models.Document{
			Title:   "Fix bug in PROJ-123",
			Content: "This addresses PROJ-123 and is related to PROJ-456",
		}

		metadata, err := extractor.ExtractMetadata(doc)
		if err != nil {
			t.Fatalf("ExtractMetadata failed: %v", err)
		}

		issueKeys, ok := metadata["issue_keys"].([]string)
		if !ok {
			t.Fatal("Expected issue_keys to be []string")
		}

		if len(issueKeys) != 2 {
			t.Errorf("Expected 2 issue keys, got %d: %v", len(issueKeys), issueKeys)
		}

		expectedKeys := map[string]bool{"PROJ-123": true, "PROJ-456": true}
		for _, key := range issueKeys {
			if !expectedKeys[key] {
				t.Errorf("Unexpected issue key: %s", key)
			}
		}
	})

	t.Run("Extract user mentions", func(t *testing.T) {
		extractor := NewExtractor(nil)

		doc := &models.Document{
			Content: "Please review @alice and cc @bob for approval",
		}

		metadata, err := extractor.ExtractMetadata(doc)
		if err != nil {
			t.Fatalf("ExtractMetadata failed: %v", err)
		}

		mentions, ok := metadata["mentions"].([]string)
		if !ok {
			t.Fatal("Expected mentions to be []string")
		}

		if len(mentions) != 2 {
			t.Errorf("Expected 2 mentions, got %d: %v", len(mentions), mentions)
		}

		expectedMentions := map[string]bool{"@alice": true, "@bob": true}
		for _, mention := range mentions {
			if !expectedMentions[mention] {
				t.Errorf("Unexpected mention: %s", mention)
			}
		}
	})

	t.Run("Extract PR references", func(t *testing.T) {
		extractor := NewExtractor(nil)

		doc := &models.Document{
			Content: "Fixes #123 and implements #456",
		}

		metadata, err := extractor.ExtractMetadata(doc)
		if err != nil {
			t.Fatalf("ExtractMetadata failed: %v", err)
		}

		prRefs, ok := metadata["pr_refs"].([]string)
		if !ok {
			t.Fatal("Expected pr_refs to be []string")
		}

		if len(prRefs) != 2 {
			t.Errorf("Expected 2 PR refs, got %d: %v", len(prRefs), prRefs)
		}

		expectedPRs := map[string]bool{"#123": true, "#456": true}
		for _, pr := range prRefs {
			if !expectedPRs[pr] {
				t.Errorf("Unexpected PR ref: %s", pr)
			}
		}
	})

	t.Run("Extract from both title and content", func(t *testing.T) {
		extractor := NewExtractor(nil)

		doc := &models.Document{
			Title:   "PROJ-100: Important fix",
			Content: "Also fixes PROJ-200",
		}

		metadata, err := extractor.ExtractMetadata(doc)
		if err != nil {
			t.Fatalf("ExtractMetadata failed: %v", err)
		}

		issueKeys, ok := metadata["issue_keys"].([]string)
		if !ok {
			t.Fatal("Expected issue_keys to be []string")
		}

		if len(issueKeys) != 2 {
			t.Errorf("Expected 2 issue keys, got %d: %v", len(issueKeys), issueKeys)
		}
	})

	t.Run("No duplicates in extracted metadata", func(t *testing.T) {
		extractor := NewExtractor(nil)

		doc := &models.Document{
			Title:   "PROJ-123 PROJ-123",
			Content: "PROJ-123 mentioned again",
		}

		metadata, err := extractor.ExtractMetadata(doc)
		if err != nil {
			t.Fatalf("ExtractMetadata failed: %v", err)
		}

		issueKeys, ok := metadata["issue_keys"].([]string)
		if !ok {
			t.Fatal("Expected issue_keys to be []string")
		}

		if len(issueKeys) != 1 {
			t.Errorf("Expected 1 unique issue key, got %d: %v", len(issueKeys), issueKeys)
		}

		if issueKeys[0] != "PROJ-123" {
			t.Errorf("Expected PROJ-123, got %s", issueKeys[0])
		}
	})

	t.Run("Empty document returns empty metadata", func(t *testing.T) {
		extractor := NewExtractor(nil)

		doc := &models.Document{
			Title:   "",
			Content: "",
		}

		metadata, err := extractor.ExtractMetadata(doc)
		if err != nil {
			t.Fatalf("ExtractMetadata failed: %v", err)
		}

		if len(metadata) != 0 {
			t.Errorf("Expected empty metadata, got: %v", metadata)
		}
	})

	t.Run("Document with no patterns returns empty metadata", func(t *testing.T) {
		extractor := NewExtractor(nil)

		doc := &models.Document{
			Title:   "Just a regular title",
			Content: "Regular content with no special patterns",
		}

		metadata, err := extractor.ExtractMetadata(doc)
		if err != nil {
			t.Fatalf("ExtractMetadata failed: %v", err)
		}

		if len(metadata) != 0 {
			t.Errorf("Expected empty metadata, got: %v", metadata)
		}
	})

	t.Run("Extract Confluence page references", func(t *testing.T) {
		extractor := NewExtractor(nil)

		doc := &models.Document{
			Content: "See page:123456 and page:789012 for details",
		}

		metadata, err := extractor.ExtractMetadata(doc)
		if err != nil {
			t.Fatalf("ExtractMetadata failed: %v", err)
		}

		pageRefs, ok := metadata["confluence_pages"].([]string)
		if !ok {
			t.Fatal("Expected confluence_pages to be []string")
		}

		if len(pageRefs) != 2 {
			t.Errorf("Expected 2 page refs, got %d: %v", len(pageRefs), pageRefs)
		}
	})
}

func TestExtractor_MergeMetadata(t *testing.T) {
	t.Run("Merge extracted metadata with existing metadata", func(t *testing.T) {
		extractor := NewExtractor(nil)

		existing := map[string]interface{}{
			"project":  "PROJ",
			"priority": "high",
		}

		extracted := map[string]interface{}{
			"issue_keys": []string{"PROJ-123"},
			"mentions":   []string{"@alice"},
		}

		merged := extractor.MergeMetadata(existing, extracted)

		// Check existing metadata preserved
		if merged["project"] != "PROJ" {
			t.Error("Expected existing project metadata preserved")
		}

		// Check extracted metadata added
		if _, ok := merged["issue_keys"]; !ok {
			t.Error("Expected extracted issue_keys in merged metadata")
		}

		// Should have 4 keys total
		if len(merged) != 4 {
			t.Errorf("Expected 4 metadata fields, got %d", len(merged))
		}
	})

	t.Run("Extracted metadata overwrites existing", func(t *testing.T) {
		extractor := NewExtractor(nil)

		existing := map[string]interface{}{
			"issue_keys": []string{"OLD-123"},
		}

		extracted := map[string]interface{}{
			"issue_keys": []string{"NEW-456"},
		}

		merged := extractor.MergeMetadata(existing, extracted)

		issueKeys, ok := merged["issue_keys"].([]string)
		if !ok {
			t.Fatal("Expected issue_keys to be []string")
		}

		if len(issueKeys) != 1 || issueKeys[0] != "NEW-456" {
			t.Errorf("Expected extracted metadata to overwrite existing, got: %v", issueKeys)
		}
	})

	t.Run("Merge with nil existing metadata", func(t *testing.T) {
		extractor := NewExtractor(nil)

		extracted := map[string]interface{}{
			"issue_keys": []string{"PROJ-123"},
		}

		merged := extractor.MergeMetadata(nil, extracted)

		if len(merged) != 1 {
			t.Errorf("Expected 1 metadata field, got %d", len(merged))
		}

		if _, ok := merged["issue_keys"]; !ok {
			t.Error("Expected issue_keys in merged metadata")
		}
	})

	t.Run("Merge with nil extracted metadata", func(t *testing.T) {
		extractor := NewExtractor(nil)

		existing := map[string]interface{}{
			"project": "PROJ",
		}

		merged := extractor.MergeMetadata(existing, nil)

		if len(merged) != 1 {
			t.Errorf("Expected 1 metadata field, got %d", len(merged))
		}

		if merged["project"] != "PROJ" {
			t.Error("Expected existing metadata preserved")
		}
	})
}

func TestExtractor_Patterns(t *testing.T) {
	t.Run("Jira issue key pattern variations", func(t *testing.T) {
		extractor := NewExtractor(nil)

		testCases := []struct {
			content  string
			expected []string
		}{
			{"PROJ-123", []string{"PROJ-123"}},
			{"BUG-1", []string{"BUG-1"}},
			{"FEATURE-999999", []string{"FEATURE-999999"}},
			{"ABC-1 DEF-2 GHI-3", []string{"ABC-1", "DEF-2", "GHI-3"}},
		}

		for _, tc := range testCases {
			doc := &models.Document{Content: tc.content}
			metadata, err := extractor.ExtractMetadata(doc)
			if err != nil {
				t.Fatalf("ExtractMetadata failed for %q: %v", tc.content, err)
			}

			issueKeys, ok := metadata["issue_keys"].([]string)
			if !ok {
				t.Fatalf("Expected issue_keys for %q", tc.content)
			}

			if len(issueKeys) != len(tc.expected) {
				t.Errorf("For %q: expected %d keys, got %d: %v",
					tc.content, len(tc.expected), len(issueKeys), issueKeys)
			}
		}
	})

	t.Run("User mention pattern variations", func(t *testing.T) {
		extractor := NewExtractor(nil)

		testCases := []struct {
			content  string
			expected []string
		}{
			{"@alice", []string{"@alice"}},
			{"@john_doe", []string{"@john_doe"}},
			{"@user123", []string{"@user123"}},
			{"@a @b @c", []string{"@a", "@b", "@c"}},
		}

		for _, tc := range testCases {
			doc := &models.Document{Content: tc.content}
			metadata, err := extractor.ExtractMetadata(doc)
			if err != nil {
				t.Fatalf("ExtractMetadata failed for %q: %v", tc.content, err)
			}

			mentions, ok := metadata["mentions"].([]string)
			if !ok {
				t.Fatalf("Expected mentions for %q", tc.content)
			}

			if len(mentions) != len(tc.expected) {
				t.Errorf("For %q: expected %d mentions, got %d: %v",
					tc.content, len(tc.expected), len(mentions), mentions)
			}
		}
	})
}
