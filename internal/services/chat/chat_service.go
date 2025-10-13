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
	"github.com/ternarybob/quaero/internal/services/identifiers"
)

// ChatService implements RAG-enabled chat functionality
type ChatService struct {
	llmService        interfaces.LLMService
	documentService   interfaces.DocumentService
	embeddingService  interfaces.EmbeddingService
	identifierService *identifiers.Extractor
	documentStorage   interfaces.DocumentStorage
	logger            arbor.ILogger
	maxDocuments      int     // Maximum documents from config
	minSimilarity     float64 // Minimum similarity from config
}

// NewChatService creates a new chat service
func NewChatService(
	llmService interfaces.LLMService,
	documentService interfaces.DocumentService,
	embeddingService interfaces.EmbeddingService,
	identifierService *identifiers.Extractor,
	documentStorage interfaces.DocumentStorage,
	logger arbor.ILogger,
	maxDocuments int,
	minSimilarity float64,
) *ChatService {
	return &ChatService{
		llmService:        llmService,
		documentService:   documentService,
		embeddingService:  embeddingService,
		identifierService: identifierService,
		documentStorage:   documentStorage,
		logger:            logger,
		maxDocuments:      maxDocuments,
		minSimilarity:     minSimilarity,
	}
}

// Chat implements the ChatService interface
func (s *ChatService) Chat(ctx context.Context, req *interfaces.ChatRequest) (*interfaces.ChatResponse, error) {
	ragEnabled := req.RAGConfig != nil && req.RAGConfig.Enabled
	s.logger.Debug().
		Str("message", req.Message).
		Str("rag_enabled", fmt.Sprintf("%v", ragEnabled)).
		Msg("Processing chat request")

	// Classify the query to determine retrieval strategy
	classification := ClassifyQuery(req.Message)
	s.logger.Info().
		Str("query_type", string(classification.Type)).
		Str("needs_corpus", fmt.Sprintf("%v", classification.NeedsCorpus)).
		Int("max_documents", classification.MaxDocuments).
		Str("use_pointer_rag", fmt.Sprintf("%v", classification.UsePointerRAG)).
		Msg("Query classified")

	// Set default RAG config if not provided, using config values
	ragConfig := req.RAGConfig
	if ragConfig == nil {
		ragConfig = &interfaces.RAGConfig{
			Enabled:       true,
			MaxDocuments:  s.maxDocuments,
			MinSimilarity: float32(s.minSimilarity),
			SearchMode:    interfaces.SearchModeVector,
		}
	}

	// Override MaxDocuments based on query classification
	ragConfig.MaxDocuments = classification.MaxDocuments

	var contextDocs []*models.Document
	var messages []interfaces.Message

	// Retrieve relevant documents if RAG is enabled
	if ragConfig.Enabled {
		// For count/statistics queries, directly retrieve corpus summary
		if classification.NeedsCorpus {
			s.logger.Info().Msg("Retrieving corpus summary for count/statistics query")
			corpusDoc, err := s.documentService.GetBySource(ctx, "system", "corpus-summary-metadata")
			if err != nil {
				s.logger.Warn().Err(err).Msg("Failed to retrieve corpus summary, falling back to vector search")
			} else if corpusDoc != nil {
				contextDocs = []*models.Document{corpusDoc}
				messages = s.buildMessages(req, s.buildContextText(contextDocs))
				s.logger.Info().Msg("Using corpus summary document directly")
			}
		}

		// If we haven't retrieved context yet, use normal retrieval
		if contextDocs == nil && s.identifierService != nil && s.documentStorage != nil && classification.UsePointerRAG {
			// Pointer RAG: Augmented retrieval with cross-source linking
			s.logger.Info().Msg("Using Pointer RAG augmented retrieval")
			result, err := s.retrieveContextAugmented(ctx, req.Message, ragConfig)
			if err != nil {
				s.logger.Warn().Err(err).Msg("Failed to retrieve augmented context")
				// Fallback to basic retrieval
				docs, fallbackErr := s.retrieveContext(ctx, req.Message, ragConfig)
				if fallbackErr != nil {
					s.logger.Warn().Err(fallbackErr).Msg("Fallback retrieval also failed")
				} else {
					contextDocs = docs
					messages = s.buildMessages(req, s.buildContextText(docs))
				}
			} else {
				contextDocs = result.Documents

				// Log Pointer RAG retrieval metrics
				s.logger.Info().
					Int("documents_retrieved", len(result.Documents)).
					Int("identifiers_found", len(result.Identifiers)).
					Strs("identifiers", result.Identifiers).
					Msg("Pointer RAG retrieval complete")

				// Calculate document content sizes
				totalContentChars := 0
				for _, doc := range result.Documents {
					totalContentChars += len(doc.Content)
				}

				s.logger.Info().
					Int("total_content_chars", totalContentChars).
					Int("avg_content_per_doc", totalContentChars/max(len(result.Documents), 1)).
					Msg("Pointer RAG content metrics")

				messages = s.buildPointerRAGMessages(req, result.Documents, result.Identifiers)
			}
		} else if contextDocs == nil {
			// Fallback to basic vector search (if context not retrieved yet)
			s.logger.Debug().Msg("Using basic vector retrieval")
			docs, err := s.retrieveContext(ctx, req.Message, ragConfig)
			if err != nil {
				s.logger.Warn().Err(err).Msg("Failed to retrieve context documents")
			} else {
				contextDocs = docs
				messages = s.buildMessages(req, s.buildContextText(docs))
			}
		}
	}

	// If messages not built yet (RAG disabled or failed), build basic messages
	if messages == nil {
		messages = s.buildMessages(req, "")
	}

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

// augmentedRetrievalResult holds the results of Pointer RAG augmented retrieval
type augmentedRetrievalResult struct {
	Documents   []*models.Document
	Identifiers []string
}

// retrieveContextAugmented performs Pointer RAG augmented retrieval:
// 1. Initial vector search for relevant documents
// 2. Extract identifiers from retrieved documents
// 3. Search for cross-source linked documents by identifier
// 4. Deduplicate and rank results
// 5. Return top-k enriched context with identifiers
func (s *ChatService) retrieveContextAugmented(
	ctx context.Context,
	query string,
	config *interfaces.RAGConfig,
) (*augmentedRetrievalResult, error) {
	s.logger.Debug().Msg("Starting Pointer RAG augmented retrieval")

	// Phase 1: Initial vector search
	initialDocs, err := s.retrieveContext(ctx, query, config)
	if err != nil {
		return nil, fmt.Errorf("phase 1 (vector search) failed: %w", err)
	}

	s.logger.Debug().
		Int("initial_docs", len(initialDocs)).
		Msg("Phase 1: Initial vector search complete")

	if len(initialDocs) == 0 {
		s.logger.Debug().Msg("No initial documents found, returning empty results")
		return &augmentedRetrievalResult{
			Documents:   []*models.Document{},
			Identifiers: []string{},
		}, nil
	}

	// Phase 2: Extract identifiers from initial documents
	identifiers := s.identifierService.ExtractFromDocuments(initialDocs)
	s.logger.Debug().
		Int("identifiers_found", len(identifiers)).
		Strs("identifiers", identifiers).
		Msg("Phase 2: Identifier extraction complete")

	// Phase 3: Search for cross-source linked documents
	var linkedDocs []*models.Document
	for _, identifier := range identifiers {
		// Search for documents referencing this identifier
		// Exclude the same source type to encourage cross-source linking
		excludeSources := []string{} // Start with no exclusions

		docs, err := s.documentStorage.SearchByIdentifier(identifier, excludeSources, 10)
		if err != nil {
			s.logger.Warn().
				Err(err).
				Str("identifier", identifier).
				Msg("Failed to search by identifier, skipping")
			continue
		}

		s.logger.Debug().
			Str("identifier", identifier).
			Int("linked_docs", len(docs)).
			Msg("Found linked documents for identifier")

		linkedDocs = append(linkedDocs, docs...)
	}

	s.logger.Debug().
		Int("linked_docs", len(linkedDocs)).
		Msg("Phase 3: Cross-source linking complete")

	// Phase 4: Combine and deduplicate
	allDocs := append(initialDocs, linkedDocs...)
	uniqueDocs := deduplicateDocuments(allDocs)

	s.logger.Debug().
		Int("total_before_dedup", len(allDocs)).
		Int("total_after_dedup", len(uniqueDocs)).
		Msg("Phase 4: Deduplication complete")

	// Phase 5: Rank by cross-source connections
	rankedDocs := rankByCrossSourceConnections(uniqueDocs, identifiers)

	// Limit to max documents - reduce for performance
	// Pointer RAG needs more selective retrieval to avoid context overflow
	maxDocs := config.MaxDocuments // Was: config.MaxDocuments * 2 - reduced to avoid hanging
	if len(rankedDocs) > maxDocs {
		rankedDocs = rankedDocs[:maxDocs]
	}

	s.logger.Info().
		Int("final_doc_count", len(rankedDocs)).
		Int("identifiers_found", len(identifiers)).
		Msg("Pointer RAG augmented retrieval complete")

	return &augmentedRetrievalResult{
		Documents:   rankedDocs,
		Identifiers: identifiers,
	}, nil
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

// buildPointerRAGMessages constructs messages specifically for Pointer RAG,
// using the specialized prompt and formatted context that highlights cross-source connections
func (s *ChatService) buildPointerRAGMessages(
	req *interfaces.ChatRequest,
	docs []*models.Document,
	identifiers []string,
) []interfaces.Message {
	messages := []interfaces.Message{}

	// Use Pointer RAG system prompt
	systemPrompt := PointerRAGSystemPrompt
	basePromptSize := len(systemPrompt)

	// Build Pointer RAG-formatted context
	contextText := buildPointerRAGContextText(docs, identifiers)
	contextSize := len(contextText)

	// Augment system prompt with context
	if contextText != "" {
		systemPrompt = fmt.Sprintf("%s\n\n%s", systemPrompt, contextText)
	}

	totalSystemPromptSize := len(systemPrompt)

	// Log context building metrics
	s.logger.Info().
		Int("base_prompt_chars", basePromptSize).
		Int("context_text_chars", contextSize).
		Int("total_system_prompt_chars", totalSystemPromptSize).
		Int("formatting_overhead_chars", contextSize-(len(docs)*300)). // Rough estimate
		Msg("Pointer RAG context formatting complete")

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

	// Calculate total message size
	totalMessageChars := 0
	for _, msg := range messages {
		totalMessageChars += len(msg.Content)
	}

	// Estimate token count (rough: ~4 chars per token)
	estimatedTokens := totalMessageChars / 4

	s.logger.Info().
		Int("total_messages", len(messages)).
		Int("context_docs", len(docs)).
		Int("identifiers", len(identifiers)).
		Int("total_message_chars", totalMessageChars).
		Int("estimated_tokens", estimatedTokens).
		Msg("Built Pointer RAG messages - ready for LLM")

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
