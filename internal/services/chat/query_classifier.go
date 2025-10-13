package chat

import (
	"regexp"
	"strings"
)

// QueryType represents different types of user queries
type QueryType string

const (
	QueryTypeCount          QueryType = "count"          // "how many", "count", "number of"
	QueryTypeStatistics     QueryType = "statistics"     // "statistics", "summary", "overview"
	QueryTypeCrossSource    QueryType = "cross_source"   // Complex queries needing Pointer RAG
	QueryTypeSimpleLookup   QueryType = "simple_lookup"  // Direct fact lookup
	QueryTypeConversational QueryType = "conversational" // General chat
)

// QueryClassification holds the classification result
type QueryClassification struct {
	Type          QueryType
	NeedsCorpus   bool // Whether to inject corpus summary
	MaxDocuments  int  // Recommended max documents
	UsePointerRAG bool // Whether to use Pointer RAG
}

// ClassifyQuery analyzes the query to determine the best retrieval strategy
func ClassifyQuery(query string) *QueryClassification {
	queryLower := strings.ToLower(strings.TrimSpace(query))

	// Count queries: "how many", "count", "number of", "total"
	countPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bhow\s+many\b`),
		regexp.MustCompile(`(?i)\bcount\b`),
		regexp.MustCompile(`(?i)\bnumber\s+of\b`),
		regexp.MustCompile(`(?i)\btotal\b.*\b(issues?|pages?|documents?)\b`),
		regexp.MustCompile(`(?i)\b(how\s+much|what\s+is\s+the\s+(count|total|number))\b`),
	}

	for _, pattern := range countPatterns {
		if pattern.MatchString(queryLower) {
			return &QueryClassification{
				Type:          QueryTypeCount,
				NeedsCorpus:   true,
				MaxDocuments:  1, // Only need corpus summary
				UsePointerRAG: false,
			}
		}
	}

	// Statistics/summary queries: "summary", "overview", "statistics"
	statsPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(summary|overview|statistics|stats)\b`),
		regexp.MustCompile(`(?i)\bwhat\s+(is|are)\s+in\s+the\s+(database|system|knowledge\s+base)\b`),
		regexp.MustCompile(`(?i)\b(show|list|display)\s+(all|everything)\b`),
	}

	for _, pattern := range statsPatterns {
		if pattern.MatchString(queryLower) {
			return &QueryClassification{
				Type:          QueryTypeStatistics,
				NeedsCorpus:   true,
				MaxDocuments:  3, // Corpus + a few examples
				UsePointerRAG: false,
			}
		}
	}

	// Cross-source queries: mentions multiple sources or relationships
	crossSourceIndicators := []string{
		"jira and confluence",
		"confluence and jira",
		"github and jira",
		"related to",
		"connected to",
		"references",
		"mentioned in",
		"links to",
		"bug fix",
		"resolved",
		"implemented",
		"documented",
	}

	for _, indicator := range crossSourceIndicators {
		if strings.Contains(queryLower, indicator) {
			return &QueryClassification{
				Type:          QueryTypeCrossSource,
				NeedsCorpus:   false,
				MaxDocuments:  10, // Pointer RAG needs more documents
				UsePointerRAG: true,
			}
		}
	}

	// Simple lookup: specific issue keys or page titles
	simpleLookupPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b[A-Z]+-\d+\b`),                    // PROJ-123
		regexp.MustCompile(`(?i)\b(issue|ticket|page)\s+\w+-\d+\b`), // "issue PROJ-123"
		regexp.MustCompile(`(?i)\bwhat\s+is\s+\w+-\d+\b`),           // "what is PROJ-123"
	}

	for _, pattern := range simpleLookupPatterns {
		if pattern.MatchString(queryLower) {
			return &QueryClassification{
				Type:          QueryTypeSimpleLookup,
				NeedsCorpus:   false,
				MaxDocuments:  5,
				UsePointerRAG: false,
			}
		}
	}

	// Default: conversational
	return &QueryClassification{
		Type:          QueryTypeConversational,
		NeedsCorpus:   false,
		MaxDocuments:  5,
		UsePointerRAG: false,
	}
}
