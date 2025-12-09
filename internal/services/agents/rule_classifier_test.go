package agents

import (
	"context"
	"testing"
)

func TestRuleClassifier_Execute(t *testing.T) {
	classifier := &RuleClassifier{}
	ctx := context.Background()

	tests := []struct {
		name             string
		title            string
		expectedCategory string
		expectedSubcat   string
		expectedRule     string
	}{
		// Test files
		{
			name:             "go test file",
			title:            "internal/queue/workers/agent_worker_test.go",
			expectedCategory: "test",
			expectedSubcat:   "unit-test",
			expectedRule:     "go-test",
		},
		{
			name:             "js test file",
			title:            "src/components/Button.test.tsx",
			expectedCategory: "test",
			expectedSubcat:   "unit-test",
			expectedRule:     "js-test",
		},
		{
			name:             "python test file",
			title:            "tests/test_api.py",
			expectedCategory: "test",
			expectedSubcat:   "unit-test",
			expectedRule:     "python-test",
		},
		{
			name:             "mock file",
			title:            "internal/mocks/storage_mock.go",
			expectedCategory: "test",
			expectedSubcat:   "mock",
			expectedRule:     "mock-file",
		},

		// CI/CD
		{
			name:             "github workflow",
			title:            ".github/workflows/ci.yml",
			expectedCategory: "ci",
			expectedSubcat:   "github-actions",
			expectedRule:     "github-workflow",
		},
		{
			name:             "gitlab ci",
			title:            ".gitlab-ci.yml",
			expectedCategory: "ci",
			expectedSubcat:   "gitlab-ci",
			expectedRule:     "gitlab-ci",
		},

		// Build files
		{
			name:             "dockerfile",
			title:            "Dockerfile",
			expectedCategory: "build",
			expectedSubcat:   "container",
			expectedRule:     "dockerfile",
		},
		{
			name:             "makefile",
			title:            "Makefile",
			expectedCategory: "build",
			expectedSubcat:   "build-system",
			expectedRule:     "makefile",
		},
		{
			name:             "go.mod",
			title:            "go.mod",
			expectedCategory: "build",
			expectedSubcat:   "dependency",
			expectedRule:     "go-mod",
		},
		{
			name:             "package.json",
			title:            "package.json",
			expectedCategory: "build",
			expectedSubcat:   "dependency",
			expectedRule:     "npm-package",
		},

		// Documentation
		{
			name:             "readme",
			title:            "README.md",
			expectedCategory: "docs",
			expectedSubcat:   "readme",
			expectedRule:     "readme",
		},
		{
			name:             "changelog",
			title:            "CHANGELOG.md",
			expectedCategory: "docs",
			expectedSubcat:   "changelog",
			expectedRule:     "changelog",
		},
		{
			name:             "license",
			title:            "LICENSE",
			expectedCategory: "docs",
			expectedSubcat:   "license",
			expectedRule:     "license",
		},

		// Config files
		{
			name:             "env file",
			title:            ".env.local",
			expectedCategory: "config",
			expectedSubcat:   "environment",
			expectedRule:     "env-file",
		},
		{
			name:             "gitignore",
			title:            ".gitignore",
			expectedCategory: "config",
			expectedSubcat:   "vcs",
			expectedRule:     "git-config",
		},

		// Source entrypoints
		{
			name:             "go main",
			title:            "cmd/server/main.go",
			expectedCategory: "source",
			expectedSubcat:   "entrypoint",
			expectedRule:     "go-main",
		},
		{
			name:             "js index",
			title:            "src/index.ts",
			expectedCategory: "source",
			expectedSubcat:   "entrypoint",
			expectedRule:     "js-main",
		},

		// Interface definitions
		{
			name:             "protobuf",
			title:            "api/service.proto",
			expectedCategory: "source",
			expectedSubcat:   "interface",
			expectedRule:     "protobuf",
		},
		{
			name:             "graphql",
			title:            "schema.graphql",
			expectedCategory: "source",
			expectedSubcat:   "interface",
			expectedRule:     "graphql",
		},

		// Scripts
		{
			name:             "shell script",
			title:            "scripts/deploy.sh",
			expectedCategory: "script",
			expectedSubcat:   "shell",
			expectedRule:     "shell-script",
		},
		{
			name:             "powershell",
			title:            "build.ps1",
			expectedCategory: "script",
			expectedSubcat:   "powershell",
			expectedRule:     "powershell-script",
		},

		// Source code
		{
			name:             "go source",
			title:            "internal/services/crawler.go",
			expectedCategory: "source",
			expectedSubcat:   "implementation",
			expectedRule:     "go-source",
		},

		// Unknown (should not match any rule)
		{
			name:             "random binary",
			title:            "data.bin",
			expectedCategory: "unknown",
			expectedSubcat:   "unclassified",
			expectedRule:     "",
		},
		{
			name:             "unknown extension",
			title:            "file.xyz",
			expectedCategory: "unknown",
			expectedSubcat:   "unclassified",
			expectedRule:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := map[string]interface{}{
				"document_id": "test-doc",
				"title":       tt.title,
				"content":     "test content",
			}

			result, err := classifier.Execute(ctx, nil, "", input)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}

			category := result["category"].(string)
			subcategory := result["subcategory"].(string)
			ruleMatched := result["rule_matched"].(string)

			if category != tt.expectedCategory {
				t.Errorf("category = %q, want %q", category, tt.expectedCategory)
			}
			if subcategory != tt.expectedSubcat {
				t.Errorf("subcategory = %q, want %q", subcategory, tt.expectedSubcat)
			}
			if ruleMatched != tt.expectedRule {
				t.Errorf("rule_matched = %q, want %q", ruleMatched, tt.expectedRule)
			}
		})
	}
}

func TestRuleClassifier_GetType(t *testing.T) {
	classifier := &RuleClassifier{}
	if got := classifier.GetType(); got != "rule_classifier" {
		t.Errorf("GetType() = %q, want %q", got, "rule_classifier")
	}
}
