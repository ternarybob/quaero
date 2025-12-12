# Architecture Documentation

This directory contains comprehensive architecture documentation for Quaero.

## Primary Documents

### [Manager/Worker Architecture](MANAGER_WORKER_ARCHITECTURE.md)
**Comprehensive documentation of Quaero's job system using Manager/Worker pattern**

The Manager/Worker Architecture document provides the definitive guide to understanding Quaero's job orchestration and execution system. This consolidates and replaces three previous architecture documents (JOB_EXECUTOR_ARCHITECTURE.md, JOB_QUEUE_MANAGEMENT.md, QUEUE_ARCHITECTURE.md) that used confusing "executor" terminology.

**Topics Covered:**
- Clear separation between orchestration (Managers) and execution (Workers)
- Manager responsibilities: Create parent jobs, define workflows, spawn child jobs
- Worker responsibilities: Execute individual jobs, process content, save results
- Orchestrator responsibilities: Monitor parent job progress, aggregate statistics
- Complete job execution flow with sequence diagrams
- Interface definitions (JobManager, JobWorker, JobMonitor)
- File structure organization (manager/, worker/, monitor/)
- Database schema (jobs table, job_logs table, queue table)
- Real-time WebSocket events for progress tracking
- Configuration examples and API endpoints
- Troubleshooting guide for common issues
- Migration plan from old "executor" terminology

**When to Read:**
- Understanding Quaero's job system architecture
- Working with crawler jobs, agent jobs, or any async workflows
- Implementing new job types (managers, workers, or monitors)
- Debugging job execution issues
- Learning about parent-child job hierarchies
- Understanding real-time progress tracking

### [Markdown + Metadata Architecture](architecture.md)
**Comprehensive documentation of Quaero's document storage design**

This document explains Quaero's canonical format strategy for storing content from diverse sources (Jira, Confluence, GitHub) as clean Markdown with structured metadata JSON.

**Topics Covered:**
- Design philosophy: Why Markdown + Metadata separation
- Content flow pipeline (HTML → Markdown + Metadata → Storage)
- Document model and metadata schemas (Jira, Confluence, GitHub)
- HTML to Markdown conversion process
- HTML parsing details (generic scraping vs specialized transformers)
- Storage implementation with SQLite
- Two-step query pattern (filter by metadata, reason from markdown)
- FTS5 full-text search indexes
- Immediate document saving during crawling
- Crawler service logging
- Known limitations and future enhancements

**When to Read:**
- Understanding how documents are stored and indexed
- Working with document transformers or scrapers
- Implementing new data sources
- Debugging markdown conversion issues
- Optimizing search queries
- Understanding metadata extraction logic

## Navigation Guide

**If you want to understand:**
- **Job monitoring and execution** → Read [Manager/Worker Architecture](MANAGER_WORKER_ARCHITECTURE.md)
- **Document storage and search** → Read [Markdown + Metadata Architecture](architecture.md)
- **How crawler jobs work** → Read both documents (Manager/Worker for job flow, Markdown+Metadata for content processing)
- **Real-time progress tracking** → Read [Manager/Worker Architecture](MANAGER_WORKER_ARCHITECTURE.md) WebSocket section
- **Metadata extraction** → Read [Markdown + Metadata Architecture](architecture.md) HTML parsing section
- **Adding a new job type** → Read [Manager/Worker Architecture](MANAGER_WORKER_ARCHITECTURE.md) implementation guide
- **Adding a new data source** → Read [Markdown + Metadata Architecture](architecture.md) transformers section

## Related Documentation

- **Main README:** [../../README.md](../../README.md) - Project overview, setup, and usage
- **Agent Guidelines:** [../../AGENTS.md](../../AGENTS.md) - AI assistant instructions and conventions
- **MCP Documentation:** `docs/implement-mcp-server/` - Model Context Protocol integration
- **Feature Documentation:** `docs/features/` - Specific feature implementation details

## Queue System Documentation (For AI Refactoring)

The following documents provide comprehensive guidance for AI agents refactoring the queue path:

| Document | Purpose |
|----------|---------|
| [Manager/Worker Architecture](manager_worker_architecture.md) | Core architecture, job hierarchy, data flow |
| [Queue Logging](QUEUE_LOGGING.md) | Logging flow, WebSocket events, log entry schema |
| [Queue UI](QUEUE_UI.md) | Frontend architecture, Alpine.js components, icon standards |
| [Queue Services](QUEUE_SERVICES.md) | Supporting services, event system, initialization order |
| [Workers Reference](workers.md) | Complete worker documentation (17+ workers) |

**Reading Order for Refactoring:**
1. Start with `manager_worker_architecture.md` for overall understanding
2. Read the specific document for the area you're modifying
3. Check `workers.md` if implementing or modifying workers

## Document History

| Document | Version | Last Updated | Status |
|----------|---------|--------------|--------|
| MANAGER_WORKER_ARCHITECTURE.md | 2.0 | 2025-12-12 | ✅ Current - Comprehensive architecture with AI refactoring guidance |
| QUEUE_LOGGING.md | 1.0 | 2025-12-12 | ✅ Current - Logging architecture for queue system |
| QUEUE_UI.md | 1.0 | 2025-12-12 | ✅ Current - UI architecture for queue management |
| QUEUE_SERVICES.md | 1.0 | 2025-12-12 | ✅ Current - Supporting services documentation |
| workers.md | 1.0 | 2025-12-12 | ✅ Current - Complete worker reference |
| architecture.md | - | 2024-11-06 | ✅ Current - Markdown + Metadata architecture |
| ~~JOB_EXECUTOR_ARCHITECTURE.md~~ | - | - | ❌ Deleted - Consolidated into MANAGER_WORKER_ARCHITECTURE.md |
| ~~JOB_QUEUE_MANAGEMENT.md~~ | - | - | ❌ Deleted - Consolidated into MANAGER_WORKER_ARCHITECTURE.md |
| ~~QUEUE_ARCHITECTURE.md~~ | - | - | ❌ Deleted - Consolidated into MANAGER_WORKER_ARCHITECTURE.md |

**Migration Note:** Architecture documentation was consolidated into MANAGER_WORKER_ARCHITECTURE.md for better maintainability. All references to the old fragmented docs should point to this unified document.

## Contributing to Architecture Docs

When updating architecture documentation:

1. **Keep Manager/Worker Architecture focused on job system** - Don't add document storage details
2. **Keep Markdown+Metadata Architecture focused on content** - Don't add job orchestration details
3. **Update this README** when adding new architecture documents
4. **Update AGENTS.md** when changing architecture patterns
5. **Include diagrams** using Mermaid syntax for complex flows
6. **Provide examples** for all concepts (code snippets, SQL queries, JSON payloads)
7. **Document troubleshooting** for common issues
8. **Maintain backward compatibility notes** when making breaking changes

## Questions or Feedback

For questions about the architecture:
- Review the appropriate document first
- Check the troubleshooting sections
- Consult AGENTS.md for development guidelines
- Open an issue in the repository for clarifications or improvements
