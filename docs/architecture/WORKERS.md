# Queue Workers Reference

This document describes all queue workers in `internal/queue/workers/`. Each worker processes specific job types and implements one or both of the worker interfaces.

## Table of Contents

- [Overview](#overview)
- [Worker Interfaces](#worker-interfaces)
- [Workers](#workers)
  - [Agent Worker](#agent-worker)
  - [Aggregate Summary Worker](#aggregate-summary-worker)
  - [Analyze Build Worker](#analyze-build-worker)
  - [Classify Worker](#classify-worker)
  - [Code Map Worker](#code-map-worker)
  - [Crawler Worker](#crawler-worker)
  - [Database Maintenance Worker](#database-maintenance-worker)
  - [Dependency Graph Worker](#dependency-graph-worker)
  - [Extract Structure Worker](#extract-structure-worker)
  - [GitHub Git Worker](#github-git-worker)
  - [GitHub Log Worker](#github-log-worker)
  - [GitHub Repo Worker](#github-repo-worker)
  - [Local Dir Worker](#local-dir-worker)
  - [Places Worker](#places-worker)
  - [Summary Worker](#summary-worker)
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
  agent_model: "gemini-2.0-flash"     # Model for agents
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

### Database Maintenance Worker

**File**: `database_maintenance_worker.go`

**Purpose**: Processes database maintenance operations. **Note: Deprecated** - BadgerDB handles maintenance automatically. All operations are no-ops.

**Interfaces**: JobWorker

**Job Type**: `"database_maintenance_operation"`

#### Inputs

**Queue Job Config**:
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `operation` | string | Yes | "vacuum", "analyze", "reindex", "optimize" |

#### Outputs

None (all operations are no-ops in BadgerDB)

#### Configuration

None required.

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

### Extract Structure Worker

**File**: `extract_structure_worker.go`

**Purpose**: Extracts C/C++ code structure including includes, defines, conditionals, and platform-specific code using regex patterns.

**Interfaces**: DefinitionWorker

**Job Type**: N/A (inline execution only)

#### Inputs

**Step Config**:
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `filter_tags` | []string | No | Tags for filtering documents |
| `force` | bool | No | Force re-extraction |

#### Outputs

- Updates document metadata with:
  - `local_includes` - Project-local includes
  - `system_includes` - System/library includes
  - `defines` - Preprocessor definitions
  - `conditionals` - ifdef/ifndef symbols
  - `platforms` - Detected platforms (windows, linux, macos, embedded)
- Adds `devops-enriched` tag to processed documents
- Tracks in `enrichment_passes`

#### Configuration

Uses document storage and search services.

#### Example Job Definition

```toml
[step.extract_structure]
type = "extract_structure"
description = "Extract includes, defines, and platform info from C/C++ files"
filter_tags = ["devops-candidate"]
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

#### Outputs

- Creates new summary document
- Stores aggregated content from source documents
- Metadata: source_document_count, source_tags, model_used

#### Configuration

```yaml
gemini:
  google_api_key: "your-api-key"      # Required
  chat_model: "gemini-2.0-flash"      # Model for summaries
  temperature: 0.7                     # Generation temperature
```

#### Example Job Definition

```toml
[step.generate_summary]
type = "summary"
description = "Summarize project documentation"
prompt = "Create a comprehensive summary of the project architecture and key components"
filter_tags = ["documentation", "imported"]
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

#### Outputs

- Documents with search results
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
  agent_model: "gemini-2.0-flash"     # Model for agent operations
  chat_model: "gemini-2.0-flash"      # Model for chat/summaries
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
- Local Dir Worker - Processes file batches

**Inline Processing** (Synchronous):
- Aggregate Summary Worker - Single aggregation task
- Analyze Build Worker - Process documents inline
- Classify Worker - Process documents inline
- Dependency Graph Worker - Single graph build
- Extract Structure Worker - Process documents inline
- GitHub Log Worker - Process individual logs
- GitHub Repo Worker - Process individual files
- Places Worker - Single search execution
- Summary Worker - Single summary generation
- Web Search Worker - Single search execution

**Deprecated**:
- Database Maintenance Worker - All operations are no-ops

### By Interface

| Worker | DefinitionWorker | JobWorker |
|--------|------------------|-----------|
| Agent | Yes | Yes |
| Aggregate Summary | Yes | No |
| Analyze Build | Yes | No |
| Classify | Yes | No |
| Code Map | Yes | Yes |
| Crawler | Yes | Yes |
| Database Maintenance | No | Yes |
| Dependency Graph | Yes | No |
| Extract Structure | Yes | No |
| GitHub Git | Yes | Yes |
| GitHub Log | Yes | Yes |
| GitHub Repo | Yes | Yes |
| Local Dir | Yes | Yes |
| Places | Yes | No |
| Summary | Yes | No |
| Web Search | Yes | No |

### By Category

**AI/LLM Workers**:
- Agent Worker
- Aggregate Summary Worker
- Analyze Build Worker
- Classify Worker
- Summary Worker
- Web Search Worker

**Data Source Workers**:
- Crawler Worker
- GitHub Git Worker
- GitHub Log Worker
- GitHub Repo Worker
- Local Dir Worker
- Places Worker

**Enrichment Workers**:
- Extract Structure Worker
- Dependency Graph Worker

**Utility Workers**:
- Code Map Worker
- Database Maintenance Worker (deprecated)
