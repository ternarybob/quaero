// -----------------------------------------------------------------------
// AnalyzeBuildSystemAction - Parse build files for targets, flags, and dependencies
// Supports Makefile, CMake, Visual Studio, and configure-based build systems
// -----------------------------------------------------------------------

package actions

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
)

// AnalyzeBuildSystemAction analyzes build files to extract build targets,
// compiler flags, linked libraries, and build dependencies.
// Uses regex for initial extraction and LLM for complex analysis.
type AnalyzeBuildSystemAction struct {
	documentStorage interfaces.DocumentStorage
	llmService      interfaces.LLMService
	logger          arbor.ILogger
}

// NewAnalyzeBuildSystemAction creates a new build system analysis action
func NewAnalyzeBuildSystemAction(
	documentStorage interfaces.DocumentStorage,
	llmService interfaces.LLMService,
	logger arbor.ILogger,
) *AnalyzeBuildSystemAction {
	return &AnalyzeBuildSystemAction{
		documentStorage: documentStorage,
		llmService:      llmService,
		logger:          logger,
	}
}

// Build file detection patterns
var buildFilePatterns = []struct {
	pattern   string
	extension string
	prefix    string
}{
	{pattern: "Makefile", prefix: "Makefile"},
	{extension: ".mk"},
	{pattern: "CMakeLists.txt"},
	{extension: ".cmake"},
	{extension: ".vcxproj"},
	{pattern: ".vcxproj.filters", extension: ".filters"},
	{prefix: "configure"},
	{extension: ".sln"},
}

// Regex patterns for extracting build information
var (
	// Makefile targets: matches "target:" at start of line
	makefileTargetPattern = regexp.MustCompile(`(?m)^([a-zA-Z_][a-zA-Z0-9_.-]*)\s*:`)

	// CMake targets: matches add_executable(name ...) and add_library(name ...)
	cmakeExecutablePattern = regexp.MustCompile(`add_executable\s*\(\s*(\w+)`)
	cmakeLibraryPattern    = regexp.MustCompile(`add_library\s*\(\s*(\w+)`)

	// Compiler flags: -D (define), -I (include), -L (library path), -O (optimization)
	compilerFlagPattern = regexp.MustCompile(`-[DILO]\S+`)

	// Linked libraries: -l flag (e.g., -lpthread, -lm)
	linkedLibPattern = regexp.MustCompile(`-l(\w+)`)

	// CMake link libraries: target_link_libraries(target lib1 lib2)
	cmakeLinkPattern = regexp.MustCompile(`target_link_libraries\s*\([^)]*\b(\w+)\b`)

	// LDFLAGS and similar variables
	ldflagsPattern = regexp.MustCompile(`(?i)LDFLAGS\s*[+:=]\s*([^\n]+)`)

	// Object file dependencies: .o, .obj files
	objFilePattern = regexp.MustCompile(`\b(\w+\.(?:o|obj))\b`)
)

// IsBuildFile checks if a file path represents a build file
func (a *AnalyzeBuildSystemAction) IsBuildFile(path string) bool {
	base := filepath.Base(path)
	ext := filepath.Ext(path)

	for _, pattern := range buildFilePatterns {
		// Check exact pattern match
		if pattern.pattern != "" && base == pattern.pattern {
			return true
		}
		// Check extension
		if pattern.extension != "" && ext == pattern.extension {
			return true
		}
		// Check prefix
		if pattern.prefix != "" && strings.HasPrefix(base, pattern.prefix) {
			return true
		}
	}

	return false
}

// Execute performs build system analysis on a document
func (a *AnalyzeBuildSystemAction) Execute(ctx context.Context, doc *models.Document, force bool) error {
	// Check if this is a build file
	if !a.IsBuildFile(doc.FilePath) {
		a.logger.Debug().
			Str("file_path", doc.FilePath).
			Msg("Skipping non-build file")
		return nil
	}

	// Check if already processed (unless force is true)
	if !force && a.hasEnrichmentPass(doc, "analyze_build_system") {
		a.logger.Debug().
			Str("document_id", doc.ID).
			Str("file_path", doc.FilePath).
			Msg("Document already has analyze_build_system pass, skipping")
		return nil
	}

	a.logger.Info().
		Str("document_id", doc.ID).
		Str("file_path", doc.FilePath).
		Msg("Analyzing build system file")

	// Initialize DevOps metadata if not present
	if doc.Metadata == nil {
		doc.Metadata = make(map[string]interface{})
	}

	devopsData := a.getDevOpsMetadata(doc)

	// Extract build information based on file type
	content := doc.Content
	base := filepath.Base(doc.FilePath)

	if strings.HasPrefix(base, "Makefile") || filepath.Ext(doc.FilePath) == ".mk" {
		a.analyzeMakefile(content, devopsData)
	} else if base == "CMakeLists.txt" || filepath.Ext(doc.FilePath) == ".cmake" {
		a.analyzeCMake(content, devopsData)
	} else if strings.HasSuffix(doc.FilePath, ".vcxproj") {
		a.analyzeVCXProj(content, devopsData)
	} else if strings.HasPrefix(base, "configure") || filepath.Ext(doc.FilePath) == ".sln" {
		a.analyzeConfigureOrSln(content, devopsData)
	}

	// Extract compiler flags and linked libraries (common to all build systems)
	devopsData.CompilerFlags = append(devopsData.CompilerFlags, a.ExtractCompilerFlags(content)...)
	devopsData.LinkedLibraries = append(devopsData.LinkedLibraries, a.ExtractLinkedLibraries(content)...)
	devopsData.BuildDeps = append(devopsData.BuildDeps, a.ExtractObjectFiles(content)...)

	// Deduplicate slices
	devopsData.BuildTargets = uniqueStrings(devopsData.BuildTargets)
	devopsData.CompilerFlags = uniqueStrings(devopsData.CompilerFlags)
	devopsData.LinkedLibraries = uniqueStrings(devopsData.LinkedLibraries)
	devopsData.BuildDeps = uniqueStrings(devopsData.BuildDeps)

	// Use LLM for complex analysis if available and content is substantial
	if a.llmService != nil && len(content) > 100 && len(devopsData.BuildTargets) > 0 {
		llmData, err := a.AnalyzeWithLLM(ctx, content, doc.FilePath)
		if err != nil {
			a.logger.Warn().
				Err(err).
				Str("file_path", doc.FilePath).
				Msg("LLM analysis failed, using regex results only")
		} else {
			// Merge LLM results with regex results
			a.mergeLLMResults(devopsData, llmData)
		}
	}

	// Add enrichment pass marker
	devopsData.EnrichmentPasses = append(devopsData.EnrichmentPasses, "analyze_build_system")
	devopsData.EnrichmentPasses = uniqueStrings(devopsData.EnrichmentPasses)

	// Update document metadata
	doc.Metadata["devops"] = devopsData

	// Save document
	if err := a.documentStorage.UpdateDocument(doc); err != nil {
		return fmt.Errorf("failed to update document: %w", err)
	}

	a.logger.Info().
		Str("document_id", doc.ID).
		Str("file_path", doc.FilePath).
		Int("build_targets", len(devopsData.BuildTargets)).
		Int("compiler_flags", len(devopsData.CompilerFlags)).
		Int("linked_libraries", len(devopsData.LinkedLibraries)).
		Msg("Build system analysis completed")

	return nil
}

// analyzeMakefile extracts targets and dependencies from Makefiles
func (a *AnalyzeBuildSystemAction) analyzeMakefile(content string, devopsData *models.DevOpsMetadata) {
	targets := a.ExtractMakefileTargets(content)
	devopsData.BuildTargets = append(devopsData.BuildTargets, targets...)
}

// analyzeCMake extracts targets from CMake files
func (a *AnalyzeBuildSystemAction) analyzeCMake(content string, devopsData *models.DevOpsMetadata) {
	targets := a.ExtractCMakeTargets(content)
	devopsData.BuildTargets = append(devopsData.BuildTargets, targets...)
}

// analyzeVCXProj extracts project name from Visual Studio project files
func (a *AnalyzeBuildSystemAction) analyzeVCXProj(content string, devopsData *models.DevOpsMetadata) {
	// For Visual Studio projects, extract ProjectName from XML
	projectNamePattern := regexp.MustCompile(`<ProjectName>([^<]+)</ProjectName>`)
	if matches := projectNamePattern.FindStringSubmatch(content); len(matches) > 1 {
		devopsData.BuildTargets = append(devopsData.BuildTargets, matches[1])
	}

	// Extract configuration type (Application, DynamicLibrary, StaticLibrary)
	configTypePattern := regexp.MustCompile(`<ConfigurationType>([^<]+)</ConfigurationType>`)
	if matches := configTypePattern.FindStringSubmatch(content); len(matches) > 1 {
		devopsData.BuildTargets = append(devopsData.BuildTargets, matches[1])
	}
}

// analyzeConfigureOrSln handles autoconf configure scripts and .sln files
func (a *AnalyzeBuildSystemAction) analyzeConfigureOrSln(content string, devopsData *models.DevOpsMetadata) {
	// For .sln files, extract project references
	if strings.Contains(content, "Microsoft Visual Studio Solution") {
		projectPattern := regexp.MustCompile(`Project\("[^"]+"\)\s*=\s*"([^"]+)"`)
		matches := projectPattern.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			if len(match) > 1 {
				devopsData.BuildTargets = append(devopsData.BuildTargets, match[1])
			}
		}
	}

	// For configure scripts, look for AC_INIT or package name
	packagePattern := regexp.MustCompile(`(?:AC_INIT|PACKAGE_NAME)\s*\(\s*\[?([^\],)]+)`)
	if matches := packagePattern.FindStringSubmatch(content); len(matches) > 1 {
		devopsData.BuildTargets = append(devopsData.BuildTargets, strings.TrimSpace(matches[1]))
	}
}

// ExtractMakefileTargets extracts targets from Makefile content
func (a *AnalyzeBuildSystemAction) ExtractMakefileTargets(content string) []string {
	var targets []string
	matches := makefileTargetPattern.FindAllStringSubmatch(content, -1)

	for _, match := range matches {
		if len(match) > 1 {
			target := match[1]
			// Skip special targets and variables
			if !strings.Contains(target, "$") && !strings.Contains(target, "%") {
				targets = append(targets, target)
			}
		}
	}

	return targets
}

// ExtractCMakeTargets extracts targets from CMake content
func (a *AnalyzeBuildSystemAction) ExtractCMakeTargets(content string) []string {
	var targets []string

	// Extract executables
	execMatches := cmakeExecutablePattern.FindAllStringSubmatch(content, -1)
	for _, match := range execMatches {
		if len(match) > 1 {
			targets = append(targets, match[1])
		}
	}

	// Extract libraries
	libMatches := cmakeLibraryPattern.FindAllStringSubmatch(content, -1)
	for _, match := range libMatches {
		if len(match) > 1 {
			targets = append(targets, match[1])
		}
	}

	return targets
}

// ExtractCompilerFlags extracts compiler flags from build file content
func (a *AnalyzeBuildSystemAction) ExtractCompilerFlags(content string) []string {
	var flags []string
	matches := compilerFlagPattern.FindAllString(content, -1)

	for _, match := range matches {
		flags = append(flags, match)
	}

	return flags
}

// ExtractLinkedLibraries extracts linked libraries from build file content
func (a *AnalyzeBuildSystemAction) ExtractLinkedLibraries(content string) []string {
	var libraries []string

	// Extract from -l flags
	lMatches := linkedLibPattern.FindAllStringSubmatch(content, -1)
	for _, match := range lMatches {
		if len(match) > 1 {
			libraries = append(libraries, match[1])
		}
	}

	// Extract from CMake target_link_libraries
	cmakeMatches := cmakeLinkPattern.FindAllStringSubmatch(content, -1)
	for _, match := range cmakeMatches {
		if len(match) > 1 {
			// Skip CMake keywords
			lib := match[1]
			if lib != "PUBLIC" && lib != "PRIVATE" && lib != "INTERFACE" {
				libraries = append(libraries, lib)
			}
		}
	}

	return libraries
}

// ExtractObjectFiles extracts object file dependencies from build content
func (a *AnalyzeBuildSystemAction) ExtractObjectFiles(content string) []string {
	var objFiles []string
	matches := objFilePattern.FindAllStringSubmatch(content, -1)

	for _, match := range matches {
		if len(match) > 1 {
			objFiles = append(objFiles, match[1])
		}
	}

	return objFiles
}

// AnalyzeWithLLM uses LLM to analyze complex build dependencies
func (a *AnalyzeBuildSystemAction) AnalyzeWithLLM(ctx context.Context, content, filePath string) (map[string]interface{}, error) {
	// Truncate content to avoid exceeding LLM token limits
	maxContentLength := 4000
	truncatedContent := content
	if len(content) > maxContentLength {
		truncatedContent = content[:maxContentLength] + "\n... (truncated)"
	}

	// Determine build system type
	buildType := "unknown"
	base := filepath.Base(filePath)
	if strings.HasPrefix(base, "Makefile") || filepath.Ext(filePath) == ".mk" {
		buildType = "Makefile"
	} else if base == "CMakeLists.txt" || filepath.Ext(filePath) == ".cmake" {
		buildType = "CMake"
	} else if strings.HasSuffix(filePath, ".vcxproj") {
		buildType = "vcxproj"
	}

	// Construct prompt
	prompt := fmt.Sprintf(`You are analyzing a build file for a DevOps engineer who needs to understand how to build this C/C++ project.

File: %s
Type: %s

Content:
%s

Analyze and return JSON with:
{
  "build_targets": ["target1", "target2"],
  "compiler_flags": ["-flag1", "-flag2"],
  "linked_libraries": ["lib1", "lib2"],
  "build_deps": ["dep1.o", "dep2.o"],
  "toolchain": "gcc|clang|msvc",
  "notes": "Any important observations"
}

Only return valid JSON. Do not include markdown code blocks or any other text.`, filePath, buildType, truncatedContent)

	// Call LLM
	messages := []interfaces.Message{
		{
			Role:    "user",
			Content: prompt,
		},
	}

	response, err := a.llmService.Chat(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("LLM chat failed: %w", err)
	}

	// Clean up response (remove markdown code blocks if present)
	response = strings.TrimSpace(response)
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	// Parse JSON response
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return nil, fmt.Errorf("failed to parse LLM response as JSON: %w", err)
	}

	return result, nil
}

// mergeLLMResults merges LLM analysis results with regex results
func (a *AnalyzeBuildSystemAction) mergeLLMResults(devopsData *models.DevOpsMetadata, llmData map[string]interface{}) {
	// Merge build targets
	if targets, ok := llmData["build_targets"].([]interface{}); ok {
		for _, target := range targets {
			if targetStr, ok := target.(string); ok {
				devopsData.BuildTargets = append(devopsData.BuildTargets, targetStr)
			}
		}
	}

	// Merge compiler flags
	if flags, ok := llmData["compiler_flags"].([]interface{}); ok {
		for _, flag := range flags {
			if flagStr, ok := flag.(string); ok {
				devopsData.CompilerFlags = append(devopsData.CompilerFlags, flagStr)
			}
		}
	}

	// Merge linked libraries
	if libs, ok := llmData["linked_libraries"].([]interface{}); ok {
		for _, lib := range libs {
			if libStr, ok := lib.(string); ok {
				devopsData.LinkedLibraries = append(devopsData.LinkedLibraries, libStr)
			}
		}
	}

	// Merge build dependencies
	if deps, ok := llmData["build_deps"].([]interface{}); ok {
		for _, dep := range deps {
			if depStr, ok := dep.(string); ok {
				devopsData.BuildDeps = append(devopsData.BuildDeps, depStr)
			}
		}
	}

	// Store toolchain and notes in metadata if present
	if toolchain, ok := llmData["toolchain"].(string); ok && toolchain != "" {
		// Could add a Toolchain field to DevOpsMetadata if needed
		a.logger.Debug().Str("toolchain", toolchain).Msg("LLM identified toolchain")
	}

	if notes, ok := llmData["notes"].(string); ok && notes != "" {
		a.logger.Debug().Str("notes", notes).Msg("LLM analysis notes")
	}
}

// getDevOpsMetadata retrieves or initializes DevOps metadata from document
func (a *AnalyzeBuildSystemAction) getDevOpsMetadata(doc *models.Document) *models.DevOpsMetadata {
	if devopsIface, ok := doc.Metadata["devops"]; ok {
		if devops, ok := devopsIface.(*models.DevOpsMetadata); ok {
			return devops
		}
		// Try to convert from map
		if devopsMap, ok := devopsIface.(map[string]interface{}); ok {
			devops := &models.DevOpsMetadata{}
			// Convert map to struct (basic conversion)
			if includes, ok := devopsMap["includes"].([]interface{}); ok {
				for _, inc := range includes {
					if incStr, ok := inc.(string); ok {
						devops.Includes = append(devops.Includes, incStr)
					}
				}
			}
			// Add more conversions as needed...
			return devops
		}
	}

	return &models.DevOpsMetadata{}
}

// hasEnrichmentPass checks if document has a specific enrichment pass
func (a *AnalyzeBuildSystemAction) hasEnrichmentPass(doc *models.Document, pass string) bool {
	devops := a.getDevOpsMetadata(doc)
	for _, p := range devops.EnrichmentPasses {
		if p == pass {
			return true
		}
	}
	return false
}

// uniqueStrings deduplicates a slice of strings
func uniqueStrings(slice []string) []string {
	seen := make(map[string]bool)
	result := []string{}

	for _, item := range slice {
		if item != "" && !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}

	return result
}
