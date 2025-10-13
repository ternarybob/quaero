package chat

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
)

// ChatService implements RAG-enabled chat functionality
type ChatService struct {
	llmService       interfaces.LLMService
	documentService  interfaces.DocumentService
	embeddingService interfaces.EmbeddingService
	logger           arbor.ILogger
}

// NewChatService creates a new chat service
func NewChatService(
	llmService interfaces.LLMService,
	documentService interfaces.DocumentService,
	embeddingService interfaces.EmbeddingService,
	logger arbor.ILogger,
) *ChatService {
	return &ChatService{
		llmService:       llmService,
		documentService:  documentService,
		embeddingService: embeddingService,
		logger:           logger,
	}
}

// Chat implements the ChatService interface
func (s *ChatService) Chat(ctx context.Context, req *interfaces.ChatRequest) (*interfaces.ChatResponse, error) {
	ragEnabled := req.RAGConfig != nil && req.RAGConfig.Enabled
	s.logger.Debug().
		Str("message", req.Message).
		Str("rag_enabled", fmt.Sprintf("%v", ragEnabled)).
		Msg("Processing chat request")

	// Set default RAG config if not provided
	ragConfig := req.RAGConfig
	if ragConfig == nil {
		ragConfig = &interfaces.RAGConfig{
			Enabled:       true,
			MaxDocuments:  5,
			MinSimilarity: 0.7,
			SearchMode:    interfaces.SearchModeVector,
		}
	}

	var contextDocs []*models.Document
	var contextText string

	// Retrieve relevant documents if RAG is enabled
	if ragConfig.Enabled {
		docs, err := s.retrieveContext(ctx, req.Message, ragConfig)
		if err != nil {
			s.logger.Warn().Err(err).Msg("Failed to retrieve context documents")
			// Continue without context rather than failing
		} else {
			contextDocs = docs
			contextText = s.buildContextText(docs)
		}
	}

	// Build messages for LLM
	messages := s.buildMessages(req, contextText)

	// Generate response
	response, err := s.llmService.Chat(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("failed to generate response: %w", err)
	}

	// Build response
	chatResponse := &interfaces.ChatResponse{
		Message:     response,
		ContextDocs: contextDocs,
		TokenUsage:  nil, // TODO: Implement token counting
		Model:       s.embeddingService.ModelName(),
		Mode:        s.llmService.GetMode(),
	}

	s.logger.Info().
		Int("context_docs", len(contextDocs)).
		Str("mode", string(chatResponse.Mode)).
		Msg("Chat request completed")

	return chatResponse, nil
}

// retrieveContext retrieves relevant documents for RAG
func (s *ChatService) retrieveContext(
	ctx context.Context,
	query string,
	config *interfaces.RAGConfig,
) ([]*models.Document, error) {
	s.logger.Debug().Msg("Starting RAG context retrieval")

	// Generate query embedding
	s.logger.Debug().Msg("Generating query embedding")
	embedding, err := s.embeddingService.GenerateQueryEmbedding(ctx, query)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to generate query embedding for RAG")
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}
	s.logger.Debug().Int("embedding_dim", len(embedding)).Msg("Query embedding generated")

	// Build search query
	searchQuery := &interfaces.SearchQuery{
		Text:       query,
		Embedding:  embedding,
		Limit:      config.MaxDocuments,
		Mode:       config.SearchMode,
		SourceType: "",
	}

	// Filter by source types if specified
	if len(config.SourceTypes) > 0 {
		// Note: Current DocumentService only supports single SourceType
		// If multiple types requested, use the first one or consider hybrid search
		searchQuery.SourceType = config.SourceTypes[0]
	}

	// Search documents
	docs, err := s.documentService.Search(ctx, searchQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to search documents: %w", err)
	}

	s.logger.Debug().
		Int("retrieved", len(docs)).
		Int("requested", config.MaxDocuments).
		Msg("Retrieved context documents")

	return docs, nil
}

// buildContextText builds a formatted context string from documents
func (s *ChatService) buildContextText(docs []*models.Document) string {
	if len(docs) == 0 {
		return ""
	}

	var parts []string
	parts = append(parts, "RELEVANT CONTEXT:")
	parts = append(parts, "")

	for i, doc := range docs {
		parts = append(parts, fmt.Sprintf("Document %d:", i+1))
		parts = append(parts, fmt.Sprintf("Source: %s", doc.SourceType))
		if doc.Title != "" {
			parts = append(parts, fmt.Sprintf("Title: %s", doc.Title))
		}
		if doc.URL != "" {
			parts = append(parts, fmt.Sprintf("URL: %s", doc.URL))
		}
		parts = append(parts, fmt.Sprintf("Content: %s", truncateContent(doc.Content, 500)))
		parts = append(parts, "")
	}

	return strings.Join(parts, "\n")
}

// buildMessages constructs the message array for the LLM
func (s *ChatService) buildMessages(req *interfaces.ChatRequest, contextText string) []interfaces.Message {
	messages := []interfaces.Message{}

	// Add system prompt
	systemPrompt := req.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = getDefaultSystemPrompt()
	}

	// If we have context, augment the system prompt
	if contextText != "" {
		systemPrompt = fmt.Sprintf("%s\n\n%s", systemPrompt, contextText)
	}

	messages = append(messages, interfaces.Message{
		Role:    "system",
		Content: systemPrompt,
	})

	// Add conversation history
	if req.History != nil {
		messages = append(messages, req.History...)
	}

	// Add current user message
	messages = append(messages, interfaces.Message{
		Role:    "user",
		Content: req.Message,
	})

	return messages
}

// GetMode returns the current LLM mode
func (s *ChatService) GetMode() interfaces.LLMMode {
	return s.llmService.GetMode()
}

// HealthCheck verifies the chat service is operational
func (s *ChatService) HealthCheck(ctx context.Context) error {
	// Check LLM service
	if err := s.llmService.HealthCheck(ctx); err != nil {
		return fmt.Errorf("LLM service unhealthy: %w", err)
	}

	// Check embedding service
	if !s.embeddingService.IsAvailable(ctx) {
		return fmt.Errorf("embedding service unavailable")
	}

	return nil
}

// getDefaultSystemPrompt returns the default system prompt
func getDefaultSystemPrompt() string {
	return `You are a helpful AI assistant with access to a knowledge base of documents from Jira, Confluence, and GitHub.

When answering questions:
1. Use the provided context documents when relevant
2. Cite your sources by mentioning the document title or URL
3. If the context doesn't contain relevant information, say so clearly
4. Be concise and accurate in your responses
5. Format your responses in clear, readable Markdown

If you're unsure about something, acknowledge it rather than making assumptions.`
}

// truncateContent truncates content to the specified length
func truncateContent(content string, maxLength int) string {
	if len(content) <= maxLength {
		return content
	}
	return content[:maxLength] + "..."
}

// GetServiceStatus returns detailed service status information
// This includes LLM server states, model loading status, and health check time
// Note: Does NOT perform health checks - caller should do that separately to avoid redundant checks
func (s *ChatService) GetServiceStatus(ctx context.Context) map[string]interface{} {
	status := make(map[string]interface{})

	// Get LLM mode
	mode := string(s.llmService.GetMode())
	status["mode"] = mode

	// Get embedding model name
	status["embedding_model"] = s.embeddingService.ModelName()

	// Default values for mock mode
	status["embed_server"] = "N/A (mock mode)"
	status["chat_server"] = "N/A (mock mode)"
	status["model_loaded"] = true
	status["last_check_time"] = "N/A"

	// For offline mode, check if servers are running
	if mode == "offline" {
		// Check embed server (port 8086)
		embedStatus := checkServerHealth("http://127.0.0.1:8086/health")
		if embedStatus {
			status["embed_server"] = "active"
		} else {
			status["embed_server"] = "inactive"
		}

		// Check chat server (port 8087)
		chatStatus := checkServerHealth("http://127.0.0.1:8087/health")
		if chatStatus {
			status["chat_server"] = "active"
		} else {
			status["chat_server"] = "inactive"
		}

		status["model_loaded"] = embedStatus && chatStatus
	}

	return status
}

// checkServerHealth checks if a server port is listening
// Does NOT make HTTP requests to avoid triggering llama-server health checks
func checkServerHealth(url string) bool {
	// Extract host:port from URL
	// url format: "http://127.0.0.1:8086/health"
	var address string
	if strings.HasPrefix(url, "http://127.0.0.1:8086") {
		address = "127.0.0.1:8086"
	} else if strings.HasPrefix(url, "http://127.0.0.1:8087") {
		address = "127.0.0.1:8087"
	} else {
		return false
	}

	// Simple TCP connection check with 500ms timeout
	conn, err := net.DialTimeout("tcp", address, 500*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
