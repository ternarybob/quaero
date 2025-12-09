package agents

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"

	"google.golang.org/genai"
)

// RuleClassifier implements the AgentExecutor interface for rule-based file classification.
// It classifies files by filename patterns, directory structure, and extensions WITHOUT
// making LLM calls - providing fast, deterministic classification for ~90% of files.
//
// Input Format:
//
//	{
//	    "document_id": "doc_123",           // Document identifier
//	    "content": "Document text...",      // Full document content (used for heuristics)
//	    "title": "main.go"                  // Document title/filename (primary classification input)
//	}
//
// Output Format:
//
//	{
//	    "category": "source",               // Primary category: source, test, config, docs, build, ci, script, data, unknown
//	    "subcategory": "entrypoint",        // More specific classification
//	    "purpose": "Main entry point",      // Brief description of file purpose
//	    "importance": "high",               // Importance level: high, medium, low
//	    "tags": ["entrypoint", "main"],     // Relevant tags for the file
//	    "rule_matched": "main-entrypoint"   // Name of the rule that matched (empty if unknown)
//	}
type RuleClassifier struct{}

// classificationRule defines a single classification rule
type classificationRule struct {
	name        string   // Rule identifier
	category    string   // Primary category
	subcategory string   // Subcategory
	importance  string   // high, medium, low
	purpose     string   // Purpose template
	tags        []string // Default tags
	// Matchers (any match triggers the rule)
	filePatterns []string         // Glob patterns for filename (e.g., "*_test.go")
	pathPatterns []string         // Glob patterns for full path (e.g., "/test/*")
	regexes      []*regexp.Regexp // Regex patterns for more complex matching
}

// classificationRules defines all rules in priority order (first match wins)
var classificationRules = []classificationRule{
	// Test files - high priority
	{
		name:         "go-test",
		category:     "test",
		subcategory:  "unit-test",
		importance:   "medium",
		purpose:      "Go unit test file",
		tags:         []string{"test", "go", "unit-test"},
		filePatterns: []string{"*_test.go"},
	},
	{
		name:         "js-test",
		category:     "test",
		subcategory:  "unit-test",
		importance:   "medium",
		purpose:      "JavaScript/TypeScript test file",
		tags:         []string{"test", "javascript"},
		filePatterns: []string{"*.test.js", "*.test.ts", "*.test.jsx", "*.test.tsx", "*.spec.js", "*.spec.ts", "*.spec.jsx", "*.spec.tsx"},
	},
	{
		name:         "python-test",
		category:     "test",
		subcategory:  "unit-test",
		importance:   "medium",
		purpose:      "Python test file",
		tags:         []string{"test", "python"},
		filePatterns: []string{"test_*.py", "*_test.py"},
	},
	{
		name:         "integration-test",
		category:     "test",
		subcategory:  "integration-test",
		importance:   "medium",
		purpose:      "Integration test file",
		tags:         []string{"test", "integration"},
		filePatterns: []string{"*_integration_test.go", "*_integration_test.py", "*.integration.test.*"},
		pathPatterns: []string{"*/integration/*", "*/e2e/*"},
	},
	{
		name:         "mock-file",
		category:     "test",
		subcategory:  "mock",
		importance:   "low",
		purpose:      "Mock/stub implementation for testing",
		tags:         []string{"test", "mock"},
		filePatterns: []string{"*_mock.go", "*_stub.go", "mock_*.go", "*.mock.js", "*.mock.ts"},
		pathPatterns: []string{"*/mocks/*", "*/stubs/*", "*/__mocks__/*"},
	},
	{
		name:         "test-directory",
		category:     "test",
		subcategory:  "test-support",
		importance:   "medium",
		purpose:      "Test support file",
		tags:         []string{"test"},
		pathPatterns: []string{"*/test/*", "*/tests/*", "*/__tests__/*"},
	},

	// CI/CD files
	{
		name:         "github-workflow",
		category:     "ci",
		subcategory:  "github-actions",
		importance:   "high",
		purpose:      "GitHub Actions workflow definition",
		tags:         []string{"ci", "github", "automation"},
		pathPatterns: []string{"**/.github/workflows/**", ".github/workflows/*"},
	},
	{
		name:         "gitlab-ci",
		category:     "ci",
		subcategory:  "gitlab-ci",
		importance:   "high",
		purpose:      "GitLab CI/CD pipeline configuration",
		tags:         []string{"ci", "gitlab", "automation"},
		filePatterns: []string{".gitlab-ci.yml", ".gitlab-ci.yaml"},
	},
	{
		name:         "jenkins",
		category:     "ci",
		subcategory:  "jenkins",
		importance:   "high",
		purpose:      "Jenkins pipeline configuration",
		tags:         []string{"ci", "jenkins", "automation"},
		filePatterns: []string{"Jenkinsfile", "jenkins.groovy"},
	},
	{
		name:         "circleci",
		category:     "ci",
		subcategory:  "circleci",
		importance:   "high",
		purpose:      "CircleCI configuration",
		tags:         []string{"ci", "circleci", "automation"},
		pathPatterns: []string{"*/.circleci/*"},
	},

	// Build/Container files
	{
		name:         "dockerfile",
		category:     "build",
		subcategory:  "container",
		importance:   "high",
		purpose:      "Docker container definition",
		tags:         []string{"docker", "container", "build"},
		filePatterns: []string{"Dockerfile", "Dockerfile.*", "*.dockerfile"},
	},
	{
		name:         "docker-compose",
		category:     "build",
		subcategory:  "container-orchestration",
		importance:   "high",
		purpose:      "Docker Compose configuration",
		tags:         []string{"docker", "compose", "orchestration"},
		filePatterns: []string{"docker-compose.yml", "docker-compose.yaml", "docker-compose.*.yml", "docker-compose.*.yaml", "compose.yml", "compose.yaml"},
	},
	{
		name:         "makefile",
		category:     "build",
		subcategory:  "build-system",
		importance:   "high",
		purpose:      "Make build configuration",
		tags:         []string{"make", "build"},
		filePatterns: []string{"Makefile", "makefile", "GNUmakefile", "*.mk"},
	},
	{
		name:         "cmake",
		category:     "build",
		subcategory:  "build-system",
		importance:   "high",
		purpose:      "CMake build configuration",
		tags:         []string{"cmake", "build", "cpp"},
		filePatterns: []string{"CMakeLists.txt", "*.cmake"},
	},

	// Dependency files
	{
		name:         "go-mod",
		category:     "build",
		subcategory:  "dependency",
		importance:   "high",
		purpose:      "Go module definition",
		tags:         []string{"go", "dependency", "module"},
		filePatterns: []string{"go.mod", "go.sum"},
	},
	{
		name:         "npm-package",
		category:     "build",
		subcategory:  "dependency",
		importance:   "high",
		purpose:      "Node.js package definition",
		tags:         []string{"npm", "nodejs", "dependency"},
		filePatterns: []string{"package.json", "package-lock.json", "yarn.lock", "pnpm-lock.yaml"},
	},
	{
		name:         "python-deps",
		category:     "build",
		subcategory:  "dependency",
		importance:   "high",
		purpose:      "Python dependency definition",
		tags:         []string{"python", "dependency"},
		filePatterns: []string{"requirements.txt", "requirements-*.txt", "Pipfile", "Pipfile.lock", "pyproject.toml", "setup.py", "setup.cfg"},
	},
	{
		name:         "cargo",
		category:     "build",
		subcategory:  "dependency",
		importance:   "high",
		purpose:      "Rust Cargo configuration",
		tags:         []string{"rust", "cargo", "dependency"},
		filePatterns: []string{"Cargo.toml", "Cargo.lock"},
	},

	// Documentation
	{
		name:         "readme",
		category:     "docs",
		subcategory:  "readme",
		importance:   "high",
		purpose:      "Project documentation",
		tags:         []string{"documentation", "readme"},
		filePatterns: []string{"README*", "readme*"},
	},
	{
		name:         "changelog",
		category:     "docs",
		subcategory:  "changelog",
		importance:   "medium",
		purpose:      "Version changelog",
		tags:         []string{"documentation", "changelog"},
		filePatterns: []string{"CHANGELOG*", "changelog*", "HISTORY*", "NEWS*"},
	},
	{
		name:         "license",
		category:     "docs",
		subcategory:  "license",
		importance:   "high",
		purpose:      "License file",
		tags:         []string{"license", "legal"},
		filePatterns: []string{"LICENSE*", "license*", "COPYING*"},
	},
	{
		name:         "docs-directory",
		category:     "docs",
		subcategory:  "documentation",
		importance:   "medium",
		purpose:      "Documentation file",
		tags:         []string{"documentation"},
		pathPatterns: []string{"*/docs/*", "*/doc/*", "*/documentation/*"},
	},
	{
		name:         "markdown-general",
		category:     "docs",
		subcategory:  "documentation",
		importance:   "low",
		purpose:      "Markdown documentation",
		tags:         []string{"documentation", "markdown"},
		filePatterns: []string{"*.md"},
	},

	// Configuration files
	{
		name:         "env-file",
		category:     "config",
		subcategory:  "environment",
		importance:   "high",
		purpose:      "Environment configuration",
		tags:         []string{"config", "environment"},
		filePatterns: []string{".env", ".env.*", "*.env"},
	},
	{
		name:         "config-yaml",
		category:     "config",
		subcategory:  "configuration",
		importance:   "medium",
		purpose:      "YAML configuration file",
		tags:         []string{"config", "yaml"},
		filePatterns: []string{"config.yml", "config.yaml", "*.config.yml", "*.config.yaml"},
		pathPatterns: []string{"*/config/*"},
	},
	{
		name:         "config-toml",
		category:     "config",
		subcategory:  "configuration",
		importance:   "medium",
		purpose:      "TOML configuration file",
		tags:         []string{"config", "toml"},
		filePatterns: []string{"config.toml", "*.config.toml"},
	},
	{
		name:         "config-json",
		category:     "config",
		subcategory:  "configuration",
		importance:   "medium",
		purpose:      "JSON configuration file",
		tags:         []string{"config", "json"},
		filePatterns: []string{"config.json", "*.config.json", "settings.json"},
	},
	{
		name:         "editor-config",
		category:     "config",
		subcategory:  "editor",
		importance:   "low",
		purpose:      "Editor/IDE configuration",
		tags:         []string{"config", "editor"},
		filePatterns: []string{".editorconfig", ".prettierrc*", ".eslintrc*", ".stylelintrc*", "tsconfig.json", "jsconfig.json"},
		pathPatterns: []string{"*/.vscode/*", "*/.idea/*"},
	},
	{
		name:         "git-config",
		category:     "config",
		subcategory:  "vcs",
		importance:   "low",
		purpose:      "Git configuration",
		tags:         []string{"config", "git"},
		filePatterns: []string{".gitignore", ".gitattributes", ".gitmodules"},
	},

	// Source code - entrypoints (high priority source files)
	{
		name:         "go-main",
		category:     "source",
		subcategory:  "entrypoint",
		importance:   "high",
		purpose:      "Go application entry point",
		tags:         []string{"go", "entrypoint", "main"},
		filePatterns: []string{"main.go"},
		pathPatterns: []string{"*/cmd/*/main.go"},
	},
	{
		name:         "python-main",
		category:     "source",
		subcategory:  "entrypoint",
		importance:   "high",
		purpose:      "Python application entry point",
		tags:         []string{"python", "entrypoint"},
		filePatterns: []string{"main.py", "app.py", "__main__.py"},
	},
	{
		name:         "js-main",
		category:     "source",
		subcategory:  "entrypoint",
		importance:   "high",
		purpose:      "JavaScript/Node.js entry point",
		tags:         []string{"javascript", "entrypoint"},
		filePatterns: []string{"index.js", "index.ts", "main.js", "main.ts", "app.js", "app.ts", "server.js", "server.ts"},
	},

	// Interface/API definitions
	{
		name:         "protobuf",
		category:     "source",
		subcategory:  "interface",
		importance:   "high",
		purpose:      "Protocol Buffers definition",
		tags:         []string{"protobuf", "api", "interface"},
		filePatterns: []string{"*.proto"},
	},
	{
		name:         "graphql",
		category:     "source",
		subcategory:  "interface",
		importance:   "high",
		purpose:      "GraphQL schema definition",
		tags:         []string{"graphql", "api", "schema"},
		filePatterns: []string{"*.graphql", "*.gql"},
	},
	{
		name:         "openapi",
		category:     "source",
		subcategory:  "interface",
		importance:   "high",
		purpose:      "OpenAPI/Swagger specification",
		tags:         []string{"openapi", "swagger", "api"},
		filePatterns: []string{"*.swagger.json", "*.swagger.yaml", "openapi.json", "openapi.yaml", "swagger.json", "swagger.yaml"},
	},

	// Scripts
	{
		name:         "shell-script",
		category:     "script",
		subcategory:  "shell",
		importance:   "medium",
		purpose:      "Shell script",
		tags:         []string{"script", "shell", "automation"},
		filePatterns: []string{"*.sh", "*.bash"},
	},
	{
		name:         "powershell-script",
		category:     "script",
		subcategory:  "powershell",
		importance:   "medium",
		purpose:      "PowerShell script",
		tags:         []string{"script", "powershell", "automation"},
		filePatterns: []string{"*.ps1", "*.psm1"},
	},
	{
		name:         "batch-script",
		category:     "script",
		subcategory:  "batch",
		importance:   "medium",
		purpose:      "Windows batch script",
		tags:         []string{"script", "batch", "windows"},
		filePatterns: []string{"*.bat", "*.cmd"},
	},
	{
		name:         "scripts-directory",
		category:     "script",
		subcategory:  "automation",
		importance:   "medium",
		purpose:      "Automation script",
		tags:         []string{"script", "automation"},
		pathPatterns: []string{"*/scripts/*", "*/script/*"},
	},

	// Data files
	{
		name:         "sql-file",
		category:     "data",
		subcategory:  "database",
		importance:   "medium",
		purpose:      "SQL database file",
		tags:         []string{"data", "sql", "database"},
		filePatterns: []string{"*.sql"},
	},
	{
		name:         "data-directory",
		category:     "data",
		subcategory:  "dataset",
		importance:   "low",
		purpose:      "Data file",
		tags:         []string{"data"},
		pathPatterns: []string{"*/data/*", "*/fixtures/*", "*/seeds/*"},
	},
	{
		name:         "csv-data",
		category:     "data",
		subcategory:  "dataset",
		importance:   "low",
		purpose:      "CSV data file",
		tags:         []string{"data", "csv"},
		filePatterns: []string{"*.csv"},
	},

	// Source code - general (lower priority, catches remaining code files)
	{
		name:         "go-source",
		category:     "source",
		subcategory:  "implementation",
		importance:   "medium",
		purpose:      "Go source file",
		tags:         []string{"go", "source"},
		filePatterns: []string{"*.go"},
	},
	{
		name:         "python-source",
		category:     "source",
		subcategory:  "implementation",
		importance:   "medium",
		purpose:      "Python source file",
		tags:         []string{"python", "source"},
		filePatterns: []string{"*.py"},
	},
	{
		name:         "javascript-source",
		category:     "source",
		subcategory:  "implementation",
		importance:   "medium",
		purpose:      "JavaScript source file",
		tags:         []string{"javascript", "source"},
		filePatterns: []string{"*.js", "*.jsx"},
	},
	{
		name:         "typescript-source",
		category:     "source",
		subcategory:  "implementation",
		importance:   "medium",
		purpose:      "TypeScript source file",
		tags:         []string{"typescript", "source"},
		filePatterns: []string{"*.ts", "*.tsx"},
	},
	{
		name:         "rust-source",
		category:     "source",
		subcategory:  "implementation",
		importance:   "medium",
		purpose:      "Rust source file",
		tags:         []string{"rust", "source"},
		filePatterns: []string{"*.rs"},
	},
	{
		name:         "java-source",
		category:     "source",
		subcategory:  "implementation",
		importance:   "medium",
		purpose:      "Java source file",
		tags:         []string{"java", "source"},
		filePatterns: []string{"*.java"},
	},
	{
		name:         "cpp-source",
		category:     "source",
		subcategory:  "implementation",
		importance:   "medium",
		purpose:      "C/C++ source file",
		tags:         []string{"cpp", "source"},
		filePatterns: []string{"*.c", "*.cpp", "*.cc", "*.cxx", "*.h", "*.hpp", "*.hxx"},
	},
	{
		name:         "csharp-source",
		category:     "source",
		subcategory:  "implementation",
		importance:   "medium",
		purpose:      "C# source file",
		tags:         []string{"csharp", "source"},
		filePatterns: []string{"*.cs"},
	},
	{
		name:         "ruby-source",
		category:     "source",
		subcategory:  "implementation",
		importance:   "medium",
		purpose:      "Ruby source file",
		tags:         []string{"ruby", "source"},
		filePatterns: []string{"*.rb"},
	},
	{
		name:         "php-source",
		category:     "source",
		subcategory:  "implementation",
		importance:   "medium",
		purpose:      "PHP source file",
		tags:         []string{"php", "source"},
		filePatterns: []string{"*.php"},
	},
}

// Execute runs the rule-based classifier.
// This is a fast, deterministic classifier that uses pattern matching
// instead of LLM calls. Files that don't match any rule get category "unknown".
func (r *RuleClassifier) Execute(ctx context.Context, client *genai.Client, modelName string, input map[string]interface{}) (map[string]interface{}, error) {
	// Note: client and modelName are ignored - this classifier doesn't use LLM

	// Extract title (filename) - this is the primary input for classification
	title, _ := input["title"].(string)
	if title == "" {
		// Fall back to document_id if no title
		title, _ = input["document_id"].(string)
	}

	// Normalize path separators for cross-platform matching
	normalizedPath := strings.ReplaceAll(title, "\\", "/")

	// Try to match against rules in priority order
	for _, rule := range classificationRules {
		if r.matchesRule(normalizedPath, &rule) {
			return map[string]interface{}{
				"category":     rule.category,
				"subcategory":  rule.subcategory,
				"purpose":      rule.purpose,
				"importance":   rule.importance,
				"tags":         rule.tags,
				"rule_matched": rule.name,
			}, nil
		}
	}

	// No rule matched - return unknown
	return map[string]interface{}{
		"category":     "unknown",
		"subcategory":  "unclassified",
		"purpose":      "File purpose could not be determined by rules",
		"importance":   "medium",
		"tags":         []string{},
		"rule_matched": "",
	}, nil
}

// matchesRule checks if a path matches any pattern in the rule
func (r *RuleClassifier) matchesRule(path string, rule *classificationRule) bool {
	filename := filepath.Base(path)

	// Check file patterns (match against filename only)
	for _, pattern := range rule.filePatterns {
		if matched, _ := filepath.Match(pattern, filename); matched {
			return true
		}
		// Also try case-insensitive match for common files
		if matched, _ := filepath.Match(strings.ToLower(pattern), strings.ToLower(filename)); matched {
			return true
		}
	}

	// Check path patterns (match against full path)
	for _, pattern := range rule.pathPatterns {
		if matchPath(path, pattern) {
			return true
		}
	}

	// Check regex patterns
	for _, re := range rule.regexes {
		if re.MatchString(path) || re.MatchString(filename) {
			return true
		}
	}

	return false
}

// matchPath performs glob-style matching on a path
// Supports * (single segment) and ** (multiple segments) wildcards
func matchPath(path, pattern string) bool {
	// Simple glob matching - convert pattern to regex
	// * matches any non-separator characters
	// ** matches any characters including separators

	// Escape regex special characters except * and /
	escapedPattern := regexp.QuoteMeta(pattern)

	// Replace escaped \*\* with .* (match anything)
	escapedPattern = strings.ReplaceAll(escapedPattern, `\*\*`, `.*`)

	// Replace remaining escaped \* with [^/]* (match non-separator)
	escapedPattern = strings.ReplaceAll(escapedPattern, `\*`, `[^/]*`)

	// Anchor the pattern
	escapedPattern = "^" + escapedPattern + "$"

	re, err := regexp.Compile(escapedPattern)
	if err != nil {
		return false
	}

	return re.MatchString(path)
}

// GetType returns the agent type identifier for registration.
func (r *RuleClassifier) GetType() string {
	return "rule_classifier"
}

// SkipRateLimit returns true because rule_classifier does not make any external API calls.
// This allows it to process documents without waiting for rate limiting delays.
func (r *RuleClassifier) SkipRateLimit() bool {
	return true
}
