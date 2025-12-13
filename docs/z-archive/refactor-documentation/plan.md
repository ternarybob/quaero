# Plan: Update Worker Architecture Documentation

## Overview
Enhance the Manager/Worker Architecture documentation to explicitly document the `JobDefinitionOrchestrator` component. The orchestrator is currently referenced in file structure but lacks detailed explanation in Components & Interfaces section and sequence diagram.

## Steps

### Step 1: Update Document Metadata
- **Skill:** @none
- **Files:** docs/architecture/MANAGER_WORKER_ARCHITECTURE.md
- **User decision:** no
- **Actions:**
  - Update "Last Updated" date to current date (2025-01-16)

### Step 2: Add JobDefinitionOrchestrator to Component Responsibilities Table
- **Skill:** @none
- **Files:** docs/architecture/MANAGER_WORKER_ARCHITECTURE.md
- **User decision:** no
- **Actions:**
  - Insert new row in component table after line 52
  - Add JobDefinitionOrchestrator as 5th component

### Step 3: Add New JobDefinitionOrchestrator Documentation Section
- **Skill:** @none
- **Files:** docs/architecture/MANAGER_WORKER_ARCHITECTURE.md
- **User decision:** no
- **Actions:**
  - Insert complete new section after line 116
  - Include responsibilities, key methods, integration points
  - Add example flow code
  - Distinguish from StepManagers

### Step 4: Update Sequence Diagram
- **Skill:** @none
- **Files:** docs/architecture/MANAGER_WORKER_ARCHITECTURE.md
- **User decision:** no
- **Actions:**
  - Replace sequence diagram with updated version
  - Add Orchestrator participant
  - Update Phase 1 flow to show Handler → Orchestrator → Manager
  - Add clarifying Notes for Dequeue operation

### Step 5: Update Flow Phases Description
- **Skill:** @none
- **Files:** docs/architecture/MANAGER_WORKER_ARCHITECTURE.md
- **User decision:** no
- **Actions:**
  - Update Phase 1 to include orchestrator steps (2-5)
  - Add Phase 2 dequeue clarification
  - Renumber subsequent steps correctly

### Step 6: Update Implementations Section - Recategorize JobDefinitionOrchestrator
- **Skill:** @none
- **Files:** docs/architecture/MANAGER_WORKER_ARCHITECTURE.md
- **User decision:** no
- **Actions:**
  - Change from "Monitors" to new "Coordinators" category
  - Clarify distinct role vs JobMonitor

### Step 7: Update File Structure Section
- **Skill:** @none
- **Files:** docs/architecture/MANAGER_WORKER_ARCHITECTURE.md
- **User decision:** no
- **Actions:**
  - Update job_definition_orchestrator.go comment
  - Change section description to "Workflow coordination"

### Step 8: Update Architecture Benefits Section
- **Skill:** @none
- **Files:** docs/architecture/MANAGER_WORKER_ARCHITECTURE.md
- **User decision:** no
- **Actions:**
  - Add "Coordinators (1)" as first layer
  - Update counts and clarify responsibilities
  - Reflect 4 distinct architectural layers

## Success Criteria
- JobDefinitionOrchestrator explicitly documented with complete section
- Sequence diagram shows orchestrator layer between Handler and Manager
- Component table includes orchestrator as 5th component
- Flow phases description includes orchestrator role
- Recategorized from "Monitors" to "Coordinators"
- File structure and benefits sections updated to reflect new categorization
- All changes match the detailed requirements in docs/features/refactor-documentation/update-worker-arch.md

## Quality Standards
- Documentation follows existing style and structure
- Technical accuracy verified against implementation
- Mermaid diagrams render correctly
- All section numbering and references updated consistently
- Changes enhance clarity without altering architecture
