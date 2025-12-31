// -----------------------------------------------------------------------
// Last Modified: Friday, 29th November 2025 12:00:00 pm
// Modified By: Claude Code
// -----------------------------------------------------------------------

package models

// WorkerType represents the type of worker that handles a job definition step.
// This provides explicit type-safety for routing steps to the appropriate worker.
type WorkerType string

// WorkerType constants define all supported worker types for job execution
const (
	WorkerTypeAgent         WorkerType = "agent"
	WorkerTypeCrawler       WorkerType = "crawler"
	WorkerTypePlacesSearch  WorkerType = "places_search"
	WorkerTypeWebSearch     WorkerType = "web_search"
	WorkerTypeGitHubRepo    WorkerType = "github_repo"
	WorkerTypeGitHubActions WorkerType = "github_actions"
	WorkerTypeGitHubGit     WorkerType = "github_git" // Clone repository via git instead of API
	WorkerTypeTransform     WorkerType = "transform"
	WorkerTypeReindex       WorkerType = "reindex"
	WorkerTypeLocalDir      WorkerType = "local_dir" // Local directory indexing (full content)
	WorkerTypeCodeMap       WorkerType = "code_map"  // Hierarchical code structure analysis
	WorkerTypeSummary       WorkerType = "summary"   // Corpus summary generation from tagged documents

	// Enrichment pipeline workers - each handles a specific enrichment step
	WorkerTypeAnalyzeBuild     WorkerType = "analyze_build"     // Parse build files (CMake, Makefile) for targets and dependencies
	WorkerTypeClassify         WorkerType = "classify"          // LLM-based classification of file roles and components
	WorkerTypeDependencyGraph  WorkerType = "dependency_graph"  // Build dependency graph from extracted metadata
	WorkerTypeAggregateSummary WorkerType = "aggregate_summary" // Generate summary of all enrichment metadata

	// Notification workers
	WorkerTypeEmail WorkerType = "email" // Send email notification with job results

	// Financial data workers
	WorkerTypeASXAnnouncements        WorkerType = "asx_announcements"         // Fetch ASX company announcements
	WorkerTypeASXStockData            WorkerType = "asx_stock_data"            // Fetch real-time stock prices and technical analysis
	WorkerTypeASXDirectorInterest     WorkerType = "asx_director_interest"     // Fetch ASX director interest (Appendix 3Y) filings
	WorkerTypeASXAnalystCoverage      WorkerType = "asx_analyst_coverage"      // Fetch analyst coverage, price targets, and broker ratings
	WorkerTypeASXHistoricalFinancials WorkerType = "asx_historical_financials" // Fetch historical financial data (revenue, profit, EPS history)
	WorkerTypeMacroData               WorkerType = "macro_data"                // Fetch macroeconomic data (RBA rates, commodity prices)
	WorkerTypeCompetitorAnalysis      WorkerType = "competitor_analysis"       // Analyze competitors and spawn stock data jobs

	// Testing workers
	WorkerTypeTestJobGenerator WorkerType = "test_job_generator" // Generates logs with random errors for testing logging, error tolerance, and job hierarchy

	// Email monitoring workers
	WorkerTypeEmailWatcher WorkerType = "email_watcher" // Monitors IMAP inbox for job execution commands

	// Template orchestration workers
	WorkerTypeJobTemplate WorkerType = "job_template" // Executes job templates with variable substitution

	// AI-powered cognitive orchestration workers
	WorkerTypeOrchestrator WorkerType = "orchestrator" // AI agent that dynamically plans and executes steps using LLM reasoning
)

// IsValid checks if the WorkerType is a known, valid type
func (w WorkerType) IsValid() bool {
	switch w {
	case WorkerTypeAgent, WorkerTypeCrawler, WorkerTypePlacesSearch, WorkerTypeWebSearch,
		WorkerTypeGitHubRepo, WorkerTypeGitHubActions, WorkerTypeGitHubGit, WorkerTypeTransform,
		WorkerTypeReindex, WorkerTypeLocalDir, WorkerTypeCodeMap, WorkerTypeSummary,
		WorkerTypeAnalyzeBuild, WorkerTypeClassify, WorkerTypeDependencyGraph,
		WorkerTypeAggregateSummary, WorkerTypeEmail, WorkerTypeASXAnnouncements,
		WorkerTypeASXStockData, WorkerTypeASXDirectorInterest, WorkerTypeASXAnalystCoverage,
		WorkerTypeASXHistoricalFinancials, WorkerTypeMacroData,
		WorkerTypeCompetitorAnalysis, WorkerTypeTestJobGenerator,
		WorkerTypeEmailWatcher, WorkerTypeJobTemplate, WorkerTypeOrchestrator:
		return true
	}
	return false
}

// String returns the string representation of the WorkerType
func (w WorkerType) String() string {
	return string(w)
}

// AllWorkerTypes returns a slice of all valid WorkerType values
func AllWorkerTypes() []WorkerType {
	return []WorkerType{
		WorkerTypeAgent,
		WorkerTypeCrawler,
		WorkerTypePlacesSearch,
		WorkerTypeWebSearch,
		WorkerTypeGitHubRepo,
		WorkerTypeGitHubActions,
		WorkerTypeGitHubGit,
		WorkerTypeTransform,
		WorkerTypeReindex,
		WorkerTypeLocalDir,
		WorkerTypeCodeMap,
		WorkerTypeSummary,
		WorkerTypeAnalyzeBuild,
		WorkerTypeClassify,
		WorkerTypeDependencyGraph,
		WorkerTypeAggregateSummary,
		WorkerTypeEmail,
		WorkerTypeASXAnnouncements,
		WorkerTypeASXStockData,
		WorkerTypeASXDirectorInterest,
		WorkerTypeASXAnalystCoverage,
		WorkerTypeASXHistoricalFinancials,
		WorkerTypeMacroData,
		WorkerTypeCompetitorAnalysis,
		WorkerTypeTestJobGenerator,
		WorkerTypeEmailWatcher,
		WorkerTypeJobTemplate,
		WorkerTypeOrchestrator,
	}
}
