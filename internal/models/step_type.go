// -----------------------------------------------------------------------
// Last Modified: Friday, 29th November 2025 12:00:00 pm
// Modified By: Claude Code
// -----------------------------------------------------------------------

package models

// StepType represents the type of action a job step performs.
// This provides explicit type-safety for step routing to the appropriate worker.
type StepType string

// StepType constants define all supported step types for job execution
const (
	StepTypeAgent               StepType = "agent"
	StepTypeCrawler             StepType = "crawler"
	StepTypePlacesSearch        StepType = "places_search"
	StepTypeWebSearch           StepType = "web_search"
	StepTypeGitHubRepo          StepType = "github_repo"
	StepTypeGitHubActions       StepType = "github_actions"
	StepTypeTransform           StepType = "transform"
	StepTypeReindex             StepType = "reindex"
	StepTypeDatabaseMaintenance StepType = "database_maintenance"
)

// IsValid checks if the StepType is a known, valid type
func (s StepType) IsValid() bool {
	switch s {
	case StepTypeAgent, StepTypeCrawler, StepTypePlacesSearch, StepTypeWebSearch,
		StepTypeGitHubRepo, StepTypeGitHubActions, StepTypeTransform,
		StepTypeReindex, StepTypeDatabaseMaintenance:
		return true
	}
	return false
}

// String returns the string representation of the StepType
func (s StepType) String() string {
	return string(s)
}

// AllStepTypes returns a slice of all valid StepType values
func AllStepTypes() []StepType {
	return []StepType{
		StepTypeAgent,
		StepTypeCrawler,
		StepTypePlacesSearch,
		StepTypeWebSearch,
		StepTypeGitHubRepo,
		StepTypeGitHubActions,
		StepTypeTransform,
		StepTypeReindex,
		StepTypeDatabaseMaintenance,
	}
}
