// Package templates provides embedded TOML templates with user override support.
// Templates are loaded with resolution order:
// 1. User override: templatesDir/{name}.toml
// 2. Embedded default: internal/templates/{name}.toml
package templates

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

//go:embed *.toml
var fs embed.FS

// TemplateType defines the type of template
type TemplateType string

const (
	// TemplateTypeOrchestrator is for OrchestratorWorker (goal + available_tools)
	TemplateTypeOrchestrator TemplateType = "orchestrator"
	// TemplateTypePrompt is for SummaryWorker (prompt + schema)
	TemplateTypePrompt TemplateType = "prompt"
)

// Tool represents a tool definition in orchestrator templates
type Tool struct {
	Name        string `toml:"name"`
	Description string `toml:"description"`
	Worker      string `toml:"worker"`
}

// Template represents a loaded template
type Template struct {
	Type           TemplateType `toml:"type"`
	Prompt         string       `toml:"prompt"`          // For prompt templates
	Goal           string       `toml:"goal"`            // For orchestrator templates
	SchemaRef      string       `toml:"schema_ref"`      // Output schema reference
	AvailableTools []Tool       `toml:"available_tools"` // For orchestrator templates
}

// GetTemplate loads a template by name with resolution order:
// 1. User override: templatesDir/{name}.toml
// 2. Embedded default: internal/templates/{name}.toml
func GetTemplate(name string, templatesDir string) (*Template, error) {
	// Try user override first
	if templatesDir != "" {
		userPath := filepath.Join(templatesDir, name+".toml")
		if data, err := os.ReadFile(userPath); err == nil {
			return parseTemplate(data)
		}
	}

	// Fall back to embedded default
	embeddedName := name + ".toml"
	data, err := fs.ReadFile(embeddedName)
	if err != nil {
		return nil, fmt.Errorf("template '%s' not found (checked user override and embedded)", name)
	}
	return parseTemplate(data)
}

// GetEmbeddedTemplate loads raw content from embedded templates (for testing)
func GetEmbeddedTemplate(name string) ([]byte, error) {
	return fs.ReadFile(name + ".toml")
}

// ListEmbeddedTemplates returns names of all embedded templates
func ListEmbeddedTemplates() ([]string, error) {
	entries, err := fs.ReadDir(".")
	if err != nil {
		return nil, err
	}

	var names []string
	for _, entry := range entries {
		if !entry.IsDir() {
			name := entry.Name()
			// Remove .toml extension
			if len(name) > 5 && name[len(name)-5:] == ".toml" {
				names = append(names, name[:len(name)-5])
			}
		}
	}
	return names, nil
}

func parseTemplate(data []byte) (*Template, error) {
	var t Template
	if err := toml.Unmarshal(data, &t); err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}
	return &t, nil
}
