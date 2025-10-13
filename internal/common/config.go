package common

import (
	"fmt"
	"os"
	"strconv"

	"github.com/pelletier/go-toml/v2"
)

// Config represents the application configuration
type Config struct {
	Server     ServerConfig     `toml:"server"`
	Sources    SourcesConfig    `toml:"sources"`
	Storage    StorageConfig    `toml:"storage"`
	LLM        LLMConfig        `toml:"llm"`
	RAG        RAGConfig        `toml:"rag"`
	Embeddings EmbeddingsConfig `toml:"embeddings"`
	Processing ProcessingConfig `toml:"processing"`
	Logging    LoggingConfig    `toml:"logging"`
}

type ServerConfig struct {
	Port     int    `toml:"port"`
	Host     string `toml:"host"`
	LlamaDir string `toml:"llama_dir"` // Directory containing llama-cli binary
}

type SourcesConfig struct {
	Confluence ConfluenceConfig `toml:"confluence"`
	Jira       JiraConfig       `toml:"jira"`
	GitHub     GitHubConfig     `toml:"github"`
}

type ConfluenceConfig struct {
	Enabled bool     `toml:"enabled"`
	Spaces  []string `toml:"spaces"`
}

type JiraConfig struct {
	Enabled  bool     `toml:"enabled"`
	Projects []string `toml:"projects"`
}

type GitHubConfig struct {
	Enabled bool     `toml:"enabled"`
	Token   string   `toml:"token"`
	Repos   []string `toml:"repos"`
}

type StorageConfig struct {
	Type       string           `toml:"type"` // "sqlite", "ravendb", etc.
	SQLite     SQLiteConfig     `toml:"sqlite"`
	RavenDB    RavenDBConfig    `toml:"ravendb"`
	Filesystem FilesystemConfig `toml:"filesystem"`
}

type SQLiteConfig struct {
	Path               string `toml:"path"`                // Database file path
	EnableFTS5         bool   `toml:"enable_fts5"`         // Enable full-text search
	EnableVector       bool   `toml:"enable_vector"`       // Enable sqlite-vec extension
	EmbeddingDimension int    `toml:"embedding_dimension"` // Vector dimension for embeddings
	CacheSizeMB        int    `toml:"cache_size_mb"`       // Cache size in MB
	WALMode            bool   `toml:"wal_mode"`            // Enable WAL mode for better concurrency
	BusyTimeoutMS      int    `toml:"busy_timeout_ms"`     // Busy timeout in milliseconds
}

type RavenDBConfig struct {
	URLs     []string `toml:"urls"`
	Database string   `toml:"database"`
}

type FilesystemConfig struct {
	Images      string `toml:"images"`
	Attachments string `toml:"attachments"`
}

type LLMConfig struct {
	Mode    string           `toml:"mode"` // "offline" or "cloud"
	Offline OfflineLLMConfig `toml:"offline"`
	Cloud   CloudLLMConfig   `toml:"cloud"`
	Audit   AuditConfig      `toml:"audit"`
}

type OfflineLLMConfig struct {
	ModelDir    string `toml:"model_dir"`    // Directory containing model files
	EmbedModel  string `toml:"embed_model"`  // e.g., "nomic-embed-text-v1.5-q8.gguf"
	ChatModel   string `toml:"chat_model"`   // e.g., "qwen2.5-7b-instruct-q4.gguf"
	ContextSize int    `toml:"context_size"` // Context window size
	ThreadCount int    `toml:"thread_count"` // CPU threads for inference
	GPULayers   int    `toml:"gpu_layers"`   // Number of layers to offload to GPU
	MockMode    bool   `toml:"mock_mode"`    // Enable mock mode for testing (bypasses binary/model requirements)
}

type CloudLLMConfig struct {
	Provider    string  `toml:"provider"`    // "gemini", "openai", "anthropic"
	APIKey      string  `toml:"api_key"`     // API key (should use env var)
	EmbedModel  string  `toml:"embed_model"` // e.g., "text-embedding-004"
	ChatModel   string  `toml:"chat_model"`  // e.g., "gemini-1.5-flash"
	MaxTokens   int     `toml:"max_tokens"`  // Max response tokens
	Temperature float64 `toml:"temperature"` // 0.0-1.0
}

type AuditConfig struct {
	Enabled    bool `toml:"enabled"`     // Enable audit logging
	LogQueries bool `toml:"log_queries"` // Log query text (disable for PII)
}

type RAGConfig struct {
	MaxDocuments  int     `toml:"max_documents"`  // Maximum number of documents to retrieve
	MinSimilarity float64 `toml:"min_similarity"` // Minimum similarity score (0.0-1.0)
	SearchMode    string  `toml:"search_mode"`    // "vector", "keyword", or "hybrid"
}

type EmbeddingsConfig struct {
	Enabled   bool   `toml:"enabled"`
	OllamaURL string `toml:"ollama_url"`
	Model     string `toml:"model"`
	Dimension int    `toml:"dimension"`
	BatchSize int    `toml:"batch_size"`
}

type ProcessingConfig struct {
	Enabled  bool   `toml:"enabled"`
	Schedule string `toml:"schedule"` // Cron schedule format
	Limit    int    `toml:"limit"`    // Max documents to process per embedding run
}

type LoggingConfig struct {
	Level  string   `toml:"level"`
	Format string   `toml:"format"`
	Output []string `toml:"output"`
}

// NewDefaultConfig creates a configuration with default values
func NewDefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port:     8080,
			Host:     "localhost",
			LlamaDir: "./llama",
		},
		Storage: StorageConfig{
			Type: "sqlite",
			SQLite: SQLiteConfig{
				Path:               "./data/quaero.db",
				EnableFTS5:         true,
				EnableVector:       true,
				EmbeddingDimension: 768,
				CacheSizeMB:        64,
				WALMode:            true,
				BusyTimeoutMS:      5000,
			},
			Filesystem: FilesystemConfig{
				Images:      "./data/images",
				Attachments: "./data/attachments",
			},
		},
		LLM: LLMConfig{
			Mode: "offline", // Default to secure offline mode
			Offline: OfflineLLMConfig{
				ModelDir:    "./models",
				EmbedModel:  "nomic-embed-text-v1.5-q8.gguf",
				ChatModel:   "qwen2.5-7b-instruct-q4.gguf",
				ContextSize: 4096, // Increased to handle more RAG documents
				ThreadCount: 4,
				GPULayers:   0, // CPU-only by default
			},
			Cloud: CloudLLMConfig{
				Provider:    "gemini",
				EmbedModel:  "text-embedding-004",
				ChatModel:   "gemini-1.5-flash",
				MaxTokens:   2048,
				Temperature: 0.7,
			},
			Audit: AuditConfig{
				Enabled:    true,
				LogQueries: false, // Don't log query text by default (PII safety)
			},
		},
		RAG: RAGConfig{
			MaxDocuments:  20,       // Retrieve up to 20 documents (local LLM can handle more)
			MinSimilarity: 0.6,      // Lower threshold to include more relevant docs
			SearchMode:    "vector", // Default to vector search
		},
		Embeddings: EmbeddingsConfig{
			Enabled:   true,
			OllamaURL: "http://localhost:11434",
			Model:     "nomic-embed-text",
			Dimension: 768,
			BatchSize: 10,
		},
		Processing: ProcessingConfig{
			Enabled:  false,           // Disabled by default, user must opt-in
			Schedule: "0 0 */6 * * *", // Every 6 hours
			Limit:    1000,            // Max documents per embedding run
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "text",
			Output: []string{"stdout", "file"},
		},
	}
}

// LoadFromFile loads configuration with priority: default -> file -> env -> CLI
// Priority system: CLI flags > Environment variables > Config file > Defaults
func LoadFromFile(path string) (*Config, error) {
	// Start with defaults
	config := NewDefaultConfig()

	// Load from file if exists (overrides defaults)
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}

		err = toml.Unmarshal(data, config)
		if err != nil {
			return nil, fmt.Errorf("failed to parse config file: %w", err)
		}
	}

	// Apply environment variables (overrides file config)
	applyEnvOverrides(config)

	return config, nil
}

// applyEnvOverrides applies environment variable overrides to config
func applyEnvOverrides(config *Config) {
	// Server configuration
	if port := os.Getenv("QUAERO_SERVER_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			config.Server.Port = p
		}
	}
	if host := os.Getenv("QUAERO_SERVER_HOST"); host != "" {
		config.Server.Host = host
	}
	if llamaDir := os.Getenv("QUAERO_SERVER_LLAMA_DIR"); llamaDir != "" {
		config.Server.LlamaDir = llamaDir
	}

	// Storage configuration
	if storageType := os.Getenv("QUAERO_STORAGE_TYPE"); storageType != "" {
		config.Storage.Type = storageType
	}
	if sqlitePath := os.Getenv("QUAERO_SQLITE_PATH"); sqlitePath != "" {
		config.Storage.SQLite.Path = sqlitePath
	}

	// LLM configuration
	if llmMode := os.Getenv("QUAERO_LLM_MODE"); llmMode != "" {
		config.LLM.Mode = llmMode
	}
	if modelDir := os.Getenv("QUAERO_LLM_OFFLINE_MODEL_DIR"); modelDir != "" {
		config.LLM.Offline.ModelDir = modelDir
	}
	if embedModel := os.Getenv("QUAERO_LLM_OFFLINE_EMBED_MODEL"); embedModel != "" {
		config.LLM.Offline.EmbedModel = embedModel
	}
	if chatModel := os.Getenv("QUAERO_LLM_OFFLINE_CHAT_MODEL"); chatModel != "" {
		config.LLM.Offline.ChatModel = chatModel
	}
	if contextSize := os.Getenv("QUAERO_LLM_OFFLINE_CONTEXT_SIZE"); contextSize != "" {
		if cs, err := strconv.Atoi(contextSize); err == nil {
			config.LLM.Offline.ContextSize = cs
		}
	}
	if threadCount := os.Getenv("QUAERO_LLM_OFFLINE_THREAD_COUNT"); threadCount != "" {
		if tc, err := strconv.Atoi(threadCount); err == nil {
			config.LLM.Offline.ThreadCount = tc
		}
	}
	if gpuLayers := os.Getenv("QUAERO_LLM_OFFLINE_GPU_LAYERS"); gpuLayers != "" {
		if gl, err := strconv.Atoi(gpuLayers); err == nil {
			config.LLM.Offline.GPULayers = gl
		}
	}
	if provider := os.Getenv("QUAERO_LLM_CLOUD_PROVIDER"); provider != "" {
		config.LLM.Cloud.Provider = provider
	}
	if apiKey := os.Getenv("QUAERO_LLM_CLOUD_API_KEY"); apiKey != "" {
		config.LLM.Cloud.APIKey = apiKey
	}
	if embedModel := os.Getenv("QUAERO_LLM_CLOUD_EMBED_MODEL"); embedModel != "" {
		config.LLM.Cloud.EmbedModel = embedModel
	}
	if chatModel := os.Getenv("QUAERO_LLM_CLOUD_CHAT_MODEL"); chatModel != "" {
		config.LLM.Cloud.ChatModel = chatModel
	}
	if maxTokens := os.Getenv("QUAERO_LLM_CLOUD_MAX_TOKENS"); maxTokens != "" {
		if mt, err := strconv.Atoi(maxTokens); err == nil {
			config.LLM.Cloud.MaxTokens = mt
		}
	}
	if temperature := os.Getenv("QUAERO_LLM_CLOUD_TEMPERATURE"); temperature != "" {
		if temp, err := strconv.ParseFloat(temperature, 64); err == nil {
			config.LLM.Cloud.Temperature = temp
		}
	}
	if auditEnabled := os.Getenv("QUAERO_LLM_AUDIT_ENABLED"); auditEnabled != "" {
		if enabled, err := strconv.ParseBool(auditEnabled); err == nil {
			config.LLM.Audit.Enabled = enabled
		}
	}
	if logQueries := os.Getenv("QUAERO_LLM_AUDIT_LOG_QUERIES"); logQueries != "" {
		if lq, err := strconv.ParseBool(logQueries); err == nil {
			config.LLM.Audit.LogQueries = lq
		}
	}

	// Logging configuration
	if level := os.Getenv("QUAERO_LOG_LEVEL"); level != "" {
		config.Logging.Level = level
	}
	if format := os.Getenv("QUAERO_LOG_FORMAT"); format != "" {
		config.Logging.Format = format
	}
	if output := os.Getenv("QUAERO_LOG_OUTPUT"); output != "" {
		// Split comma-separated output types
		outputs := []string{}
		for _, o := range splitString(output, ",") {
			trimmed := trimSpace(o)
			if trimmed != "" {
				outputs = append(outputs, trimmed)
			}
		}
		if len(outputs) > 0 {
			config.Logging.Output = outputs
		}
	}
}

// ApplyCLIOverrides applies CLI flag overrides to config
func ApplyCLIOverrides(config *Config, port int, host string) {
	// CLI flags have highest priority
	if port > 0 {
		config.Server.Port = port
	}
	if host != "" {
		config.Server.Host = host
	}
}

// Helper functions for string manipulation
func splitString(s, sep string) []string {
	result := []string{}
	start := 0
	for i := 0; i < len(s); i++ {
		if i+len(sep) <= len(s) && s[i:i+len(sep)] == sep {
			result = append(result, s[start:i])
			start = i + len(sep)
			i = start - 1
		}
	}
	result = append(result, s[start:])
	return result
}

func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}
