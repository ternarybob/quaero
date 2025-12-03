package common

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
	"github.com/robfig/cron/v3"
	"github.com/ternarybob/arbor"

	"github.com/ternarybob/quaero/internal/interfaces"
)

// Config represents the application configuration
type Config struct {
	Environment string             `toml:"environment"` // "development" or "production" - controls test URL validation
	Server      ServerConfig       `toml:"server"`
	Queue       QueueConfig        `toml:"queue"`
	Storage     StorageConfig      `toml:"storage"`
	Processing  ProcessingConfig   `toml:"processing"`
	Logging     LoggingConfig      `toml:"logging"`
	Jobs        JobsConfig         `toml:"jobs"`
	Auth        AuthDirConfig      `toml:"auth"`
	Variables   KeysDirConfig      `toml:"variables"`  // Variables directory configuration (./keys/*.toml) for key/value pairs
	Connectors  ConnectorDirConfig `toml:"connectors"` // Connectors directory configuration (./connectors/*.toml)
	Crawler     CrawlerConfig      `toml:"crawler"`
	Search      SearchConfig       `toml:"search"`
	WebSocket   WebSocketConfig    `toml:"websocket"`
	PlacesAPI   PlacesAPIConfig    `toml:"places_api"`
	Gemini      GeminiConfig       `toml:"gemini"`
}

type ServerConfig struct {
	Port int    `toml:"port"`
	Host string `toml:"host"`
}

type QueueConfig struct {
	PollInterval      string `toml:"poll_interval"`      // e.g., "1s" - how often workers poll for messages
	Concurrency       int    `toml:"concurrency"`        // Number of concurrent workers
	VisibilityTimeout string `toml:"visibility_timeout"` // e.g., "5m" - message visibility timeout for redelivery
	MaxReceive        int    `toml:"max_receive"`        // Max times a message can be received before dead-letter
	QueueName         string `toml:"queue_name"`         // Queue name prefix in Badger
}

type StorageConfig struct {
	Badger     BadgerConfig     `toml:"badger"`
	Filesystem FilesystemConfig `toml:"filesystem"`
}

// BadgerConfig represents BadgerDB-specific configuration
type BadgerConfig struct {
	Path           string `toml:"path"`             // Database directory path
	ResetOnStartup bool   `toml:"reset_on_startup"` // Delete database on startup for clean test runs
}

type FilesystemConfig struct {
	Images      string `toml:"images"`
	Attachments string `toml:"attachments"`
}

type ProcessingConfig struct {
	Enabled  bool   `toml:"enabled"`
	Schedule string `toml:"schedule"` // Cron schedule format
	Limit    int    `toml:"limit"`    // Max documents to process per embedding run
}

type LoggingConfig struct {
	Level         string   `toml:"level"`           // "debug", "info", "warn", "error"
	Format        string   `toml:"format"`          // "json" or "text"
	Output        []string `toml:"output"`          // "stdout", "file"
	TimeFormat    string   `toml:"time_format"`     // Time format for logs (e.g. "15:04:05" or "2006-01-02 15:04:05")
	ClientDebug   bool     `toml:"client_debug"`    // Enable client-side debug logging
	MinEventLevel string   `toml:"min_event_level"` // Minimum log level to publish as events to UI ("debug", "info", "warn", "error")
}

// JobsConfig contains configuration for job definitions
type JobsConfig struct {
	DefinitionsDir string `toml:"definitions_dir"` // Directory containing job definition files (TOML/JSON)
}

// KeysDirConfig contains configuration for key/value file loading (generic secrets/configuration)
type KeysDirConfig struct {
	Dir string `toml:"dir"` // Directory containing variable files (TOML)
}

// AuthDirConfig contains configuration for authentication file loading
type AuthDirConfig struct {
	CredentialsDir string `toml:"credentials_dir"` // Directory containing auth credential files (TOML)
}

// ConnectorDirConfig contains configuration for connector file loading
type ConnectorDirConfig struct {
	Dir string `toml:"dir"` // Directory containing connector files (TOML)
}

// CrawlerConfig contains Firecrawl-inspired HTML scraping configuration
type CrawlerConfig struct {
	UserAgent                 string        `toml:"user_agent"`                   // Default user agent string
	UserAgentRotation         bool          `toml:"user_agent_rotation"`          // Enable random user agent rotation
	MaxConcurrency            int           `toml:"max_concurrency"`              // Maximum concurrent requests per domain
	RequestDelay              time.Duration `toml:"request_delay"`                // Minimum delay between requests to same domain
	RandomDelay               time.Duration `toml:"random_delay"`                 // Random delay jitter to add
	RequestTimeout            time.Duration `toml:"request_timeout"`              // HTTP request timeout
	MaxBodySize               int           `toml:"max_body_size"`                // Maximum response body size in bytes
	MaxDepth                  int           `toml:"max_depth"`                    // Maximum crawl depth
	FollowRobotsTxt           bool          `toml:"follow_robots_txt"`            // Respect robots.txt rules
	OutputFormat              string        `toml:"output_format"`                // Output format: "markdown", "html", or "both"
	OnlyMainContent           bool          `toml:"only_main_content"`            // Extract only main content, removing nav/footer/ads
	IncludeLinks              bool          `toml:"include_links"`                // Include discovered links in scrape results
	IncludeMetadata           bool          `toml:"include_metadata"`             // Extract and include page metadata
	UseHTMLSeeds              bool          `toml:"use_html_seeds"`               // Use HTML page URLs instead of REST API endpoints for seed URLs
	AllowedContentTypes       []string      `toml:"allowed_content_types"`        // Comment 5: Whitelist of allowed content types (e.g., "text/html", "application/json")
	EnableEmptyOutputFallback bool          `toml:"enable_empty_output_fallback"` // Apply HTML stripping fallback when HTML→MD conversion produces empty output (default: true)
	EnableJavaScript          bool          `toml:"enable_javascript"`            // Enable JavaScript rendering with chromedp (default: true for SPAs like Jira)
	JavaScriptWaitTime        time.Duration `toml:"javascript_wait_time"`         // Time to wait for JavaScript to render (default: 3s)
	QuickCrawlMaxDepth        int           `toml:"quick_crawl_max_depth"`        // Default max depth for quick crawl operations (default: 2)
	QuickCrawlMaxPages        int           `toml:"quick_crawl_max_pages"`        // Default max pages for quick crawl operations (default: 10)
}

// SearchConfig contains configuration for search behavior
type SearchConfig struct {
	Mode                    string `toml:"mode"`                      // Search service mode: "fts5", "advanced" (default), or "disabled"
	CaseSensitiveMultiplier int    `toml:"case_sensitive_multiplier"` // Multiplier for case-sensitive searches (default: 3)
	CaseSensitiveMaxCap     int    `toml:"case_sensitive_max_cap"`    // Maximum results cap for case-sensitive searches (default: 1000)
}

// WebSocketConfig contains configuration for WebSocket log streaming
type WebSocketConfig struct {
	MinLevel        string   `toml:"min_level"`        // Minimum log level to broadcast ("debug", "info", "warn", "error")
	ExcludePatterns []string `toml:"exclude_patterns"` // Log message patterns to exclude from broadcasting
	// Whitelist of event types to broadcast via WebSocket. Empty list allows all events.
	// Example: ["job_created", "job_completed", "crawl_progress"]
	AllowedEvents []string `toml:"allowed_events"`
	// Throttle intervals for high-frequency events. Map of event type to duration string.
	// Example: {"crawl_progress": "1s", "job_spawn": "500ms"}
	ThrottleIntervals map[string]string `toml:"throttle_intervals"`
}

// PlacesAPIConfig contains Google Places API configuration
type PlacesAPIConfig struct {
	APIKey              string        `toml:"api_key"`                // Google Places API key
	RateLimit           time.Duration `toml:"rate_limit"`             // Minimum time between API requests
	RequestTimeout      time.Duration `toml:"request_timeout"`        // HTTP request timeout
	MaxResultsPerSearch int           `toml:"max_results_per_search"` // Google Places API limit per request
}

// GeminiConfig contains unified Google Gemini API configuration for all AI services
type GeminiConfig struct {
	GoogleAPIKey string  `toml:"google_api_key"` // Google Gemini API key for all AI operations
	AgentModel   string  `toml:"agent_model"`    // Gemini model for agent operations (default: "gemini-2.0-flash")
	ChatModel    string  `toml:"chat_model"`     // Gemini model for chat operations (default: "gemini-2.0-flash")
	MaxTurns     int     `toml:"max_turns"`      // Maximum agent conversation turns (default: 10)
	Timeout      string  `toml:"timeout"`        // Operation timeout as duration string (default: "5m")
	RateLimit    string  `toml:"rate_limit"`     // Rate limit duration string (default: "4s" for 15 RPM)
	Temperature  float32 `toml:"temperature"`    // Chat completion temperature (default: 0.7)
}

// NewDefaultConfig creates a configuration with default values
// Technical parameters are hardcoded here for production stability.
// Only user-facing settings should be exposed in quaero.toml.
func NewDefaultConfig() *Config {
	return &Config{
		Environment: "development", // Default to development mode - allows test URLs
		Server: ServerConfig{
			Port: 8080,
			Host: "localhost",
		},
		Queue: QueueConfig{
			PollInterval:      "1s",
			Concurrency:       10, // Global job processor concurrency - how many jobs run in parallel
			VisibilityTimeout: "5m",
			MaxReceive:        3,
			QueueName:         "quaero_jobs",
		},
		Storage: StorageConfig{
			Badger: BadgerConfig{
				Path: "./data/quaero.badger",
			},
			Filesystem: FilesystemConfig{
				Images:      "./data/images",
				Attachments: "./data/attachments",
			},
		},
		Processing: ProcessingConfig{
			Enabled:  false,           // Disabled by default - user must explicitly opt-in
			Schedule: "0 0 */6 * * *", // Every 6 hours (cron format)
			Limit:    1000,            // Max documents per embedding run to prevent resource exhaustion
		},
		Logging: LoggingConfig{
			Level:         "info",                     // Info level for production (debug|info|warn|error)
			Format:        "text",                     // Human-readable text format (text|json)
			Output:        []string{"stdout", "file"}, // Log to both console and file
			MinEventLevel: "info",                     // Publish info and above as events to UI (debug logs only to DB)
		},
		Jobs: JobsConfig{
			DefinitionsDir: "./job-definitions", // Default directory for user-defined job files
		},
		Auth: AuthDirConfig{
			CredentialsDir: "./auth", // Default directory for auth files
		},
		Variables: KeysDirConfig{
			Dir: "./variables", // Default directory for variables files (./variables/*.toml)
		},
		Connectors: ConnectorDirConfig{
			Dir: "./connectors", // Default directory for connector files (relative to executable)
		},
		Crawler: CrawlerConfig{
			UserAgent:                 "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			UserAgentRotation:         true,
			MaxConcurrency:            3,
			RequestDelay:              1 * time.Second,
			RandomDelay:               500 * time.Millisecond,
			RequestTimeout:            30 * time.Second,
			MaxBodySize:               10 * 1024 * 1024, // 10MB
			MaxDepth:                  5,
			FollowRobotsTxt:           true,
			OutputFormat:              "markdown",
			OnlyMainContent:           true,
			IncludeLinks:              true,
			IncludeMetadata:           true,
			UseHTMLSeeds:              true,                                      // Default to HTML page URLs for seed generation
			AllowedContentTypes:       []string{"text/html", "application/json"}, // Comment 5: Allow HTML and JSON for Jira/Confluence APIs
			EnableEmptyOutputFallback: true,                                      // Apply HTML stripping fallback when conversion produces empty output
			EnableJavaScript:          true,                                      // Enable JavaScript rendering for SPAs like Jira
			JavaScriptWaitTime:        3 * time.Second,                           // Wait 3 seconds for JavaScript to render
			QuickCrawlMaxDepth:        2,                                         // Default max depth for quick crawl from extension
			QuickCrawlMaxPages:        10,                                        // Default max pages for quick crawl from extension
		},
		Search: SearchConfig{
			Mode:                    "advanced", // Advanced search with Google-style query parsing (fts5|advanced|disabled)
			CaseSensitiveMultiplier: 3,          // Fetch 3x the requested limit for case-sensitive searches
			CaseSensitiveMaxCap:     1000,       // Cap at 1000 results to prevent excessive memory usage
		},
		WebSocket: WebSocketConfig{
			MinLevel: "info", // Default: info level and above
			ExcludePatterns: []string{
				"WebSocket client connected",
				"WebSocket client disconnected",
				"HTTP request",
				"HTTP response",
				"Publishing Event",
				"DEBUG: Memory writer entry",
			},
			// Empty AllowedEvents allows all events (backward compatible)
			AllowedEvents: []string{},
			// Throttle high-frequency events to prevent WebSocket flooding during large crawls
			ThrottleIntervals: map[string]string{
				"crawl_progress": "1s",    // Max 1 crawl progress update per second per job
				"job_spawn":      "500ms", // Max 2 job spawn events per second
			},
		},
		PlacesAPI: PlacesAPIConfig{
			APIKey:              "",              // User must provide API key in config file
			RateLimit:           1 * time.Second, // 1 request per second (respects Google API quotas)
			RequestTimeout:      30 * time.Second,
			MaxResultsPerSearch: 20, // Google Places API default limit
		},
		Gemini: GeminiConfig{
			GoogleAPIKey: "",                 // User must provide API key (no fallback)
			AgentModel:   "gemini-2.0-flash", // Fast, cost-effective model for agents
			ChatModel:    "gemini-2.0-flash", // Fast, cost-effective model for chat
			MaxTurns:     10,                 // Reasonable limit for agent loops
			Timeout:      "5m",               // 5 minutes for operations
			RateLimit:    "4s",               // Default to 4s (15 RPM) for free tier
			Temperature:  0.7,                // Default temperature for chat completions
		},
	}
}

// LoadFromFile loads configuration with priority: default -> file -> env -> CLI
// Priority system: CLI flags > Environment variables > Config file > Defaults
// kvStorage can be nil for backward compatibility (replacement will be skipped)
func LoadFromFile(kvStorage interfaces.KeyValueStorage, path string) (*Config, error) {
	if path == "" {
		return LoadFromFiles(kvStorage)
	}
	return LoadFromFiles(kvStorage, path)
}

// LoadFromFiles loads configuration from multiple files with priority: default -> file1 -> file2 -> ... -> env -> CLI
// Later files override earlier files. Priority system: CLI flags > Environment variables > Last config file > ... > First config file > Defaults
// Example: LoadFromFiles(kvStorage, "base.toml", "override.toml") - override.toml settings take precedence over base.toml
// kvStorage can be nil for backward compatibility (replacement will be skipped)
func LoadFromFiles(kvStorage interfaces.KeyValueStorage, paths ...string) (*Config, error) {
	// Start with defaults
	config := NewDefaultConfig()

	// Load and merge each config file in order (later files override earlier files)
	for i, path := range paths {
		if path == "" {
			continue
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
		}

		// Unmarshal into config (merges with existing values, later values override)
		err = toml.Unmarshal(data, config)
		if err != nil {
			return nil, fmt.Errorf("failed to parse config file %s (file %d of %d): %w", path, i+1, len(paths), err)
		}
	}

	// Perform {key-name} replacement if KV storage is available
	if kvStorage != nil {
		ctx := context.Background()
		kvMap, err := kvStorage.GetAll(ctx)
		if err != nil {
			// Log warning and skip replacement (graceful degradation)
			logger := arbor.NewLogger()
			logger.Warn().Err(err).Msg("Failed to fetch KV map for config replacement, skipping replacement")
		} else {
			// Replace in config struct
			logger := arbor.NewLogger()
			if err := ReplaceInStruct(config, kvMap, logger); err != nil {
				logger.Warn().Err(err).Msg("Failed to replace key references in config")
			} else {
				logger.Info().Int("keys", len(kvMap)).Msg("Applied key/value replacements to config")
			}
		}
	}

	// Apply environment variables (overrides all file configs and replacements)
	applyEnvOverrides(config)

	return config, nil
}

// applyEnvOverrides applies environment variable overrides to config
func applyEnvOverrides(config *Config) {
	// Environment configuration (highest priority: QUAERO_ENV, fallback: GO_ENV)
	if env := os.Getenv("QUAERO_ENV"); env != "" {
		config.Environment = env
	} else if env := os.Getenv("GO_ENV"); env != "" {
		config.Environment = env
	}

	// Server configuration
	if port := os.Getenv("QUAERO_SERVER_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			config.Server.Port = p
		}
	}
	if host := os.Getenv("QUAERO_SERVER_HOST"); host != "" {
		config.Server.Host = host
	}

	// Queue configuration
	if pollInterval := os.Getenv("QUAERO_QUEUE_POLL_INTERVAL"); pollInterval != "" {
		config.Queue.PollInterval = pollInterval
	}
	if concurrency := os.Getenv("QUAERO_QUEUE_CONCURRENCY"); concurrency != "" {
		if c, err := strconv.Atoi(concurrency); err == nil {
			config.Queue.Concurrency = c
		}
	}
	if visibilityTimeout := os.Getenv("QUAERO_QUEUE_VISIBILITY_TIMEOUT"); visibilityTimeout != "" {
		config.Queue.VisibilityTimeout = visibilityTimeout
	}
	if maxReceive := os.Getenv("QUAERO_QUEUE_MAX_RECEIVE"); maxReceive != "" {
		if mr, err := strconv.Atoi(maxReceive); err == nil {
			config.Queue.MaxReceive = mr
		}
	}
	if queueName := os.Getenv("QUAERO_QUEUE_NAME"); queueName != "" {
		config.Queue.QueueName = queueName
	}

	// Storage configuration
	if badgerPath := os.Getenv("QUAERO_BADGER_PATH"); badgerPath != "" {
		config.Storage.Badger.Path = badgerPath
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
	if minEventLevel := os.Getenv("QUAERO_LOG_MIN_EVENT_LEVEL"); minEventLevel != "" {
		config.Logging.MinEventLevel = minEventLevel
	}

	// Crawler configuration
	if userAgent := os.Getenv("QUAERO_CRAWLER_USER_AGENT"); userAgent != "" {
		config.Crawler.UserAgent = userAgent
	}
	if userAgentRotation := os.Getenv("QUAERO_CRAWLER_USER_AGENT_ROTATION"); userAgentRotation != "" {
		if uar, err := strconv.ParseBool(userAgentRotation); err == nil {
			config.Crawler.UserAgentRotation = uar
		}
	}
	if maxConcurrency := os.Getenv("QUAERO_CRAWLER_MAX_CONCURRENCY"); maxConcurrency != "" {
		if mc, err := strconv.Atoi(maxConcurrency); err == nil {
			config.Crawler.MaxConcurrency = mc
		}
	}
	if requestDelay := os.Getenv("QUAERO_CRAWLER_REQUEST_DELAY"); requestDelay != "" {
		if rd, err := time.ParseDuration(requestDelay); err == nil {
			config.Crawler.RequestDelay = rd
		}
	}
	if randomDelay := os.Getenv("QUAERO_CRAWLER_RANDOM_DELAY"); randomDelay != "" {
		if rd, err := time.ParseDuration(randomDelay); err == nil {
			config.Crawler.RandomDelay = rd
		}
	}
	if requestTimeout := os.Getenv("QUAERO_CRAWLER_REQUEST_TIMEOUT"); requestTimeout != "" {
		if rt, err := time.ParseDuration(requestTimeout); err == nil {
			config.Crawler.RequestTimeout = rt
		}
	}
	if maxBodySize := os.Getenv("QUAERO_CRAWLER_MAX_BODY_SIZE"); maxBodySize != "" {
		if mbs, err := strconv.Atoi(maxBodySize); err == nil {
			config.Crawler.MaxBodySize = mbs
		}
	}
	if maxDepth := os.Getenv("QUAERO_CRAWLER_MAX_DEPTH"); maxDepth != "" {
		if md, err := strconv.Atoi(maxDepth); err == nil {
			config.Crawler.MaxDepth = md
		}
	}
	if followRobotsTxt := os.Getenv("QUAERO_CRAWLER_FOLLOW_ROBOTS_TXT"); followRobotsTxt != "" {
		if frt, err := strconv.ParseBool(followRobotsTxt); err == nil {
			config.Crawler.FollowRobotsTxt = frt
		}
	}
	if outputFormat := os.Getenv("QUAERO_CRAWLER_OUTPUT_FORMAT"); outputFormat != "" {
		config.Crawler.OutputFormat = outputFormat
	}
	if onlyMainContent := os.Getenv("QUAERO_CRAWLER_ONLY_MAIN_CONTENT"); onlyMainContent != "" {
		if omc, err := strconv.ParseBool(onlyMainContent); err == nil {
			config.Crawler.OnlyMainContent = omc
		}
	}
	if includeLinks := os.Getenv("QUAERO_CRAWLER_INCLUDE_LINKS"); includeLinks != "" {
		if il, err := strconv.ParseBool(includeLinks); err == nil {
			config.Crawler.IncludeLinks = il
		}
	}
	if includeMetadata := os.Getenv("QUAERO_CRAWLER_INCLUDE_METADATA"); includeMetadata != "" {
		if im, err := strconv.ParseBool(includeMetadata); err == nil {
			config.Crawler.IncludeMetadata = im
		}
	}
	if useHTMLSeeds := os.Getenv("QUAERO_CRAWLER_USE_HTML_SEEDS"); useHTMLSeeds != "" {
		if uhs, err := strconv.ParseBool(useHTMLSeeds); err == nil {
			config.Crawler.UseHTMLSeeds = uhs
		}
	}
	if enableEmptyOutputFallback := os.Getenv("QUAERO_CRAWLER_ENABLE_EMPTY_OUTPUT_FALLBACK"); enableEmptyOutputFallback != "" {
		if eof, err := strconv.ParseBool(enableEmptyOutputFallback); err == nil {
			config.Crawler.EnableEmptyOutputFallback = eof
		}
	}

	// Search configuration
	if searchMode := os.Getenv("QUAERO_SEARCH_MODE"); searchMode != "" {
		config.Search.Mode = searchMode
	}
	if caseSensitiveMultiplier := os.Getenv("QUAERO_SEARCH_CASE_SENSITIVE_MULTIPLIER"); caseSensitiveMultiplier != "" {
		if csm, err := strconv.Atoi(caseSensitiveMultiplier); err == nil {
			config.Search.CaseSensitiveMultiplier = csm
		}
	}
	if caseSensitiveMaxCap := os.Getenv("QUAERO_SEARCH_CASE_SENSITIVE_MAX_CAP"); caseSensitiveMaxCap != "" {
		if csmc, err := strconv.Atoi(caseSensitiveMaxCap); err == nil {
			config.Search.CaseSensitiveMaxCap = csmc
		}
	}

	// WebSocket configuration
	if minLevel := os.Getenv("QUAERO_WEBSOCKET_MIN_LEVEL"); minLevel != "" {
		config.WebSocket.MinLevel = minLevel
	}
	if excludePatterns := os.Getenv("QUAERO_WEBSOCKET_EXCLUDE_PATTERNS"); excludePatterns != "" {
		// Split comma-separated patterns
		patterns := []string{}
		for _, p := range splitString(excludePatterns, ",") {
			trimmed := trimSpace(p)
			if trimmed != "" {
				patterns = append(patterns, trimmed)
			}
		}
		if len(patterns) > 0 {
			config.WebSocket.ExcludePatterns = patterns
		}
	}
	if allowedEvents := os.Getenv("QUAERO_WEBSOCKET_ALLOWED_EVENTS"); allowedEvents != "" {
		// Split comma-separated event types
		events := []string{}
		for _, e := range splitString(allowedEvents, ",") {
			trimmed := trimSpace(e)
			if trimmed != "" {
				events = append(events, trimmed)
			}
		}
		if len(events) > 0 {
			config.WebSocket.AllowedEvents = events
		}
	}
	if crawlProgressThrottle := os.Getenv("QUAERO_WEBSOCKET_THROTTLE_CRAWL_PROGRESS"); crawlProgressThrottle != "" {
		// Parse duration string (e.g., "2s", "1500ms")
		if _, err := time.ParseDuration(crawlProgressThrottle); err == nil {
			if config.WebSocket.ThrottleIntervals == nil {
				config.WebSocket.ThrottleIntervals = make(map[string]string)
			}
			config.WebSocket.ThrottleIntervals["crawl_progress"] = crawlProgressThrottle
		}
	}
	if jobSpawnThrottle := os.Getenv("QUAERO_WEBSOCKET_THROTTLE_JOB_SPAWN"); jobSpawnThrottle != "" {
		// Parse duration string (e.g., "2s", "1500ms")
		if _, err := time.ParseDuration(jobSpawnThrottle); err == nil {
			if config.WebSocket.ThrottleIntervals == nil {
				config.WebSocket.ThrottleIntervals = make(map[string]string)
			}
			config.WebSocket.ThrottleIntervals["job_spawn"] = jobSpawnThrottle
		}
	}

	// Places API configuration
	if apiKey := os.Getenv("QUAERO_PLACES_API_KEY"); apiKey != "" {
		config.PlacesAPI.APIKey = apiKey
	}

	// Gemini configuration
	if apiKey := os.Getenv("QUAERO_GEMINI_GOOGLE_API_KEY"); apiKey != "" {
		config.Gemini.GoogleAPIKey = apiKey
	}
	if agentModel := os.Getenv("QUAERO_GEMINI_AGENT_MODEL"); agentModel != "" {
		config.Gemini.AgentModel = agentModel
	}
	if chatModel := os.Getenv("QUAERO_GEMINI_CHAT_MODEL"); chatModel != "" {
		config.Gemini.ChatModel = chatModel
	}
	if maxTurns := os.Getenv("QUAERO_GEMINI_MAX_TURNS"); maxTurns != "" {
		if mt, err := strconv.Atoi(maxTurns); err == nil {
			config.Gemini.MaxTurns = mt
		}
	}
	if timeout := os.Getenv("QUAERO_GEMINI_TIMEOUT"); timeout != "" {
		config.Gemini.Timeout = timeout
	}
	if rateLimit := os.Getenv("QUAERO_GEMINI_RATE_LIMIT"); rateLimit != "" {
		config.Gemini.RateLimit = rateLimit
	}
	if temperature := os.Getenv("QUAERO_GEMINI_TEMPERATURE"); temperature != "" {
		if t, err := strconv.ParseFloat(temperature, 32); err == nil {
			config.Gemini.Temperature = float32(t)
		}
	}

	// Auth configuration
	if authDir := os.Getenv("QUAERO_AUTH_CREDENTIALS_DIR"); authDir != "" {
		config.Auth.CredentialsDir = authDir
	}

	// Variables configuration
	if variablesDir := os.Getenv("QUAERO_VARIABLES_DIR"); variablesDir != "" {
		config.Variables.Dir = variablesDir
	}

	// Connectors configuration
	if connectorsDir := os.Getenv("QUAERO_CONNECTORS_DIR"); connectorsDir != "" {
		config.Connectors.Dir = connectorsDir
	}
}

// ApplyFlagOverrides applies command-line flag overrides to config
func ApplyFlagOverrides(config *Config, port int, host string) {
	// Command-line flags have highest priority
	if port > 0 {
		config.Server.Port = port
	}
	if host != "" {
		config.Server.Host = host
	}
}

// ResolveAPIKey resolves an API key by name with environment variable priority
// Resolution order: environment variables → KV store → config fallback → error
// This ensures QUAERO_* environment variables always take precedence
func ResolveAPIKey(ctx context.Context, kvStorage interfaces.KeyValueStorage, name string, configFallback string) (string, error) {
	// Map of KV store key names to environment variable names
	// Environment variables have highest priority
	keyToEnvMapping := map[string]string{
		"google_api_key": "QUAERO_GEMINI_GOOGLE_API_KEY",
		// Add more mappings as needed
		// "places_api_key": "QUAERO_PLACES_API_KEY", // If we move this to KV store
	}

	// Check environment variable first (highest priority)
	if envVarName, hasMappedEnv := keyToEnvMapping[name]; hasMappedEnv {
		if envValue := os.Getenv(envVarName); envValue != "" {
			// Environment variable takes priority over everything
			return envValue, nil
		}
	}

	// Try to resolve from KV store (medium priority - file-based variables)
	if kvStorage != nil {
		apiKey, err := kvStorage.Get(ctx, name)
		if err == nil && apiKey != "" {
			return apiKey, nil
		}
	}

	// Fallback to config value (lowest priority)
	if configFallback != "" {
		return configFallback, nil
	}

	return "", fmt.Errorf("API key '%s' not found in environment, KV store, or config", name)
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

// ValidateJobSchedule validates a cron schedule expression and ensures minimum 5-minute interval
func ValidateJobSchedule(schedule string) error {
	// Parse the cron expression
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	_, err := parser.Parse(schedule)
	if err != nil {
		return fmt.Errorf("invalid cron expression: %w", err)
	}

	// Check for minimum 5-minute interval
	// Validate minute field (first field in standard cron)
	parts := strings.Fields(schedule)
	if len(parts) < 5 {
		return fmt.Errorf("invalid cron format: expected 5 fields")
	}

	minuteField := parts[0]

	// Check for patterns that violate 5-minute minimum
	if minuteField == "*" {
		return fmt.Errorf("schedule must have minimum 5-minute interval (every minute is not allowed)")
	}

	// Check for */n patterns where n < 5
	if strings.HasPrefix(minuteField, "*/") {
		intervalStr := strings.TrimPrefix(minuteField, "*/")
		interval, err := strconv.Atoi(intervalStr)
		if err == nil && interval < 5 {
			return fmt.Errorf("schedule interval must be at least 5 minutes, got %d", interval)
		}
	}

	return nil
}

// IsProduction returns true if the environment is set to production
func (c *Config) IsProduction() bool {
	env := strings.ToLower(strings.TrimSpace(c.Environment))
	return env == "production" || env == "prod"
}

// AllowTestURLs returns true if test URLs (localhost, 127.0.0.1, etc.) are allowed
// Test URLs are only allowed in development mode
func (c *Config) AllowTestURLs() bool {
	return !c.IsProduction()
}

// DeepCloneConfig creates a deep copy of the Config struct
// This is used by ConfigService to prevent mutations of the original config
func DeepCloneConfig(c *Config) *Config {
	if c == nil {
		return nil
	}

	// Clone the config struct (shallow copy first)
	clone := *c

	// Deep clone slice fields to prevent shared memory
	if len(c.Logging.Output) > 0 {
		clone.Logging.Output = make([]string, len(c.Logging.Output))
		copy(clone.Logging.Output, c.Logging.Output)
	}

	if len(c.Crawler.AllowedContentTypes) > 0 {
		clone.Crawler.AllowedContentTypes = make([]string, len(c.Crawler.AllowedContentTypes))
		copy(clone.Crawler.AllowedContentTypes, c.Crawler.AllowedContentTypes)
	}

	if len(c.WebSocket.ExcludePatterns) > 0 {
		clone.WebSocket.ExcludePatterns = make([]string, len(c.WebSocket.ExcludePatterns))
		copy(clone.WebSocket.ExcludePatterns, c.WebSocket.ExcludePatterns)
	}

	if len(c.WebSocket.AllowedEvents) > 0 {
		clone.WebSocket.AllowedEvents = make([]string, len(c.WebSocket.AllowedEvents))
		copy(clone.WebSocket.AllowedEvents, c.WebSocket.AllowedEvents)
	}

	if len(c.WebSocket.ThrottleIntervals) > 0 {
		clone.WebSocket.ThrottleIntervals = make(map[string]string, len(c.WebSocket.ThrottleIntervals))
		for k, v := range c.WebSocket.ThrottleIntervals {
			clone.WebSocket.ThrottleIntervals[k] = v
		}
	}

	return &clone
}
