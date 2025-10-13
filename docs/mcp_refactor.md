Quaero Refactoring Plan: From RAG to AI Agent
1. Introduction and Intent
1.1. Current Application Intent
An analysis of the quaero codebase reveals its primary purpose: to be a self-hosted, local-first knowledge base assistant. It connects to enterprise data sources like Jira and Confluence, scrapes their content, stores it locally in a SQLite database, and uses a local LLM (via an Ollama-compatible service) to answer user questions about the ingested content.

The current architecture, visible in internal/services/chat/chat_service.go, employs a traditional Retrieval-Augmented Generation (RAG) model. It performs a semantic or keyword search to find relevant documents, "stuffs" their content into the context window of an LLM, and asks the LLM to generate an answer from that context.

1.2. Strategic Goal of this Refactor
The core limitation of the current RAG model is its passive nature. The quality of the answer is entirely dependent on the quality of the initial, pre-emptive document search.

This refactor will pivot quaero to a more intelligent and dynamic AI Agent model. Instead of pushing a fixed context to the LLM, the agent will be empowered to actively "pull" information on demand. It will reason about a user's query, formulate a plan, and execute a series of tool calls to gather precisely the information it needs.

The outcome will be a system that is more accurate, transparent, and powerful, capable of answering complex, multi-hop questions that the RAG model would fail to address. Furthermore, the agent's reasoning process will be streamed to the user in real-time, creating a more interactive and trustworthy experience.

2. Architectural Analysis and Refactoring Strategy
The existing codebase is well-structured with a service-oriented design (internal/interfaces, internal/services), which makes it an ideal candidate for a refactor rather than a rewrite. The core server, configuration, LLM communication, and WebSocket layers can be preserved.

The refactoring will focus on three key areas:

Data Layer: The highly specific atlassian scraper services will be replaced with a generic, Go-native crawling service that leverages the existing cookie-based authentication. The SQLite FTS5 backend will be enhanced to serve as the agent's primary search tool.

Core Logic: The ChatService will be completely overhauled. The RAG logic will be removed and replaced with a new conversation loop that orchestrates the AI agent's planning and tool-use cycle.

Communication Layer: The existing WebSocket handler will be repurposed to stream the agent's intermediate thoughts and actions to the frontend.

3. Stepped Implementation Plan for AI CLI
This plan provides a sequence of actionable instructions to refactor the quaero application.

Stage 1: Overhaul the Data Layer
Goal: Simplify data ingestion and consolidate search functionality within the SQLite storage layer.

Step 1.1: Implement an Authenticated, Crawling Scraper Service
Action: Replace the specialized internal/services/atlassian/ data scrapers with a single, generic, in-process crawling service that leverages the existing cookie-based authentication mechanism. The functionality should be modeled after the deep crawling and content extraction capabilities of Firecrawl.

Instruction:

Preserve Authentication Service: The file internal/services/atlassian/auth_service.go is critical as it manages the authentication cookies provided by the Chrome extension. This service must be preserved. Relocate it to a new, more appropriate location: internal/services/auth/service.go. Update its package declaration and any import paths that refer to it.

Create New Crawler Service: Create a new directory internal/services/crawler/ and within it, a file service.go.

Define Crawler Service and Dependencies: Define a Service interface and a crawlerService struct. This service must depend on the AuthService and the DocumentService (for saving data). Its constructor (NewService) must accept these dependencies.

// internal/services/crawler/service.go
type Service interface {
    Crawl(ctx context.Context, startURL string, maxDepth int) error
}

type crawlerService struct {
    authService     auth.Service
    documentService document.Service
    // ... other fields like an authenticated http.Client
}

Implement Authenticated HTTP Requests: Inside the crawlerService, create a helper method that uses the authService to retrieve the necessary cookies and constructs an authenticated http.Client. All HTTP requests made by the crawler must use this client to ensure access to protected Jira and Confluence pages.

Implement Core Crawling Logic: Implement the Crawl method. This method should function like a web crawler. For a concrete reference on implementation, study the approach used by Firecrawl (https://github.com/firecrawl/firecrawl). The implementation must:

Maintain a queue of URLs to visit and a map of already visited URLs to prevent infinite loops.

Start with the startURL, fetching the page using the authenticated client.

Use Go's HTML parsing libraries to extract the main readable content and convert it to clean Markdown.

Parse the page for new hyperlinks (<a> tags) that are within the scope of the original site (e.g., if starting on my-jira.atlassian.net/browse/PROJ, it should follow links to other issues in PROJ but not to external sites).

Add discovered, in-scope links to the visit queue, respecting the maxDepth parameter.

For each successfully scraped page, create a models.Document and save it to the database using the documentService.

Update the Scheduler: Modify the internal/services/scheduler/scheduler_service.go to use this new CrawlerService. The scheduler's tasks should now be configured to call crawlerService.Crawl with the base URLs for Jira projects and Confluence spaces.

Cleanup: Once the new CrawlerService and relocated AuthService are fully integrated and working, delete the now-empty internal/services/atlassian/ directory.

Step 1.2: Enhance SQLite Storage to Serve as the Agent's Toolset
Action: Fortify the existing DocumentStorage to provide the specific search functions the AI agent will need.

Instruction:

Review the database schema in internal/storage/sqlite/schema.go. Ensure that the documents table is backed by an FTS5 virtual table for high-performance full-text search.

In internal/storage/sqlite/document_storage.go, implement a set of new public methods that will be exposed as tools to the agent:

SearchByText(ctx context.Context, query string, sources []string, limit int) ([]*models.Document, error): Performs a full-text search using FTS5 queries.

GetByID(ctx context.Context, docID string) (*models.Document, error): Fetches a single document by its primary key.

SearchByReference(ctx context.Context, referenceKey string, limit int) ([]*models.Document, error): Searches for documents that contain a specific reference identifier (e.g., "BUG-123") in their content or metadata.

Stage 2: Implement the MCP Agent Framework
Goal: Build the internal components that allow the LLM to function as an agent.

Step 2.1: Define the MCP Data Contracts
Action: Create the data structures for agent-tool communication.

Instruction:

Create a new file internal/models/mcp.go.

In this file, define the data structures that will govern tool calls: MCPRequest (containing Tool and Arguments) and MCPResponse (containing Status, Result, and Error).

Step 2.2: Create the Internal MCP Tool Router
Action: Implement an internal dispatcher that executes tool calls from the agent.

Instruction:

Create a new file internal/services/mcp/router.go.

Define a ToolRouter struct that holds a reference to the DocumentService.

Implement a method ExecuteTool(req *models.MCPRequest) *models.MCPResponse.

This method will use a switch req.Tool statement to call the appropriate method on the DocumentService (e.g., s.documentService.SearchByText(...)) based on the tool name. It will then format the result into an MCPResponse. This router is called directly from Go code, not exposed as an HTTP endpoint.

Stage 3: Rebuild the Chat Service into a Streaming Agent Orchestrator
Goal: Transform the core chat logic from a single-shot RAG process to an iterative, streaming agent loop.

Step 3.1: Create the Agent's "Constitution" (System Prompt)
Action: Write a detailed system prompt that defines the agent's behavior, available tools, and required output format.

Instruction:

Create a new file internal/services/chat/prompts.go.

Add a constant AgentSystemPrompt containing the rules of engagement for the LLM. This prompt must clearly list the available tools (search_documents, get_document_by_id, etc.) and instruct the LLM to respond with a JSON object for tool calls.

Step 3.2: Re-implement the Chat Service with a Streaming Agent Loop
Action: This is the central task. Rewrite the Chat method in internal/services/chat/chat_service.go to manage the agent's lifecycle.

Instruction:

Modify the Chat method signature to accept a WebSocket connection object for streaming.

Remove all existing RAG-related logic.

Implement a new conversation loop that (for a maximum of N turns):
a.  Sends the conversation history (including the system prompt) to the LLMService.
b.  Receives the LLM's response.
c.  Parses the response: Differentiates between a final answer and a tool call request.
d.  Streams the State: Sends a structured message (e.g., {"type": "thought", "content": "..."}) over the WebSocket to inform the UI of its current state or action.
e.  Executes Tools: If a tool call is requested, use the MCP Tool Router from Stage 2 to execute it.
f.  Streams the Result: Sends the tool's result back to the UI as an observation message.
g.  Appends the result to the conversation history and continues the loop.

Once the LLM provides a final answer, stream it to the UI and terminate the loop.

Stage 4: Upgrade the Frontend for Real-Time Interaction
Goal: Modify the UI to display the agent's real-time thought process.

Step 4.1: Enhance the WebSocket Client
Action: Update the client-side JavaScript to handle the new variety of messages from the agent.

Instruction:

In pages/static/websocket-manager.js, modify the onmessage event handler.

The handler should now parse incoming JSON messages and check for a type field (thought, action, observation, final_answer).

Based on the type, it should call different rendering functions.

Step 4.2: Update the Chat UI for Streaming Display
Action: Add a new UI component to display the agent's status.

Instruction:

In pages/chat.html, add a new container element before the main chat message area, e.g., <div id="agent-status" class="agent-status-box"></div>.

In your JavaScript, write functions to append formatted HTML for each message type into the #agent-status container. For example, a "thought" could be rendered as a light-grey text block, while an "observation" could be a collapsible block showing the data returned from a tool.

When a message with type: "final_answer" is received, clear the #agent-status container and render the final message into the main chat history as normal.

4. Expected Outcome
Upon completion of this refactor, quaero will be transformed into a fundamentally more capable application. It will feature a simplified and more robust backend, a transparent and interactive user interface, and an intelligent core that can reason and dynamically seek out information to solve complex user queries. The final deliverable will be a single, self-contained Go binary that serves as a powerful, local-first AI research assistant.