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

	// Financial/market data workers (market_ prefix)
	WorkerTypeMarketAnnouncements    WorkerType = "market_announcements"     // Company announcements with classification
	WorkerTypeMarketData             WorkerType = "market_data"              // Price data, technicals, indices via EODHD
	WorkerTypeMarketNews             WorkerType = "market_news"              // Multi-exchange news via EODHD News API
	WorkerTypeMarketDirectorInterest WorkerType = "market_director_interest" // Director interest (Appendix 3Y) filings
	WorkerTypeMarketFundamentals     WorkerType = "market_fundamentals"      // Price, analyst coverage, financials via EODHD
	WorkerTypeMarketMacro            WorkerType = "market_macro"             // Macroeconomic data (RBA rates, commodities)
	WorkerTypeMarketCompetitor       WorkerType = "market_competitor"        // Competitor stock analysis
	WorkerTypeMarketSignal           WorkerType = "market_signal"            // PBAS, VLI, Regime signals computation
	WorkerTypeMarketPortfolio        WorkerType = "market_portfolio"         // Portfolio-level signal aggregation
	WorkerTypeMarketAssessor         WorkerType = "market_assessor"          // AI-powered stock assessment
	WorkerTypeMarketDataCollection   WorkerType = "market_data_collection"   // Deterministic market data collection
	WorkerTypeMarketConsolidate      WorkerType = "market_consolidate"       // Consolidate tagged documents (no AI)
	WorkerTypeSignalAnalysis         WorkerType = "signal_analysis"          // Announcement signal classification and scoring
	WorkerTypeOutputFormatter        WorkerType = "output_formatter"         // Format output documents for email delivery

	// Navexa portfolio workers
	WorkerTypeNavexaPortfolios      WorkerType = "navexa_portfolios"       // Fetch all Navexa portfolios for the user
	WorkerTypeNavexaPortfolio       WorkerType = "navexa_portfolio"        // Fetch specific portfolio by name with holdings
	WorkerTypeNavexaHoldings        WorkerType = "navexa_holdings"         // Fetch holdings for a Navexa portfolio
	WorkerTypeNavexaPerformance     WorkerType = "navexa_performance"      // Fetch P/L performance for a Navexa portfolio
	WorkerTypeNavexaPortfolioReview WorkerType = "navexa_portfolio_review" // LLM-generated portfolio review from holdings

	// Testing workers
	WorkerTypeTestJobGenerator WorkerType = "test_job_generator" // Generates logs with random errors

	// Email monitoring workers
	WorkerTypeEmailWatcher WorkerType = "email_watcher" // Monitors IMAP inbox for job execution commands

	// Template orchestration workers
	WorkerTypeJobTemplate WorkerType = "job_template" // Executes job templates with variable substitution

	// AI-powered cognitive orchestration workers
	WorkerTypeOrchestrator WorkerType = "orchestrator" // AI agent that dynamically plans and executes steps

	// Rating workers - Investability scoring system
	WorkerTypeRatingBFS       WorkerType = "rating_bfs"       // Business Foundation Score (0-2)
	WorkerTypeRatingCDS       WorkerType = "rating_cds"       // Capital Discipline Score (0-2)
	WorkerTypeRatingNFR       WorkerType = "rating_nfr"       // Narrative-to-Fact Ratio (0-1)
	WorkerTypeRatingPPS       WorkerType = "rating_pps"       // Price Progression Score (0-1)
	WorkerTypeRatingVRS       WorkerType = "rating_vrs"       // Volatility Regime Stability (0-1)
	WorkerTypeRatingOB        WorkerType = "rating_ob"        // Optionality Bonus (0, 0.5, 1)
	WorkerTypeRatingComposite WorkerType = "rating_composite" // Composite investability rating
)

// IsValid checks if the WorkerType is a known, valid type
func (w WorkerType) IsValid() bool {
	switch w {
	case WorkerTypeAgent, WorkerTypeCrawler, WorkerTypePlacesSearch, WorkerTypeWebSearch,
		WorkerTypeGitHubRepo, WorkerTypeGitHubActions, WorkerTypeGitHubGit, WorkerTypeTransform,
		WorkerTypeReindex, WorkerTypeLocalDir, WorkerTypeCodeMap, WorkerTypeSummary,
		WorkerTypeAnalyzeBuild, WorkerTypeClassify, WorkerTypeDependencyGraph,
		WorkerTypeAggregateSummary, WorkerTypeEmail,
		WorkerTypeMarketAnnouncements, WorkerTypeMarketData, WorkerTypeMarketNews,
		WorkerTypeMarketDirectorInterest, WorkerTypeMarketFundamentals, WorkerTypeMarketMacro,
		WorkerTypeMarketCompetitor, WorkerTypeMarketSignal, WorkerTypeMarketPortfolio,
		WorkerTypeMarketAssessor, WorkerTypeMarketDataCollection, WorkerTypeMarketConsolidate,
		WorkerTypeSignalAnalysis, WorkerTypeOutputFormatter,
		WorkerTypeNavexaPortfolios, WorkerTypeNavexaPortfolio, WorkerTypeNavexaHoldings, WorkerTypeNavexaPerformance, WorkerTypeNavexaPortfolioReview,
		WorkerTypeTestJobGenerator, WorkerTypeEmailWatcher, WorkerTypeJobTemplate, WorkerTypeOrchestrator,
		WorkerTypeRatingBFS, WorkerTypeRatingCDS, WorkerTypeRatingNFR, WorkerTypeRatingPPS,
		WorkerTypeRatingVRS, WorkerTypeRatingOB, WorkerTypeRatingComposite:
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
		WorkerTypeMarketAnnouncements,
		WorkerTypeMarketData,
		WorkerTypeMarketNews,
		WorkerTypeMarketDirectorInterest,
		WorkerTypeMarketFundamentals,
		WorkerTypeMarketMacro,
		WorkerTypeMarketCompetitor,
		WorkerTypeMarketSignal,
		WorkerTypeMarketPortfolio,
		WorkerTypeMarketAssessor,
		WorkerTypeMarketDataCollection,
		WorkerTypeMarketConsolidate,
		WorkerTypeSignalAnalysis,
		WorkerTypeOutputFormatter,
		WorkerTypeNavexaPortfolios,
		WorkerTypeNavexaPortfolio,
		WorkerTypeNavexaHoldings,
		WorkerTypeNavexaPerformance,
		WorkerTypeNavexaPortfolioReview,
		WorkerTypeTestJobGenerator,
		WorkerTypeEmailWatcher,
		WorkerTypeJobTemplate,
		WorkerTypeOrchestrator,
		WorkerTypeRatingBFS,
		WorkerTypeRatingCDS,
		WorkerTypeRatingNFR,
		WorkerTypeRatingPPS,
		WorkerTypeRatingVRS,
		WorkerTypeRatingOB,
		WorkerTypeRatingComposite,
	}
}
