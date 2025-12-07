// -----------------------------------------------------------------------
// ExtractStructureAction - Extract C/C++ code structure without LLM
// Regex-based extraction of includes, defines, conditionals, and platform info
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

// ExtractStructureAction processes C/C++ files to extract includes, defines, and platform info
type ExtractStructureAction struct {
	documentStorage interfaces.DocumentStorage
	logger          arbor.ILogger
}

// NewExtractStructureAction creates a new extract structure action
func NewExtractStructureAction(
	documentStorage interfaces.DocumentStorage,
	logger arbor.ILogger,
) *ExtractStructureAction {
	return &ExtractStructureAction{
		documentStorage: documentStorage,
		logger:          logger,
	}
}

// C/C++ file extensions
var cppExtensions = map[string]bool{
	".c":   true,
	".cpp": true,
	".cc":  true,
	".cxx": true,
	".h":   true,
	".hpp": true,
	".hxx": true,
	".hh":  true,
}

// Regex patterns
var (
	localIncludePattern  = regexp.MustCompile(`#include\s*"([^"]+)"`)
	systemIncludePattern = regexp.MustCompile(`#include\s*<([^>]+)>`)
	definePattern        = regexp.MustCompile(`#define\s+(\w+)`)
	ifdefPattern         = regexp.MustCompile(`#ifn?def\s+(\w+)`)
)

// Platform detection
var platformPatterns = map[string][]string{
	"windows":  {"_WIN32", "_WIN64", "WIN32", "__MINGW"},
	"linux":    {"__linux__", "__gnu_linux__"},
	"macos":    {"__APPLE__", "__MACH__", "TARGET_OS_MAC"},
	"embedded": {"__ARM__", "__EMBEDDED__", "ARDUINO", "STM32"},
}

// Execute processes a document and extracts C/C++ structure information
func (a *ExtractStructureAction) Execute(ctx context.Context, doc *models.Document, force bool) error {
	// 1. Check if already processed (unless force)
	if !force {
		devopsMetadata := a.getDevOpsMetadata(doc)
		if devopsMetadata != nil && a.containsPass(devopsMetadata.EnrichmentPasses, "extract_structure") {
			a.logger.Debug().
				Str("doc_id", doc.ID).
				Str("title", doc.Title).
				Msg("Skipping extract_structure - already processed")
			return nil
		}
	}

	// 2. Check file extension - extract path from metadata or URL
	filePath := a.getFilePath(doc)
	if !a.IsCppFile(filePath) {
		a.logger.Debug().
			Str("doc_id", doc.ID).
			Str("title", doc.Title).
			Str("file_path", filePath).
			Msg("Skipping extract_structure - not a C/C++ file")
		return nil
	}

	a.logger.Info().
		Str("doc_id", doc.ID).
		Str("title", doc.Title).
		Str("file_path", filePath).
		Msg("Extracting C/C++ structure")

	// 3. Extract all patterns from doc.ContentMarkdown
	content := doc.ContentMarkdown
	localIncludes, systemIncludes := a.ExtractIncludes(content)
	defines := a.ExtractDefines(content)
	conditionals := a.ExtractConditionals(content)
	platforms := a.DetectPlatforms(content)

	// 4. Get existing DevOps metadata or create new
	devopsMetadata := a.getDevOpsMetadata(doc)
	if devopsMetadata == nil {
		devopsMetadata = &models.DevOpsMetadata{}
	}

	// 5. Update metadata with extracted data
	devopsMetadata.LocalIncludes = localIncludes
	devopsMetadata.SystemIncludes = systemIncludes
	devopsMetadata.Defines = defines
	devopsMetadata.Conditionals = conditionals
	devopsMetadata.Platforms = platforms

	// Combine all includes for convenience
	allIncludes := make([]string, 0, len(localIncludes)+len(systemIncludes))
	allIncludes = append(allIncludes, localIncludes...)
	allIncludes = append(allIncludes, systemIncludes...)
	devopsMetadata.Includes = allIncludes

	// 6. Add "extract_structure" to enrichment_passes
	if !a.containsPass(devopsMetadata.EnrichmentPasses, "extract_structure") {
		devopsMetadata.EnrichmentPasses = append(devopsMetadata.EnrichmentPasses, "extract_structure")
	}

	// 7. Update document metadata
	if doc.Metadata == nil {
		doc.Metadata = make(map[string]interface{})
	}
	doc.Metadata["devops"] = devopsMetadata

	// 8. Save document
	if err := a.documentStorage.UpdateDocument(doc); err != nil {
		return fmt.Errorf("failed to update document: %w", err)
	}

	a.logger.Info().
		Str("doc_id", doc.ID).
		Int("local_includes", len(localIncludes)).
		Int("system_includes", len(systemIncludes)).
		Int("defines", len(defines)).
		Int("conditionals", len(conditionals)).
		Strs("platforms", platforms).
		Msg("C/C++ structure extracted successfully")

	return nil
}

// IsCppFile checks if a file path is a C/C++ file based on extension
func (a *ExtractStructureAction) IsCppFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return cppExtensions[ext]
}

// ExtractIncludes extracts local and system includes from content
func (a *ExtractStructureAction) ExtractIncludes(content string) (local, system []string) {
	// Extract local includes (unique)
	localMatches := localIncludePattern.FindAllStringSubmatch(content, -1)
	localSet := make(map[string]bool)
	for _, match := range localMatches {
		if len(match) > 1 {
			localSet[match[1]] = true
		}
	}
	local = make([]string, 0, len(localSet))
	for include := range localSet {
		local = append(local, include)
	}

	// Extract system includes (unique)
	systemMatches := systemIncludePattern.FindAllStringSubmatch(content, -1)
	systemSet := make(map[string]bool)
	for _, match := range systemMatches {
		if len(match) > 1 {
			systemSet[match[1]] = true
		}
	}
	system = make([]string, 0, len(systemSet))
	for include := range systemSet {
		system = append(system, include)
	}

	return
}

// ExtractDefines extracts unique define symbols from content
func (a *ExtractStructureAction) ExtractDefines(content string) []string {
	matches := definePattern.FindAllStringSubmatch(content, -1)
	definesSet := make(map[string]bool)
	for _, match := range matches {
		if len(match) > 1 {
			definesSet[match[1]] = true
		}
	}
	defines := make([]string, 0, len(definesSet))
	for define := range definesSet {
		defines = append(defines, define)
	}
	return defines
}

// ExtractConditionals extracts unique ifdef/ifndef symbols from content
func (a *ExtractStructureAction) ExtractConditionals(content string) []string {
	matches := ifdefPattern.FindAllStringSubmatch(content, -1)
	conditionalsSet := make(map[string]bool)
	for _, match := range matches {
		if len(match) > 1 {
			conditionalsSet[match[1]] = true
		}
	}
	conditionals := make([]string, 0, len(conditionalsSet))
	for conditional := range conditionalsSet {
		conditionals = append(conditionals, conditional)
	}
	return conditionals
}

// DetectPlatforms checks for platform-specific symbols in content
func (a *ExtractStructureAction) DetectPlatforms(content string) []string {
	platforms := make([]string, 0)
	for platform, patterns := range platformPatterns {
		for _, pattern := range patterns {
			if strings.Contains(content, pattern) {
				platforms = append(platforms, platform)
				break // Found this platform, move to next
			}
		}
	}
	return platforms
}

// Helper functions

// getDevOpsMetadata extracts DevOps metadata from document metadata
func (a *ExtractStructureAction) getDevOpsMetadata(doc *models.Document) *models.DevOpsMetadata {
	if doc.Metadata == nil {
		return nil
	}

	devopsData, ok := doc.Metadata["devops"]
	if !ok {
		return nil
	}

	// Try direct type assertion first
	if devops, ok := devopsData.(*models.DevOpsMetadata); ok {
		return devops
	}

	// If stored as map[string]interface{}, convert via JSON
	if devopsMap, ok := devopsData.(map[string]interface{}); ok {
		var devops models.DevOpsMetadata
		data, err := json.Marshal(devopsMap)
		if err != nil {
			a.logger.Warn().
				Err(err).
				Str("doc_id", doc.ID).
				Msg("Failed to marshal devops metadata")
			return nil
		}
		if err := json.Unmarshal(data, &devops); err != nil {
			a.logger.Warn().
				Err(err).
				Str("doc_id", doc.ID).
				Msg("Failed to unmarshal devops metadata")
			return nil
		}
		return &devops
	}

	return nil
}

// getFilePath extracts file path from document metadata or URL
func (a *ExtractStructureAction) getFilePath(doc *models.Document) string {
	// Try local_dir metadata first
	if doc.Metadata != nil {
		if localDir, ok := doc.Metadata["local_dir"]; ok {
			if localDirMap, ok := localDir.(map[string]interface{}); ok {
				if filePath, ok := localDirMap["file_path"].(string); ok {
					return filePath
				}
			}
		}

		// Try GitHub metadata
		if github, ok := doc.Metadata["github"]; ok {
			if githubMap, ok := github.(map[string]interface{}); ok {
				if filePath, ok := githubMap["file_path"].(string); ok {
					return filePath
				}
			}
		}
	}

	// Fall back to document URL or title
	if doc.URL != "" {
		return doc.URL
	}
	return doc.Title
}

// containsPass checks if a pass exists in the enrichment passes list
func (a *ExtractStructureAction) containsPass(passes []string, pass string) bool {
	for _, p := range passes {
		if p == pass {
			return true
		}
	}
	return false
}
