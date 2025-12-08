// -----------------------------------------------------------------------
// DevOps Metadata - Enrichment data structures for C/C++ code analysis
// -----------------------------------------------------------------------

package models

// DevOpsMetadata contains all enrichment data for C/C++ analysis
type DevOpsMetadata struct {
	// Pass 1: Extracted (regex, no LLM)
	Includes       []string `json:"includes,omitempty"`
	LocalIncludes  []string `json:"local_includes,omitempty"`
	SystemIncludes []string `json:"system_includes,omitempty"`
	Defines        []string `json:"defines,omitempty"`
	Conditionals   []string `json:"conditionals,omitempty"`
	Platforms      []string `json:"platforms,omitempty"`

	// Pass 2: Build System
	BuildTargets    []string `json:"build_targets,omitempty"`
	CompilerFlags   []string `json:"compiler_flags,omitempty"`
	LinkedLibraries []string `json:"linked_libraries,omitempty"`
	BuildDeps       []string `json:"build_deps,omitempty"`

	// Pass 3: Classification (LLM)
	FileRole      string   `json:"file_role,omitempty"`
	Component     string   `json:"component,omitempty"`
	TestType      string   `json:"test_type,omitempty"`
	TestFramework string   `json:"test_framework,omitempty"`
	TestRequires  []string `json:"test_requires,omitempty"`
	ExternalDeps  []string `json:"external_deps,omitempty"`
	ConfigSources []string `json:"config_sources,omitempty"`

	// Tracking
	EnrichmentPasses []string `json:"enrichment_passes,omitempty"`
}
