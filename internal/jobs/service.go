package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/pelletier/go-toml/v2"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
)

// Service handles job definition business logic including TOML parsing,
// conversion, and validation. This keeps the HTTP handler thin.
type Service struct {
	kvStorage    interfaces.KeyValueStorage
	agentService interfaces.AgentService // Optional: nil if agent service unavailable
	logger       arbor.ILogger
}

// NewService creates a new job definition service
func NewService(
	kvStorage interfaces.KeyValueStorage,
	agentService interfaces.AgentService, // Optional: can be nil
	logger arbor.ILogger,
) *Service {
	return &Service{
		kvStorage:    kvStorage,
		agentService: agentService,
		logger:       logger,
	}
}

// JobDefinitionFile represents the TOML file structure for job definitions
// Uses [step.{name}] format for step definitions
type JobDefinitionFile struct {
	ID          string   `toml:"id"`
	Name        string   `toml:"name"`
	Type        string   `toml:"type"`     // Optional: derived from first step's worker type if not specified
	JobType     string   `toml:"job_type"` // Owner type: "system" or "user"
	Description string   `toml:"description"`
	Schedule    string   `toml:"schedule"`
	Timeout     string   `toml:"timeout"`
	Enabled     *bool    `toml:"enabled"` // Pointer to detect if explicitly set; defaults to true
	AutoStart   bool     `toml:"auto_start"`
	AuthID      string   `toml:"authentication"`
	Tags        []string `toml:"tags"`

	// Extension matching: when true, this job definition can be matched by the Chrome extension
	// based on url_patterns. Default is false.
	Extension bool `toml:"extension"`

	// URL patterns for automatic job matching (wildcards: *.domain.com/*)
	// Used by Chrome extension to find matching job definitions when extension=true
	UrlPatterns []string `toml:"url_patterns"`

	// Crawler shorthand fields (creates default crawl step if no [step.*] defined)
	StartURLs       []string `toml:"start_urls"`
	IncludePatterns []string `toml:"include_patterns"`
	ExcludePatterns []string `toml:"exclude_patterns"`
	MaxDepth        int      `toml:"max_depth"`
	MaxPages        int      `toml:"max_pages"`
	Concurrency     int      `toml:"concurrency"`
	FollowLinks     bool     `toml:"follow_links"`

	// Step definitions: [step.{name}] tables - use map[string]interface{} to capture all fields
	Step map[string]map[string]interface{} `toml:"step"`
}

// ParseTOML parses TOML content into a JobDefinitionFile
func ParseTOML(content []byte) (*JobDefinitionFile, error) {
	// First, parse into a generic map to detect invalid keys like [[steps]]
	var rawConfig map[string]interface{}
	if err := toml.Unmarshal(content, &rawConfig); err != nil {
		return nil, fmt.Errorf("invalid TOML syntax: %w", err)
	}

	// Reject [[steps]] format - use [step.{name}] instead
	if _, hasSteps := rawConfig["steps"]; hasSteps {
		return nil, fmt.Errorf("invalid format: '[[steps]]' is not supported - use '[step.{name}]' format for step definitions")
	}

	var jobFile JobDefinitionFile
	if err := toml.Unmarshal(content, &jobFile); err != nil {
		return nil, fmt.Errorf("invalid TOML syntax: %w", err)
	}
	return &jobFile, nil
}

// ToJobDefinition converts the file structure to the internal model
func (f *JobDefinitionFile) ToJobDefinition() (*models.JobDefinition, error) {
	var steps []models.JobStep

	// Parse [step.{name}] tables
	if len(f.Step) > 0 {
		for name, stepData := range f.Step {
			// Extract known fields from the step map
			typeStr, _ := stepData["type"].(string)
			description, _ := stepData["description"].(string)
			onError, _ := stepData["on_error"].(string)
			depends, _ := stepData["depends"].(string)
			condition, _ := stepData["condition"].(string)

			// Validate that we have a valid type
			stepType := models.WorkerType(typeStr)
			if stepType == "" {
				return nil, fmt.Errorf("step '%s': 'type' field is required", name)
			}

			if !stepType.IsValid() {
				return nil, fmt.Errorf("step '%s': invalid type '%s' - must be one of: agent, crawler, places_search, web_search, github_repo, github_actions, github_git, transform, reindex, local_dir, code_map, summary, analyze_build, classify, dependency_graph, aggregate_summary, test_job_generator", name, stepType)
			}

			// Build config from all remaining fields (excluding known step metadata)
			config := make(map[string]interface{})
			knownFields := map[string]bool{
				"type":        true,
				"description": true,
				"on_error":    true,
				"depends":     true,
				"condition":   true,
			}
			for k, v := range stepData {
				if !knownFields[k] {
					config[k] = v
				}
			}

			step := models.JobStep{
				Name:        name,
				Type:        stepType,
				Description: description,
				Config:      config,
				OnError:     models.ErrorStrategy(onError),
				Depends:     depends,
				Condition:   condition,
			}
			// Default OnError if empty
			if step.OnError == "" {
				step.OnError = models.ErrorStrategyContinue
			}
			steps = append(steps, step)
		}

		// Sort steps by dependencies using topological sort
		steps = sortStepsByDependencies(steps)
	} else {
		// Crawler shorthand: Create default crawl step from flat config fields
		config := make(map[string]interface{})

		if len(f.StartURLs) > 0 {
			config["start_urls"] = f.StartURLs
		}
		if len(f.IncludePatterns) > 0 {
			config["include_patterns"] = f.IncludePatterns
		}
		if len(f.ExcludePatterns) > 0 {
			config["exclude_patterns"] = f.ExcludePatterns
		}

		config["max_depth"] = f.MaxDepth
		config["max_pages"] = f.MaxPages
		config["concurrency"] = f.Concurrency
		config["follow_links"] = f.FollowLinks

		step := models.JobStep{
			Name:    "crawl",
			Type:    models.WorkerTypeCrawler,
			Config:  config,
			OnError: models.ErrorStrategyContinue,
		}
		steps = append(steps, step)
	}

	// Determine job owner type (system vs user)
	jobOwnerType := models.JobOwnerTypeUser
	if f.JobType == "system" {
		jobOwnerType = models.JobOwnerTypeSystem
	}

	// Determine job definition type: use explicit type if provided, otherwise derive from first step
	var jobDefType models.JobDefinitionType
	if f.Type != "" {
		jobDefType = models.JobDefinitionType(f.Type)
	} else if len(steps) > 0 {
		// Derive from first step's worker type
		jobDefType = deriveJobDefinitionType(steps[0].Type)
	} else {
		// Default to crawler for backwards compatibility
		jobDefType = models.JobDefinitionTypeCrawler
	}

	// Default enabled to true if not explicitly set
	enabled := true
	if f.Enabled != nil {
		enabled = *f.Enabled
	}

	return &models.JobDefinition{
		ID:          f.ID,
		Name:        f.Name,
		Type:        jobDefType,
		JobType:     jobOwnerType,
		Description: f.Description,
		Schedule:    f.Schedule,
		Timeout:     f.Timeout,
		Enabled:     enabled,
		AutoStart:   f.AutoStart,
		AuthID:      f.AuthID,
		Tags:        f.Tags,
		Extension:   f.Extension,
		UrlPatterns: f.UrlPatterns,
		Steps:       steps,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}, nil
}

// deriveJobDefinitionType maps a WorkerType to the corresponding JobDefinitionType
func deriveJobDefinitionType(workerType models.WorkerType) models.JobDefinitionType {
	switch workerType {
	case models.WorkerTypeCrawler:
		return models.JobDefinitionTypeCrawler
	case models.WorkerTypeAgent:
		return models.JobDefinitionTypeAgent
	case models.WorkerTypeGitHubRepo, models.WorkerTypeGitHubActions, models.WorkerTypeGitHubGit:
		return models.JobDefinitionTypeFetch
	case models.WorkerTypeWebSearch:
		return models.JobDefinitionTypeWebSearch
	case models.WorkerTypePlacesSearch:
		return models.JobDefinitionTypePlaces
	default:
		return models.JobDefinitionTypeCustom
	}
}

// ConvertToTOML converts a JobDefinition to simplified TOML format for export
func (s *Service) ConvertToTOML(jobDef *models.JobDefinition) ([]byte, error) {
	// Extract crawler configuration from first step
	var crawlConfig map[string]interface{}
	if len(jobDef.Steps) > 0 && jobDef.Steps[0].Type == models.WorkerTypeCrawler {
		crawlConfig = jobDef.Steps[0].Config
	} else {
		crawlConfig = make(map[string]interface{})
	}

	// Build simplified structure matching the file format
	simplified := map[string]interface{}{
		"id":             jobDef.ID,
		"name":           jobDef.Name,
		"description":    jobDef.Description,
		"schedule":       jobDef.Schedule,
		"timeout":        jobDef.Timeout,
		"enabled":        jobDef.Enabled,
		"auto_start":     jobDef.AutoStart,
		"authentication": jobDef.AuthID, // Include authentication reference
	}

	// Include URL patterns if set
	if len(jobDef.UrlPatterns) > 0 {
		simplified["url_patterns"] = jobDef.UrlPatterns
	}

	// Extract crawler-specific fields from config
	if startURLs, ok := crawlConfig["start_urls"].([]interface{}); ok {
		urls := make([]string, 0, len(startURLs))
		for _, url := range startURLs {
			if urlStr, ok := url.(string); ok {
				urls = append(urls, urlStr)
			}
		}
		simplified["start_urls"] = urls
	} else {
		simplified["start_urls"] = []string{}
	}

	if includePatterns, ok := crawlConfig["include_patterns"].([]interface{}); ok {
		patterns := make([]string, 0, len(includePatterns))
		for _, pattern := range includePatterns {
			if patternStr, ok := pattern.(string); ok {
				patterns = append(patterns, patternStr)
			}
		}
		simplified["include_patterns"] = patterns
	} else {
		simplified["include_patterns"] = []string{}
	}

	if excludePatterns, ok := crawlConfig["exclude_patterns"].([]interface{}); ok {
		patterns := make([]string, 0, len(excludePatterns))
		for _, pattern := range excludePatterns {
			if patternStr, ok := pattern.(string); ok {
				patterns = append(patterns, patternStr)
			}
		}
		simplified["exclude_patterns"] = patterns
	} else {
		simplified["exclude_patterns"] = []string{}
	}

	// Extract numeric fields with defaults
	if maxDepth, ok := crawlConfig["max_depth"].(float64); ok {
		simplified["max_depth"] = int(maxDepth)
	} else {
		simplified["max_depth"] = 2
	}

	if maxPages, ok := crawlConfig["max_pages"].(float64); ok {
		simplified["max_pages"] = int(maxPages)
	} else {
		simplified["max_pages"] = 100
	}

	if concurrency, ok := crawlConfig["concurrency"].(float64); ok {
		simplified["concurrency"] = int(concurrency)
	} else {
		simplified["concurrency"] = 5
	}

	if followLinks, ok := crawlConfig["follow_links"].(bool); ok {
		simplified["follow_links"] = followLinks
	} else {
		simplified["follow_links"] = true
	}

	// Marshal to TOML
	tomlData, err := toml.Marshal(simplified)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal to TOML: %w", err)
	}

	return tomlData, nil
}

// ValidateStepActions validates that all step actions are registered
// TODO Phase 8-11: Re-enable when job registry is re-integrated
func (s *Service) ValidateStepActions(jobType models.JobDefinitionType, steps []models.JobStep) error {
	// Temporarily disabled during queue refactor - jobRegistry is interface{} with no methods
	_ = jobType // Suppress unused variable
	_ = steps   // Suppress unused variable
	return nil  // Skip validation during refactor

	// TODO Phase 8-11: Uncomment when job registry is available
	// for _, step := range steps {
	// 	if _, err := s.jobRegistry.GetAction(jobType, step.Action); err != nil {
	// 		return fmt.Errorf("unknown action '%s' for step '%s'", step.Action, step.Name)
	// 	}
	// }
	// return nil
}

// ValidateRuntimeDependencies checks if a job definition can execute based on available services
// This is separate from TOML validation - it checks runtime service availability
func (s *Service) ValidateRuntimeDependencies(jobDef *models.JobDefinition) {
	// Default to ready
	jobDef.RuntimeStatus = "ready"
	jobDef.RuntimeError = ""

	// Validate API keys referenced in job definition steps
	s.ValidateAPIKeys(jobDef)
	if jobDef.RuntimeStatus != "ready" {
		// ValidateAPIKeys set an error status, return early
		return
	}

	// Check each step for dependencies
	for _, step := range jobDef.Steps {
		switch step.Type {
		case models.WorkerTypeAgent:
			// Agent steps require agent service
			if s.agentService == nil {
				jobDef.RuntimeStatus = "disabled"
				jobDef.RuntimeError = "Google API key is required for agent service (set QUAERO_GEMINI_GOOGLE_API_KEY or gemini.google_api_key in config)"
				return
			}
			// Add more step types here as needed
			// case models.StepTypePlacesSearch:
			//     if s.placesService == nil {
			//         jobDef.RuntimeStatus = "disabled"
			//         jobDef.RuntimeError = "Google Places API key required"
			//         return
			//     }
		}
	}
}

// ValidateAPIKeys validates that API keys referenced in job definition steps exist in storage
func (s *Service) ValidateAPIKeys(jobDef *models.JobDefinition) {
	ctx := context.Background()

	// Check all steps for api_key field
	for _, step := range jobDef.Steps {
		if step.Config != nil {
			if apiKeyName, ok := step.Config["api_key"].(string); ok && apiKeyName != "" {
				// Handle {key-name} variable reference pattern
				// If api_key is wrapped in braces like "{google_gemini_api_key}",
				// extract the actual key name for lookup
				lookupName := apiKeyName
				if len(apiKeyName) > 2 && apiKeyName[0] == '{' && apiKeyName[len(apiKeyName)-1] == '}' {
					lookupName = apiKeyName[1 : len(apiKeyName)-1]
				}

				// Try to resolve the API key from KV store
				_, err := common.ResolveAPIKey(ctx, s.kvStorage, lookupName, "")
				if err != nil {
					// API key not found or invalid
					jobDef.RuntimeStatus = "error"
					jobDef.RuntimeError = fmt.Sprintf("API key '%s' not found", apiKeyName)
					s.logger.Warn().
						Str("job_def_id", jobDef.ID).
						Str("api_key_name", apiKeyName).
						Str("error", err.Error()).
						Msg("API key validation failed for job definition")
					return // Return immediately on first error
				}
			}
		}
	}
}

// sortStepsByDependencies sorts steps using topological sort based on the depends field
// Steps with no dependencies come first, followed by steps in dependency order
func sortStepsByDependencies(steps []models.JobStep) []models.JobStep {
	if len(steps) <= 1 {
		return steps
	}

	// Build name to step map and adjacency list
	nameToStep := make(map[string]models.JobStep)
	inDegree := make(map[string]int)

	for _, step := range steps {
		nameToStep[step.Name] = step
		inDegree[step.Name] = 0
	}

	// Calculate in-degrees based on dependencies
	for _, step := range steps {
		if step.Depends != "" {
			// Split comma-separated dependencies
			deps := splitAndTrimDeps(step.Depends)
			for _, dep := range deps {
				if _, exists := nameToStep[dep]; exists {
					inDegree[step.Name]++
				}
			}
		}
	}

	// Kahn's algorithm for topological sort
	var queue []string
	for name, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, name)
		}
	}

	var sorted []models.JobStep
	for len(queue) > 0 {
		// Pop first element
		name := queue[0]
		queue = queue[1:]

		step := nameToStep[name]
		sorted = append(sorted, step)

		// Reduce in-degree for dependent steps
		for _, s := range steps {
			if s.Depends != "" {
				deps := splitAndTrimDeps(s.Depends)
				for _, dep := range deps {
					if dep == name {
						inDegree[s.Name]--
						if inDegree[s.Name] == 0 {
							queue = append(queue, s.Name)
						}
					}
				}
			}
		}
	}

	// If we couldn't sort all steps (cycle detected), return original order
	if len(sorted) != len(steps) {
		return steps
	}

	return sorted
}

// splitAndTrimDeps splits a comma-separated dependency string and trims whitespace
func splitAndTrimDeps(depends string) []string {
	if depends == "" {
		return nil
	}
	parts := make([]string, 0)
	for _, p := range splitString(depends, ",") {
		trimmed := trimWhitespace(p)
		if trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return parts
}

// splitString splits a string by a separator
func splitString(s, sep string) []string {
	var result []string
	start := 0
	for i := 0; i <= len(s)-len(sep); i++ {
		if s[i:i+len(sep)] == sep {
			result = append(result, s[start:i])
			start = i + len(sep)
			i += len(sep) - 1
		}
	}
	result = append(result, s[start:])
	return result
}

// trimWhitespace trims leading and trailing whitespace from a string
func trimWhitespace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}
