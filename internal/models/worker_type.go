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
	WorkerTypeAgent               WorkerType = "agent"
	WorkerTypeCrawler             WorkerType = "crawler"
	WorkerTypePlacesSearch        WorkerType = "places_search"
	WorkerTypeWebSearch           WorkerType = "web_search"
	WorkerTypeGitHubRepo          WorkerType = "github_repo"
	WorkerTypeGitHubActions       WorkerType = "github_actions"
	WorkerTypeGitHubGit           WorkerType = "github_git" // Clone repository via git instead of API
	WorkerTypeTransform           WorkerType = "transform"
	WorkerTypeReindex             WorkerType = "reindex"
	WorkerTypeDatabaseMaintenance WorkerType = "database_maintenance"
	WorkerTypeLocalDir            WorkerType = "local_dir" // Local directory indexing (full content)
	WorkerTypeCodeMap             WorkerType = "code_map"  // Hierarchical code structure analysis
)

// IsValid checks if the WorkerType is a known, valid type
func (w WorkerType) IsValid() bool {
	switch w {
	case WorkerTypeAgent, WorkerTypeCrawler, WorkerTypePlacesSearch, WorkerTypeWebSearch,
		WorkerTypeGitHubRepo, WorkerTypeGitHubActions, WorkerTypeGitHubGit, WorkerTypeTransform,
		WorkerTypeReindex, WorkerTypeDatabaseMaintenance, WorkerTypeLocalDir, WorkerTypeCodeMap:
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
		WorkerTypeDatabaseMaintenance,
		WorkerTypeLocalDir,
		WorkerTypeCodeMap,
	}
}
