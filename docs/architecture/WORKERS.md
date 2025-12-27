# Queue Workers Reference

This document describes all queue workers in `internal/queue/workers/`. Each worker processes specific job types and implements one or both of the worker interfaces.

## Table of Contents

- [Overview](#overview)
- [Worker Interfaces](#worker-interfaces)
- [Workers](#workers)
  - [Agent Worker](#agent-worker)
  - [Aggregate Summary Worker](#aggregate-summary-worker)
  - [Orchestrator Worker](#orchestrator-worker)
  - [Analyze Build Worker](#analyze-build-worker)
  - [ASX Announcements Worker](#asx-announcements-worker)
  - [Classify Worker](#classify-worker)
  - [Code Map Worker](#code-map-worker)
  - [Crawler Worker](#crawler-worker)
  - [Dependency Graph Worker](#dependency-graph-worker)
  - [Email Worker](#email-worker)
  - [GitHub Git Worker](#github-git-worker)
  - [GitHub Log Worker](#github-log-worker)
  - [GitHub Repo Worker](#github-repo-worker)
  - [Local Dir Worker](#local-dir-worker)
  - [Places Worker](#places-worker)
  - [Summary Worker](#summary-worker)
  - [Test Job Generator Worker](#test-job-generator-worker)
  - [Web Search Worker](#web-search-worker)
- [Configuration Reference](#configuration-reference)
- [Worker Classification](#worker-classification)

---

## Overview

Workers are the execution units of the Quaero job queue system. They process jobs created by job definitions and can be categorized into two types:

1. **DefinitionWorker**: Handles job definition steps, initializes work, and optionally creates child jobs
2. **JobWorker**: Executes individual queue jobs

Some workers implement both interfaces to support both orchestration and execution.

---

## Worker Interfaces

### DefinitionWorker Interface

Implements step-level orchestration for job definitions:

```go
type DefinitionWorker interface {
    GetType() string              // Returns worker type identifier
    Init(ctx, step, job) error    // Validation and preparation
    CreateJobs(ctx, step, job) (string, error)  // Job creation/execution
    ReturnsChildJobs() bool       // Whether child jobs are created
    ValidateConfig(config) error  // Config validation
}
```

### JobWorker Interface

Implements individual job execution:

```go
type JobWorker interface {
    GetWorkerType() string        // Returns job type string
    Validate(job) error           // Job validation
    Execute(ctx, job) error       // Job execution logic
}
```

---

## Workers

### Agent Worker

**File**: `agent_worker.go`

**Purpose**: Unified AI-powered document processing. Acts as a bridge between document processing pipelines and AI agent services (keyword extraction, summarization, entity recognition, etc.).

**Interfaces**: DefinitionWorker, JobWorker

**Job Type**: `"agent"`

#### Inputs

**Step Config** (from job definition):
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `agent_type` | string | Yes | Agent type to use |
| `api_key` | string | No | KV store placeholder like `{google_api_key}` |
| `filter_source_type` | string | No | Filter documents by source type |
| `filter_tags` | []string | No | Filter documents by tags |
| `filter_limit` | int | No | Maximum documents to process |
| `filter_created_after` | string | No | Filter by creation date |
| `filter_updated_after` | string | No | Filter by update date |

**Queue Job Config**:
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `document_id` | string | Yes | ID of document to process |
| `agent_type` | string | Yes | Type of agent to run |
| `max_keywords` | int | No | Parameter for keyword agent |
| `gemini_api_key` | string | No | Override API key |
| `gemini_model` | string | No | Override model name |
| `gemini_timeout` | duration | No | Override timeout |
| `gemini_rate_limit` | duration | No | Override rate limit |

**Agent Types**:
- `keyword_extractor` - Extract keywords from documents
- `document_generator` - Generate new documents
- `web_enricher` - Enrich documents with web data
- `content_summarizer` - Summarize document content
- `metadata_enricher` - Enrich document metadata
- `sentiment_analyzer` - Analyze document sentiment
- `entity_recognizer` - Recognize named entities
- `category_classifier` - Classify document categories
- `relation_extractor` - Extract entity relations
- `question_answerer` - Answer questions about documents

#### Outputs

- Updates document metadata under key matching agent_type
- Publishes `DocumentUpdated` event
- Logs execution status

#### Configuration

```yaml
gemini:
  google_api_key: "your-api-key"      # Required
  agent_model: "gemini-3-flash-preview"   # Model for agents
  timeout: "5m"                        # Operation timeout
  rate_limit: "4s"                     # Rate limit between requests
```

#### Example Job Definition

```toml
[step.extract_keywords]
type = "agent"
description = "Extract keywords from documents"
agent_type = "keyword_extractor"
filter_tags = ["imported"]
```

---

### Aggregate Summary Worker

**File**: `aggregate_summary_worker.go`

**Purpose**: Aggregates enrichment metadata from processed documents and generates a comprehensive DevOps summary using LLM.

**Interfaces**: DefinitionWorker

**Job Type**: N/A (inline execution only)

#### Inputs

**Step Config**:
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `filter_tags` | []string | No | Tags for filtering enriched documents |

#### Outputs

- Creates comprehensive DevOps summary document
- Stores aggregated enrichment data
- Summary includes: build targets, dependencies, platforms, CI/CD recommendations

#### Configuration

Requires LLM service to be available in the service container.

#### Example Job Definition

```toml
[step.aggregate_devops_summary]
type = "aggregate_summary"
description = "Generate actionable DevOps summary and CI/CD guide"
depends = "build_dependency_graph"
filter_tags = ["devops-candidate"]
```

---

### Orchestrator Worker

**File**: `orchestrator_worker.go`

**Purpose**: AI-powered cognitive orchestration using LLM reasoning to dynamically plan and execute workflows based on natural language goals. Implements the **Planner-Executor-Reviewer** pattern for intelligent task decomposition and goal-directed execution. This is the "thinking" component that makes decisions about what to do, as opposed to the `JobDispatcher` which handles mechanical job dispatch.

**Interfaces**: DefinitionWorker

**Job Type**: N/A (inline execution, spawns child jobs via tool execution)

#### Architecture: Planner-Executor-Reviewer Loop

The OrchestratorWorker operates in three phases:

```
┌─────────────────────────────────────────────────────────────────────┐
│                    ORCHESTRATOR EXECUTION LOOP                       │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  1. PLANNER PHASE                                                   │
│     ├── Read variables from job definition [config]                │
│     ├── Format available tools for LLM                             │
│     └── LLM generates structured Plan with steps                   │
│                                                                     │
│  2. EXECUTOR PHASE                                                  │
│     ├── Validate each step references valid tool                   │
│     ├── Execute steps (respecting dependencies)                    │
│     └── Collect results for each step                              │
│                                                                     │
│  3. REVIEWER PHASE                                                  │
│     ├── LLM assesses if goal was achieved                          │
│     ├── Returns confidence score (0.0-1.0)                         │
│     └── Suggests recovery actions if needed                        │
│                                                                     │
│  RECOVERY LOOP (max 3 iterations)                                   │
│     └── If goal not achieved, retry with recovery actions          │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

#### Inputs

**Job-Level Config** (in `[config]` section):
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `variables` | []map | No | User data (stocks, holdings, etc.) following the job-definitions pattern |
| `benchmarks` | []map | No | Benchmark indices for comparison (portfolio jobs) |

**Step Config** (in `[step.X]` section):
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `goal` | string | Yes | Natural language description of desired outcome |
| `available_tools` | []map | No | Workers exposed as callable tools (each must have `name` field) |
| `thinking_level` | string | No | MINIMAL, LOW, MEDIUM, HIGH (default: MEDIUM) |
| `model_preference` | string | No | Model selection (default: auto). See Model Selection table below. |

**Model Selection** (`model_preference` values):

The orchestrator supports both Gemini and Claude models. Use `model_preference` to select the LLM:

| Value | Model Used | Provider | Use Case |
|-------|------------|----------|----------|
| `auto` (default) | Provider default | Config-dependent | Uses `llm.default_provider` from config |
| `flash`, `gemini-flash` | `gemini-3-flash-preview` | Gemini | Fast, cost-effective planning |
| `pro`, `gemini-pro` | `gemini-3-pro-preview` | Gemini | Advanced reasoning |
| `claude`, `claude-sonnet`, `sonnet` | `claude-sonnet-4-5-20250929` | Claude | Balanced Claude model |
| `claude-opus`, `opus` | `claude-opus-4-5-20251101` | Claude | Most capable Claude model |
| `claude-haiku`, `haiku` | `claude-haiku-4-5-20251001` | Claude | Fast, efficient Claude model |

**ThinkingLevel Values**:
| Level | Use Case |
|-------|----------|
| MINIMAL | Quick planning, minimal reasoning |
| LOW | Light reasoning, fast execution |
| MEDIUM | Balanced reasoning (default) |
| HIGH | Deep reasoning, thorough analysis |

**Available Tools Format**:
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Tool identifier for LLM to reference |
| `description` | string | Yes | Human-readable description of tool capability |
| `worker` | string | Yes | Worker type to invoke (e.g., `asx_stock_data`, `summary`) |
| `template` | string | No | Template name if worker is `job_template` |

#### Data Structures

**Plan** (generated by Planner):
```go
type Plan struct {
    Reasoning string     `json:"reasoning"`     // LLM's reasoning for the plan
    Steps     []PlanStep `json:"steps"`         // Ordered execution steps
}

type PlanStep struct {
    ID        string                 `json:"id"`          // Unique step identifier
    Tool      string                 `json:"tool"`        // Tool name from available_tools
    Params    map[string]interface{} `json:"params"`      // Parameters for tool
    DependsOn []string               `json:"depends_on"`  // Step IDs this depends on
}
```

**ReviewResult** (generated by Reviewer):
```go
type ReviewResult struct {
    GoalAchieved    bool                     `json:"goal_achieved"`    // Whether goal was met
    Confidence      float64                  `json:"confidence"`       // 0.0-1.0 confidence score
    Summary         string                   `json:"summary"`          // Assessment summary
    MissingData     []string                 `json:"missing_data"`     // What data was missing
    RecoveryActions []map[string]interface{} `json:"recovery_actions"` // Suggested recovery steps
}
```

#### Outputs

- Creates planning document with reasoning steps
- Executes tools via existing workers (asx_stock_data, summary, web_search, etc.)
- Creates final result document with execution summary
- Logs each phase's progress and decisions

#### Configuration

Requires LLM service (Gemini/Claude) to be configured.

#### Variables Pattern

Variables provide context to the LLM Planner. They are declared in the job-level `[config]` section following the standard job-definitions pattern where user data lives in the job definition (not external files).

**Variables Format** (in `[config]` section):
```toml
[config]
variables = [
    { ticker = "GNP", name = "GenusPlus Group Ltd", industry = "infrastructure", units = 1000, avg_price = 5.00, weighting = 50.0 },
    { ticker = "SKS", name = "SKS Technologies Group", industry = "technology-infrastructure", units = 500, avg_price = 4.00, weighting = 50.0 },
]

benchmarks = [
    { code = "XJO", name = "S&P/ASX 200" },
    { code = "XSO", name = "S&P/ASX Small Ordinaries" },
]
```

The orchestrator reads these variables and formats them as context for the LLM Planner. This follows the job-definitions pattern where:
- **Job definitions** contain user data (variables) and process flow (steps)
- **Job templates** contain reusable components referenced by steps

#### Example Job Definition

**Basic orchestrator job**:
```toml
id = "asx-stocks-daily-orchestrated"
name = "ASX Daily Stock Analysis (Thinking)"
type = "orchestrator"
description = "AI-powered stock analysis"

# Variables defined at job level - follows job-definitions pattern
[config]
variables = [
    { ticker = "GNP", name = "GenusPlus Group Ltd", industry = "infrastructure" },
]

[step.analyze_stocks]
type = "orchestrator"
description = "AI-powered planning and execution"
goal = "Analyze all stocks in the variables list and generate recommendations"
available_tools = [
    { name = "fetch_stock_data", description = "Fetch ASX stock data", worker = "asx_stock_data" },
    { name = "run_analysis", description = "Generate summary analysis", worker = "summary" },
    { name = "search_web", description = "Search the web", worker = "web_search" }
]
thinking_level = "HIGH"
model_preference = "auto"  # Or: "opus", "sonnet", "flash", "pro"
```

**Complete orchestrated job definition with portfolio holdings**:
```toml
id = "smsf-portfolio-daily-orchestrated"
name = "SMSF Portfolio Strategy Review (Thinking)"
type = "orchestrator"
description = "AI-powered SMSF portfolio analysis with strategic reasoning"
tags = ["smsf", "portfolio", "daily", "orchestrator", "thinking"]
timeout = "3h"
enabled = true

# Variables at job level - user data follows job-definitions pattern
[config]
variables = [
    { ticker = "GNP", name = "GenusPlus Group Ltd", industry = "infrastructure", units = 3159, avg_price = 6.303, weighting = 5.97 },
    { ticker = "SKS", name = "SKS Technologies Group", industry = "technology-infrastructure", units = 4925, avg_price = 4.025, weighting = 6.01 },
]

benchmarks = [
    { code = "XJO", name = "S&P/ASX 200" },
    { code = "XSO", name = "S&P/ASX Small Ordinaries" },
]

[step.strategic_review]
type = "orchestrator"
description = "AI-powered strategic portfolio review"
goal = """
Perform a comprehensive SMSF portfolio review for all holdings in the variables list.
For each holding, calculate current valuation and P/L, collect benchmark data,
perform stock analysis, and generate portfolio-level insights.
"""
thinking_level = "HIGH"
model_preference = "opus"  # Use Claude Opus for deep reasoning
available_tools = [
    { name = "fetch_stock_data", description = "Fetch ASX stock prices", worker = "asx_stock_data" },
    { name = "fetch_announcements", description = "Fetch ASX announcements", worker = "asx_announcements" },
    { name = "search_web", description = "Search financial news", worker = "web_search" },
    { name = "analyze_summary", description = "Generate LLM analysis", worker = "summary" },
    { name = "run_stock_review", description = "Execute stock review template", worker = "job_template", template = "asx-stock-review" }
]

[step.email_portfolio]
type = "email"
description = "Email consolidated SMSF portfolio review"
depends = "strategic_review"
to = "user@example.com"
subject = "SMSF Portfolio Review - AI-Powered Strategic Analysis"
body_from_tag = "smsf-portfolio-review"
```

#### Directory Structure

Orchestrator job definitions follow the standard job-definitions pattern:
- `deployments/common/job-definitions/` - Production job definitions
- `test/config/job-definitions/` - Test job definitions
- `bin/job-definitions/` - UAT/development (untracked)

Job templates (reusable components) are stored in:
- `deployments/common/job-templates/` - Production templates
- `test/config/job-templates/` - Test templates
- `bin/job-templates/` - UAT/development (untracked)

All three locations should maintain configuration parity.

---

### Analyze Build Worker

**File**: `analyze_build_worker.go`

**Purpose**: Analyzes build system files (CMake, Makefile, vcxproj, etc.) to extract build targets, compiler flags, and linked libraries using LLM.

**Interfaces**: DefinitionWorker

**Job Type**: N/A (inline execution only)

#### Inputs

**Step Config**:
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `filter_tags` | []string | No | Tags for filtering documents |
| `force` | bool | No | Force re-analysis even if already processed |

#### Outputs

- Updates document metadata with build system analysis
- Extracts: targets, dependencies, compiler flags
- Tracks analysis in `enrichment_passes`

#### Configuration

Requires LLM service to be available.

#### Example Job Definition

```toml
[step.analyze_build_system]
type = "analyze_build"
description = "Analyze Makefiles, CMake, and other build files"
depends = "extract_structure"
filter_tags = ["devops-candidate"]
```

---

### ASX Announcements Worker

**File**: `asx_announcements_worker.go`

**Purpose**: Fetches ASX (Australian Securities Exchange) company announcements from the official ASX website. Parses the announcements page and stores each announcement as an individual document with metadata including PDF links, price sensitivity, and date information.

**Interfaces**: DefinitionWorker

**Job Type**: N/A (inline execution only)

#### Inputs

**Step Config**:
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `asx_code` | string | Yes | ASX company code (e.g., "GNP", "BHP") |
| `period` | string | No | Time period for announcements (default: "Y1" = 1 year). Options: D1 (1 day), W1 (1 week), M1 (1 month), M3 (3 months), M6 (6 months), Y1 (1 year), Y5 (5 years) |
| `limit` | int | No | Maximum number of announcements to fetch (default: 50) |
| `output_tags` | []string | No | Additional tags to apply to output documents |

**Period Options**:
| Value | Description | Use Case |
|-------|-------------|----------|
| D1 | Last 1 day | Daily monitoring |
| W1 | Last 1 week | Weekly review |
| M1 | Last 1 month | Monthly review |
| M3 | Last 3 months | Quarterly analysis |
| M6 | Last 6 months | Half-yearly review |
| Y1 | Last 1 year | Annual analysis (default) |
| Y5 | Last 5 years | Long-term trend analysis, FY results history |

#### Outputs

- Individual documents for each announcement with tags: `["asx-announcement", "{asx_code}", "date:YYYY-MM-DD", ...output_tags]`
- If price sensitive, adds tag: `"price-sensitive"`
- Metadata includes: asx_code, headline, announcement_date, price_sensitive, pdf_url, pdf_filename, parent_job_id, file_details
- Document content in markdown format with headline, date, company code, price sensitivity status, and PDF link

#### Document Structure

Each announcement is stored as a markdown document:

```markdown
# ASX Announcement: {Headline}

**Date**: 2 January 2006 3:04 PM
**Company**: ASX:GNP
**Price Sensitive**: Yes ⚠️

**Document**: [filename.pdf](https://www.asx.com.au/...)
**Details**: 5 pages, 1.2 MB

---
*Full announcement available at PDF link above*
```

#### Configuration

No additional configuration required. The worker fetches announcements from the public ASX website.

#### Example Job Definition

```toml
[step.fetch_announcements]
type = "asx_announcements"
description = "Fetch official ASX announcements for GNP"
on_error = "continue"
asx_code = "GNP"
period = "M6"
limit = 20
output_tags = ["asx-gnp-search", "gnp"]
```

---

### ASX Stock Data Worker

**File**: `asx_stock_data_worker.go`

**Purpose**: Fetches real-time and historical stock data from the ASX. Uses Markit Digital API for fundamentals and Yahoo Finance for OHLCV data. Provides accurate price data and calculates technical indicators for analysis summaries.

**Interfaces**: DefinitionWorker

**Job Type**: N/A (inline execution only)

#### Inputs

**Step Config**:
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `asx_code` | string | Yes | ASX company code (e.g., "BHP", "CBA") or index code (e.g., "XJO", "XSO") |
| `period` | string | No | Historical data period (default: "Y1" = 1 year). Options: M1 (1 month), M3 (3 months), M6 (6 months), Y1 (1 year), Y2 (2 years), Y5 (5 years) |
| `output_tags` | []string | No | Additional tags to apply to output documents |
| `cache_hours` | int | No | Hours to cache data before refresh (default: 24) |
| `force_refresh` | bool | No | Force data refresh ignoring cache (default: false) |

**Period Options**:
| Value | Description | Use Case |
|-------|-------------|----------|
| M1 | Last 1 month | Short-term technical analysis |
| M3 | Last 3 months | Quarterly performance |
| M6 | Last 6 months | Half-year trends |
| Y1 | Last 1 year | Annual analysis (default) |
| Y2 | Last 2 years | Medium-term performance |
| Y5 | Last 5 years | Long-term CAGR, growth trajectory |

#### Outputs

- Document with comprehensive stock data including:
  - Current price, bid/ask, price change
  - Day range, 52-week range
  - Volume and average volume
  - Market cap, P/E ratio, EPS, dividend yield
  - Historical OHLCV data (last 365 days)
  - Technical indicators: SMA20, SMA50, SMA200, RSI14
  - Support/resistance levels
  - Trend signal (bullish/bearish/neutral)
- Tags: `["asx-stock-data", "{asx_code}", "date:YYYY-MM-DD", ...output_tags]`

#### Configuration

No additional configuration required. Fetches data from public APIs.

#### Example Job Definition

```toml
[step.fetch_stock_data]
type = "asx_stock_data"
description = "Fetch real-time stock data for CBA"
asx_code = "CBA"
output_tags = ["banking-sector", "portfolio"]
```

---

### Competitor Analysis Worker

**File**: `competitor_analysis_worker.go`

**Purpose**: Analyzes a target company and identifies ASX-listed competitors using LLM. Automatically fetches stock data for each identified competitor using the ASXStockDataWorker.

**Interfaces**: DefinitionWorker

**Job Type**: N/A (inline execution only)

#### Inputs

**Step Config**:
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `asx_code` | string | Yes | Target company ASX code |
| `prompt` | string | No | Custom prompt for competitor identification (default: identifies top 3-5 competitors) |
| `api_key` | string | Yes | Gemini API key or KV store placeholder like `{google_api_key}` |
| `output_tags` | []string | No | Additional tags to apply to output documents |

#### Outputs

- Document summarizing competitor analysis
- Individual stock data documents for each identified competitor
- Tags: `["competitor-analysis", "{asx_code}", ...output_tags]`

#### Configuration

Requires Gemini API key for LLM-based competitor identification.

#### Example Job Definition

```toml
[step.analyze_competitors]
type = "competitor_analysis"
description = "Identify and analyze competitors for BHP"
asx_code = "BHP"
api_key = "{google_api_key}"
output_tags = ["mining-sector"]
```

---

### Classify Worker

**File**: `classify_worker.go`

**Purpose**: LLM-based classification of C/C++ files. Classifies file roles, identifies components, detects test types/frameworks, and identifies external dependencies.

**Interfaces**: DefinitionWorker

**Job Type**: N/A (inline execution only)

#### Inputs

**Step Config**:
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `filter_tags` | []string | No | Tags for filtering documents |
| `force` | bool | No | Force re-classification |

#### Outputs

- Updates document metadata with DevOps classification
- Stores: file roles, components, test types, dependencies
- Tracks in `enrichment_passes`

#### Configuration

Requires LLM service to be available.

#### Example Job Definition

```toml
[step.classify_devops]
type = "classify"
description = "LLM-based classification of file roles and components"
depends = "extract_structure"
filter_tags = ["devops-candidate"]
```

---

### Code Map Worker

**File**: `code_map_worker.go`

**Purpose**: Creates hierarchical code structure maps optimized for large codebases (2GB+, 10k+ files). Stores project summaries, directory aggregations, and file metadata instead of full content.

**Interfaces**: DefinitionWorker, JobWorker

**Job Types**: `"code_map_structure"`, `"code_map_summary"`

#### Inputs

**Step Config**:
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `dir_path` or `path` | string | Yes | Directory to analyze |
| `project_name` | string | No | Project name for documents |
| `max_depth` | int | No | Maximum directory depth (default: 10) |
| `skip_summarization` | bool | No | Skip AI summarization (default: false) |

**Queue Job Config**:
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `base_path` | string | Yes | Base directory path |
| `dir_path` | string | Yes | Current directory path |
| `project_name` | string | No | Project name |
| `tags` | []string | No | Tags to apply to documents |

#### Outputs

- Documents for: project summary, directories, files
- Metadata includes: file count, LOC, languages, children, exports, imports
- MD5 hashing for change detection

#### Configuration

Automatically excludes common build/cache directories:
`.git`, `node_modules`, `vendor`, `__pycache__`, `.venv`, `venv`, `dist`, `build`, `target`, `.gradle`, `.mvn`, `.idea`, `.vscode`, `.vs`, `bin`, `obj`, `out`, `.next`, `.nuxt`, `coverage`

#### Example Job Definition

```toml
[step.map_codebase]
type = "code_map"
description = "Create code structure map"
dir_path = "/path/to/project"
project_name = "my-project"
max_depth = 10
```

---

### Crawler Worker

**File**: `crawler_worker.go`

**Purpose**: Unified web crawler with ChromeDP rendering. Supports JavaScript rendering, content extraction, and link discovery for recursive crawling.

**Interfaces**: DefinitionWorker, JobWorker

**Job Type**: `"crawler_url"`

#### Inputs

**Step Config**:
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `entity_type` | string | No | "issues", "pages", "all" |
| `start_urls` | []string | No | Array of URLs to start from |
| All crawler config options | various | No | See Configuration |

**Queue Job Config**:
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `seed_url` | string | Yes | URL to crawl |
| `source_type` | string | Yes | "web", "jira", "confluence", etc. |
| `entity_type` | string | Yes | Type of entity to extract |
| `crawl_config` | object | Yes | Full crawl configuration |

#### Outputs

- Documents for each crawled page
- Extracts markdown content, metadata, links
- Creates child jobs for discovered URLs (respecting depth)

#### Configuration

```yaml
crawler:
  user_agent: "Mozilla/5.0 Chrome/120.0.0.0"
  user_agent_rotation: false
  max_concurrency: 3          # Requests per domain
  request_delay: "1s"         # Delay between requests
  random_delay: "500ms"       # Jitter
  request_timeout: "30s"      # HTTP timeout
  max_body_size: 10485760     # 10MB max response
  max_depth: 5                # Maximum crawl depth
  follow_robots_txt: true
  output_format: "markdown"   # "markdown", "html", or "both"
  only_main_content: true     # Extract main content only
  include_links: true
  include_metadata: true
  enable_javascript: true     # Enable ChromeDP rendering
  javascript_wait_time: "3s"  # Wait for JS rendering
```

#### Example Job Definition

```toml
[step.crawl_docs]
type = "crawler"
description = "Crawl documentation site"
start_urls = ["https://docs.example.com"]
max_depth = 3
enable_javascript = true
```

---

### Dependency Graph Worker

**File**: `dependency_graph_worker.go`

**Purpose**: Builds dependency graphs from DevOps metadata. Creates a graph showing file dependencies based on includes, library links, and build dependencies.

**Interfaces**: DefinitionWorker

**Job Type**: N/A (inline execution only)

#### Inputs

**Step Config**:
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `filter_tags` | []string | No | Tags for filtering documents |

#### Outputs

- Creates dependency graph document
- Updates `enrichment_passes` tracking
- Stores graph relationships in documents

#### Configuration

Uses document storage and search services.

#### Example Job Definition

```toml
[step.build_dependency_graph]
type = "dependency_graph"
description = "Build dependency graph from extracted metadata"
depends = "extract_structure, classify_devops"
filter_tags = ["devops-candidate"]
```

---

### Email Worker

**File**: `email_worker.go`

**Purpose**: Sends email notifications with job results. Used as a step in job definitions to email results/summaries to users.

**Interfaces**: DefinitionWorker

**Job Type**: N/A (inline execution only)

#### Inputs

**Step Config**:
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `to` | string | Yes | Recipient email address |
| `subject` | string | No | Email subject (default: "Quaero Job Results") |
| `body` | string | No | Plain text email body |
| `body_html` | string | No | HTML email body |
| `body_from_document` | string | No | Document ID to use as email body (markdown auto-converted to HTML) |
| `body_from_tag` | string | No | Get latest document with tag as email body (markdown auto-converted to HTML) |

#### Outputs

- Sends email to specified recipient (HTML formatted with professional styling)
- Markdown content is automatically converted to styled HTML with headings, lists, code blocks, tables
- Logs success/failure to job logs

#### Prerequisites

Requires SMTP configuration in Settings > Email.

#### Example Job Definition

```toml
[step.notify]
type = "email"
description = "Send job results via email"

[step.notify.config]
to = "team@example.com"
subject = "Daily Report Complete"
body = "The daily report job has completed successfully."
```

---

### Email Watcher Worker

**File**: `email_watcher_worker.go`

**Purpose**: Monitors email inbox for job execution commands. Reads IMAP emails with subject containing 'quaero' and parses job execution requests. Enables remote job triggering via email.

**Interfaces**: DefinitionWorker

**Job Type**: N/A (inline execution only)

#### Inputs

**Step Config**:
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| No specific config required | - | - | Uses system IMAP configuration |

#### Email Format

Emails with subject containing 'quaero' are processed. The email body should contain job execution commands.

#### Outputs

- Parses incoming emails for job requests
- Executes matching job definitions
- Logs processed emails and execution results

#### Prerequisites

Requires IMAP configuration in Settings:
- IMAP server host
- IMAP port
- Username and password
- Folder to monitor (default: INBOX)

#### Example Job Definition

```toml
[step.check_emails]
type = "email_watcher"
description = "Monitor inbox for job execution commands"
```

---

### GitHub Git Worker

**File**: `github_git_worker.go`

**Purpose**: Clones GitHub repositories via git command (faster than API for bulk downloads). Processes files in batches.

**Interfaces**: DefinitionWorker, JobWorker

**Job Type**: `"github_git_batch"`

#### Inputs

**Queue Job Config**:
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `owner` | string | Yes | Repository owner |
| `repo` | string | Yes | Repository name |
| `branch` | string | Yes | Branch to clone |
| `clone_dir` | string | Yes | Directory where repo was cloned |
| `batch_idx` | int | Yes | Batch index |
| `files` | []object | Yes | Array of file objects |
| `tags` | []string | No | Tags to apply to documents |

**File Object**:
| Field | Type | Description |
|-------|------|-------------|
| `path` | string | File path in repository |
| `folder` | string | Folder grouping |

#### Outputs

- Documents for each file in batch
- Stores file content as document markdown
- Applies tags and metadata

#### Configuration

Uses system temp directory for clones (`quaero-git-clones`).

---

### GitHub Log Worker

**File**: `github_log_worker.go`

**Purpose**: Processes GitHub Action workflow run logs.

**Interfaces**: DefinitionWorker, JobWorker

**Job Type**: `"github_action_log"`

#### Inputs

**Queue Job Config**:
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `owner` | string | Yes | Repository owner |
| `repo` | string | Yes | Repository name |
| `run_id` | int | Yes | GitHub Actions run ID |
| `workflow_name` | string | No | Workflow name |
| `run_started_at` | string | No | Run start timestamp |
| `branch` | string | No | Branch name |
| `commit_sha` | string | No | Commit SHA |
| `conclusion` | string | No | Run conclusion status |

#### Outputs

- Document containing workflow run logs
- Metadata: owner, repo, run_id, workflow_name, branch, conclusion
- Tags: `["github", "actions", repo, conclusion]`

#### Configuration

Requires GitHub connector to be configured.

---

### GitHub Repo Worker

**File**: `github_repo_worker.go`

**Purpose**: Processes individual GitHub repository files via API.

**Interfaces**: DefinitionWorker, JobWorker

**Job Type**: `"github_repo_file"`

#### Inputs

**Queue Job Config**:
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `owner` | string | Yes | Repository owner |
| `repo` | string | Yes | Repository name |
| `branch` | string | Yes | Branch name |
| `path` | string | Yes | File path in repository |
| `folder` | string | No | Folder grouping |
| `sha` | string | No | Commit SHA |

#### Outputs

- Document containing file content
- Metadata: owner, repo, branch, folder, path, sha, file_type
- Tags: `["github", repo, branch]`

#### Configuration

Requires GitHub connector (30-second timeout for file fetch).

---

### Local Dir Worker

**File**: `local_dir_worker.go`

**Purpose**: Indexes local filesystem directories. Scans directory and indexes files as documents for AI processing.

**Interfaces**: DefinitionWorker, JobWorker

**Job Type**: `"local_dir_batch"`

#### Inputs

**Queue Job Config**:
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `base_path` | string | Yes | Base directory path |
| `batch_idx` | int | Yes | Batch index |
| `files` | []object | Yes | Array of file objects |
| `tags` | []string | No | Tags to apply |

**File Object**:
| Field | Type | Description |
|-------|------|-------------|
| `path` | string | Relative file path |
| `folder` | string | Folder grouping |
| `absolute_path` | string | Full file path |
| `extension` | string | File extension |
| `file_size` | int | File size in bytes |
| `file_type` | string | File type |

#### Outputs

- Documents for each file in batch
- Stores file content as document markdown
- Applies tags and metadata

#### Configuration

Uses document storage service.

---

### Places Worker

**File**: `places_worker.go`

**Purpose**: Executes Google Places API search operations.

**Interfaces**: DefinitionWorker

**Job Type**: N/A (inline execution only)

#### Inputs

**Step Config**:
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `search_query` | string | Yes | Search query |
| `search_type` | string | Yes | "text_search" or "nearby_search" |
| `api_key` | string | No | API key or KV store placeholder |

#### Outputs

- Documents for each place result
- Metadata includes: place details, ratings, location
- Tags: `["places", searchQuery]`

#### Configuration

```yaml
places_api:
  api_key: "your-google-places-api-key"   # Required
  rate_limit: "1s"                         # Between requests
  request_timeout: "30s"                   # HTTP timeout
  max_results_per_search: 20               # Results per request
```

#### Example Job Definition

```toml
[step.search_places]
type = "places"
description = "Search for restaurants"
search_query = "restaurants in Melbourne"
search_type = "text_search"
```

---

### Summary Worker

**File**: `summary_worker.go`

**Purpose**: Generates comprehensive summaries from tagged document collections using Gemini LLM.

**Interfaces**: DefinitionWorker

**Job Type**: N/A (inline execution only)

#### Inputs

**Step Config**:
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `prompt` | string | Yes | Natural language instruction for summary |
| `filter_tags` | []string | Yes | Tags for filtering documents to include |
| `api_key` | string | No | Gemini API key |
| `output_tags` | []string | No | Additional tags to apply to the output document (useful for downstream steps) |
| `thinking_level` | string | No | Reasoning depth: MINIMAL, LOW, MEDIUM, HIGH. Use HIGH for complex analysis. |

#### Outputs

- Creates new summary document with tags: `["summary", job-name-slug, ...job.Tags, ...output_tags]`
- Stores aggregated content from source documents
- Metadata: source_document_count, source_tags, model_used

#### Configuration

```yaml
gemini:
  google_api_key: "your-api-key"      # Required
  chat_model: "gemini-3-flash-preview"    # Model for summaries
  temperature: 0.7                     # Generation temperature
```

#### Thinking Levels

The `thinking_level` parameter controls how deeply the model reasons about the task:

| Level | Use Case | Token Budget |
|-------|----------|--------------|
| LOW | Standard summaries | Low |
| HIGH | Complex analysis, recommendations | Highest |

**Note**: Gemini 3 Pro supports LOW and HIGH. Gemini 3 Flash additionally supports MINIMAL and MEDIUM.

#### Example Job Definition

```toml
[step.generate_summary]
type = "summary"
description = "Summarize project documentation"
prompt = "Create a comprehensive summary of the project architecture and key components"
filter_tags = ["documentation", "imported"]
```

**Example with thinking_level for complex analysis:**

```toml
[step.investment_recommendation]
type = "summary"
description = "Generate investment recommendation"
prompt = "Analyze the stock data and generate a BUY/SELL/HOLD recommendation"
filter_tags = ["stock-data", "announcements"]
api_key = "{google_gemini_api_key}"
thinking_level = "HIGH"
```

---

### Test Job Generator Worker

**File**: `test_job_generator_worker.go`

**Purpose**: Generates test jobs with random logs, warnings, and errors for testing the logging system, error tolerance, and job hierarchy. Supports recursive child job creation.

**Interfaces**: DefinitionWorker, JobWorker

**Job Type**: `"test_job_generator"`

#### Inputs

**Step Config**:
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `worker_count` | int | No | Number of parallel worker jobs (default: 10) |
| `log_count` | int | No | Number of log entries per job (default: 100) |
| `log_delay_ms` | int | No | Delay between logs in ms (default: 50) |
| `failure_rate` | float | No | Probability of job failure 0-1 (default: 0.1) |
| `child_count` | int | No | Number of child jobs per job (default: 2) |
| `recursion_depth` | int | No | Maximum recursion depth (default: 3) |

#### Outputs

- Generates log entries with random distribution: 80% INFO, 15% WARN, 5% ERROR
- Creates child jobs recursively up to specified depth
- Jobs may fail based on failure_rate probability

#### Use Cases

- Testing job logging and log viewer UI
- Testing error tolerance and partial failure handling
- Testing job hierarchy and parent-child relationships
- Stress testing the queue system

#### Example Job Definition

```toml
[step.test_logging]
type = "test_job_generator"
description = "Generate test jobs with random errors"

[step.test_logging.config]
worker_count = 5
log_count = 50
log_delay_ms = 100
failure_rate = 0.2
child_count = 2
recursion_depth = 2
```

---

### Job Template Worker

**File**: `job_template_worker.go`

**Purpose**: Executes job templates with variable substitution. Loads templates from `{exe}/job-templates/`, applies variable replacements using `{namespace:key}` syntax, and executes the resulting job definitions. Supports both sequential and parallel execution.

**Interfaces**: DefinitionWorker

**Job Type**: N/A (inline execution, spawns child jobs)

#### Inputs

**Step Config**:
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `template` | string | Yes | Template name (without .toml extension) |
| `variables` | array | No* | Array of variable objects for substitution |
| `parallel` | bool | No | Execute instances in parallel (default: false) |

*Variables can be declared at job level in `[config]` section and inherited by steps.

**Template Config** (in template's `[config]` section):
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `singleton` | bool | No | If true, template runs once regardless of parent variables |

#### Variable Substitution

Templates use `{namespace:key}` syntax for variable placeholders:
- `{stock:ticker}` - Replaced with ticker value from variables
- `{stock:ticker_lower}` - Lowercase version
- `{stock:ticker_upper}` - Uppercase version

**Variables Format** (step-level):
```toml
[step.run_analysis]
type = "job_template"
template = "stock-analysis"
variables = [
    { ticker = "CBA", name = "Commonwealth Bank", industry = "banking" },
    { ticker = "BHP", name = "BHP Group", industry = "mining" }
]
```

**Global Variables** (job-level, inherited by all steps):
```toml
[config]
variables = [
    { ticker = "CBA", name = "Commonwealth Bank", industry = "banking" },
    { ticker = "BHP", name = "BHP Group", industry = "mining" }
]

[step.run_analysis]
type = "job_template"
template = "stock-analysis"
# Variables inherited from [config] section

[step.email_reports]
type = "job_template"
template = "stock-email"
depends = "run_analysis"
# Variables inherited from [config] section
```

**Variable Inheritance Rules**:
- Template `singleton = true` takes precedence - runs once, parent variables inherited for data access only
- Step-level variables override job-level variables for iteration
- Omitting variables in step config = inherit from job level for iteration
- `variables = false` in step explicitly opts out of variable iteration (backward compatibility)

#### Outputs

- Executes job definition for each variable set
- Creates child jobs visible in job hierarchy
- Logs execution progress for each instance

#### Configuration

Templates must be located in `{exe}/job-templates/` directory as TOML files.

#### Example Job Definition

**Iterating Template** (runs once per variable set):
```toml
[step.run_stock_analysis]
type = "job_template"
description = "Run stock analysis for multiple companies"
template = "asx-stock-analysis"
parallel = true
variables = [
    { ticker = "CBA", name = "Commonwealth Bank" },
    { ticker = "NAB", name = "National Australia Bank" },
    { ticker = "WBC", name = "Westpac Banking" }
]
```

**Singleton Template** (template declares it runs once):
```toml
# In the template file (portfolio-summary.toml):
[config]
singleton = true  # Template runs once, not iterating over variables

# In the job definition:
[step.generate_summary]
type = "job_template"
template = "portfolio-summary"
depends = "run_stock_analysis"
# No variables needed - template declares singleton mode
# Parent config.variables still inherited for data access within template steps
```

---

### Web Search Worker

**File**: `web_search_worker.go`

**Purpose**: Performs web search operations using Gemini SDK with GoogleSearch grounding.

**Interfaces**: DefinitionWorker

**Job Type**: N/A (inline execution only)

#### Inputs

**Step Config**:
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `query` | string | Yes | Search query |
| `depth` | int | No | Search depth for follow-ups (1-10, default: 1) |
| `breadth` | int | No | Results per search (1-5, default: 3) |
| `api_key` | string | No | Gemini API key |
| `output_tags` | []string | No | Additional tags to apply to output documents (useful for downstream steps) |

#### Outputs

- Documents with search results tagged with: `["web_search", ...job.Tags, "date:YYYY-MM-DD", ...output_tags]`
- Automatic date tag for filtering documents by search date
- Metadata: query, sources, result_count, depth, breadth, search_queries
- Sources stored as array with URL and title

#### Configuration

Requires Gemini API key (same as Summary Worker).

#### Example Job Definition

```toml
[step.research_topic]
type = "web_search"
description = "Research the topic"
query = "Kubernetes best practices 2024"
depth = 2
breadth = 5
```

---

## Configuration Reference

### Gemini Configuration

Used by: Agent Worker, Summary Worker, Web Search Worker

```yaml
gemini:
  google_api_key: "your-api-key"      # Required - Gemini API key
  agent_model: "gemini-3-flash-preview"   # Model for agent operations
  chat_model: "gemini-3-flash-preview"    # Model for chat/summaries
  max_turns: 10                        # Max agent conversation turns
  timeout: "5m"                        # Operation timeout
  rate_limit: "4s"                     # Rate limit between requests
  temperature: 0.7                     # Generation temperature
```

### Crawler Configuration

Used by: Crawler Worker

```yaml
crawler:
  user_agent: "Mozilla/5.0 Chrome/120.0.0.0"
  user_agent_rotation: false          # Enable random UA rotation
  max_concurrency: 3                  # Max requests per domain
  request_delay: "1s"                 # Minimum delay between requests
  random_delay: "500ms"               # Jitter to add
  request_timeout: "30s"              # HTTP timeout
  max_body_size: 10485760             # Max response size (10MB)
  max_depth: 5                        # Maximum crawl depth
  follow_robots_txt: true             # Respect robots.txt
  output_format: "markdown"           # "markdown", "html", or "both"
  only_main_content: true             # Extract main content only
  include_links: true                 # Include discovered links
  include_metadata: true              # Extract metadata
  enable_javascript: true             # Enable ChromeDP rendering
  javascript_wait_time: "3s"          # Wait for JS rendering
```

### Places API Configuration

Used by: Places Worker

```yaml
places_api:
  api_key: "your-google-places-api-key"
  rate_limit: "1s"                    # Minimum between requests
  request_timeout: "30s"              # HTTP timeout
  max_results_per_search: 20          # Results per request
```

### Search Configuration

Used by: Agent Worker (and others using search service)

```yaml
search:
  mode: "advanced"                    # "fts5", "advanced", or "disabled"
  case_sensitive_multiplier: 3        # Multiplier for case-sensitive searches
  case_sensitive_max_cap: 500         # Max results cap
```

---

## Worker Classification

### By Processing Strategy

**Parallel Processing** (Create Child Jobs):
- Agent Worker - Creates jobs per document
- Crawler Worker - Creates jobs per discovered URL
- Code Map Worker - Creates structure/summary jobs
- GitHub Git Worker - Processes file batches
- Job Template Worker - Spawns template-based child jobs
- Local Dir Worker - Processes file batches
- Test Job Generator Worker - Creates recursive child jobs

**Inline Processing** (Synchronous):
- Aggregate Summary Worker - Single aggregation task
- Analyze Build Worker - Process documents inline
- ASX Announcements Worker - Fetch ASX company announcements
- ASX Stock Data Worker - Fetch stock prices and indicators
- Classify Worker - Process documents inline
- Competitor Analysis Worker - Identify and analyze competitors
- Dependency Graph Worker - Single graph build
- Email Worker - Send email notification
- Email Watcher Worker - Monitor inbox for commands
- GitHub Log Worker - Process individual logs
- GitHub Repo Worker - Process individual files
- Places Worker - Single search execution
- Summary Worker - Single summary generation
- Web Search Worker - Single search execution

### By Interface

| Worker | DefinitionWorker | JobWorker |
|--------|------------------|-----------|
| Agent | Yes | Yes |
| Aggregate Summary | Yes | No |
| Analyze Build | Yes | No |
| ASX Announcements | Yes | No |
| ASX Stock Data | Yes | No |
| Classify | Yes | No |
| Code Map | Yes | Yes |
| Competitor Analysis | Yes | No |
| Crawler | Yes | Yes |
| Dependency Graph | Yes | No |
| Email | Yes | No |
| Email Watcher | Yes | No |
| GitHub Git | Yes | Yes |
| GitHub Log | Yes | Yes |
| GitHub Repo | Yes | Yes |
| Job Template | Yes | No |
| Local Dir | Yes | Yes |
| Orchestrator | Yes | No |
| Places | Yes | No |
| Summary | Yes | No |
| Test Job Generator | Yes | Yes |
| Web Search | Yes | No |

### By Category

**AI/LLM Workers**:
- Agent Worker
- Aggregate Summary Worker
- Analyze Build Worker
- Classify Worker
- Competitor Analysis Worker
- Orchestrator Worker
- Summary Worker
- Web Search Worker

**Data Source Workers**:
- ASX Announcements Worker
- ASX Stock Data Worker
- Crawler Worker
- GitHub Git Worker
- GitHub Log Worker
- GitHub Repo Worker
- Local Dir Worker
- Places Worker

**Enrichment Workers**:
- Dependency Graph Worker

**Notification Workers**:
- Email Worker
- Email Watcher Worker

**Orchestration Workers**:
- Job Template Worker

**Utility Workers**:
- Code Map Worker
- Test Job Generator Worker
