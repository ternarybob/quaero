// -----------------------------------------------------------------------
// WorkerDebug - Debug metadata collection for workers
// Captures timing, API calls, and AI source information for debugging
// Only active when config.Workers.Debug = true
// -----------------------------------------------------------------------

package workers

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// WorkerDebugInfo collects debug metadata for a single worker instance
type WorkerDebugInfo struct {
	enabled      bool
	mu           sync.Mutex
	WorkerType   string               `json:"worker_type"`
	Ticker       string               `json:"ticker,omitempty"` // Optional: ticker being processed
	StartedAt    time.Time            `json:"started_at"`
	CompletedAt  time.Time            `json:"completed_at,omitempty"`
	APIEndpoints []APICallInfo        `json:"api_endpoints,omitempty"`
	AISource     *AISourceInfo        `json:"ai_source,omitempty"`
	Timing       TimingInfo           `json:"timing"`
	phases       map[string]time.Time // internal phase tracking
}

// APICallInfo records details of an API call
type APICallInfo struct {
	Endpoint   string `json:"endpoint"`
	Method     string `json:"method"`
	DurationMs int64  `json:"duration_ms"`
	StatusCode int    `json:"status_code,omitempty"`
}

// AISourceInfo records AI provider and usage information
type AISourceInfo struct {
	Provider     string `json:"provider"` // "gemini", "claude"
	Model        string `json:"model"`    // e.g., "gemini-2.5-flash-preview"
	InputTokens  int    `json:"input_tokens"`
	OutputTokens int    `json:"output_tokens"`
}

// TimingInfo records timing for different phases of worker execution
type TimingInfo struct {
	APIFetchMs           int64 `json:"api_fetch_ms,omitempty"`
	JSONGenerationMs     int64 `json:"json_generation_ms,omitempty"`
	MarkdownConversionMs int64 `json:"markdown_conversion_ms,omitempty"`
	AIGenerationMs       int64 `json:"ai_generation_ms,omitempty"`
	ComputationMs        int64 `json:"computation_ms,omitempty"`
	TotalMs              int64 `json:"total_ms"`
}

// NewWorkerDebug creates a new WorkerDebugInfo instance
// If debugEnabled is false, all recording operations are no-ops
func NewWorkerDebug(workerType string, debugEnabled bool) *WorkerDebugInfo {
	return &WorkerDebugInfo{
		enabled:    debugEnabled,
		WorkerType: workerType,
		StartedAt:  time.Now(),
		phases:     make(map[string]time.Time),
	}
}

// SetTicker sets the ticker being processed (optional)
func (w *WorkerDebugInfo) SetTicker(ticker string) {
	if !w.enabled {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.Ticker = ticker
}

// RecordAPICall records details of an API call
func (w *WorkerDebugInfo) RecordAPICall(endpoint, method string, duration time.Duration, statusCode int) {
	if !w.enabled {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()

	w.APIEndpoints = append(w.APIEndpoints, APICallInfo{
		Endpoint:   endpoint,
		Method:     method,
		DurationMs: duration.Milliseconds(),
		StatusCode: statusCode,
	})

	// Accumulate API fetch time
	w.Timing.APIFetchMs += duration.Milliseconds()
}

// RecordAISource records AI provider and token usage
func (w *WorkerDebugInfo) RecordAISource(provider, model string, inputTokens, outputTokens int) {
	if !w.enabled {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.AISource == nil {
		w.AISource = &AISourceInfo{
			Provider:     provider,
			Model:        model,
			InputTokens:  inputTokens,
			OutputTokens: outputTokens,
		}
	} else {
		// Accumulate tokens if multiple AI calls
		w.AISource.InputTokens += inputTokens
		w.AISource.OutputTokens += outputTokens
	}
}

// StartPhase marks the start of a named phase for timing
func (w *WorkerDebugInfo) StartPhase(phase string) {
	if !w.enabled {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.phases[phase] = time.Now()
}

// EndPhase marks the end of a named phase and records duration
func (w *WorkerDebugInfo) EndPhase(phase string) {
	if !w.enabled {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()

	startTime, ok := w.phases[phase]
	if !ok {
		return
	}

	durationMs := time.Since(startTime).Milliseconds()
	delete(w.phases, phase)

	// Map phase names to timing fields
	switch phase {
	case "api_fetch":
		w.Timing.APIFetchMs += durationMs
	case "json_generation":
		w.Timing.JSONGenerationMs += durationMs
	case "markdown_conversion":
		w.Timing.MarkdownConversionMs += durationMs
	case "ai_generation":
		w.Timing.AIGenerationMs += durationMs
	case "computation":
		w.Timing.ComputationMs += durationMs
	}
}

// Complete marks the worker execution as complete and calculates total time
func (w *WorkerDebugInfo) Complete() {
	if !w.enabled {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()

	w.CompletedAt = time.Now()
	w.Timing.TotalMs = w.CompletedAt.Sub(w.StartedAt).Milliseconds()
}

// ToMetadata converts the debug info to a map suitable for document metadata
// Returns nil if debug is disabled (zero overhead)
func (w *WorkerDebugInfo) ToMetadata() map[string]interface{} {
	if !w.enabled {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()

	result := map[string]interface{}{
		"worker_type": w.WorkerType,
		"started_at":  w.StartedAt.Format(time.RFC3339),
		"timing": map[string]interface{}{
			"total_ms": w.Timing.TotalMs,
		},
	}

	if w.Ticker != "" {
		result["ticker"] = w.Ticker
	}

	if !w.CompletedAt.IsZero() {
		result["completed_at"] = w.CompletedAt.Format(time.RFC3339)
	}

	// Add non-zero timing fields
	timing := result["timing"].(map[string]interface{})
	if w.Timing.APIFetchMs > 0 {
		timing["api_fetch_ms"] = w.Timing.APIFetchMs
	}
	if w.Timing.JSONGenerationMs > 0 {
		timing["json_generation_ms"] = w.Timing.JSONGenerationMs
	}
	if w.Timing.MarkdownConversionMs > 0 {
		timing["markdown_conversion_ms"] = w.Timing.MarkdownConversionMs
	}
	if w.Timing.AIGenerationMs > 0 {
		timing["ai_generation_ms"] = w.Timing.AIGenerationMs
	}
	if w.Timing.ComputationMs > 0 {
		timing["computation_ms"] = w.Timing.ComputationMs
	}

	// Add API endpoints if any
	if len(w.APIEndpoints) > 0 {
		endpoints := make([]map[string]interface{}, len(w.APIEndpoints))
		for i, ep := range w.APIEndpoints {
			endpoints[i] = map[string]interface{}{
				"endpoint":    ep.Endpoint,
				"method":      ep.Method,
				"duration_ms": ep.DurationMs,
			}
			if ep.StatusCode > 0 {
				endpoints[i]["status_code"] = ep.StatusCode
			}
		}
		result["api_endpoints"] = endpoints
	}

	// Add AI source if present
	if w.AISource != nil {
		result["ai_source"] = map[string]interface{}{
			"provider":      w.AISource.Provider,
			"model":         w.AISource.Model,
			"input_tokens":  w.AISource.InputTokens,
			"output_tokens": w.AISource.OutputTokens,
		}
	}

	return result
}

// IsEnabled returns whether debug is enabled
func (w *WorkerDebugInfo) IsEnabled() bool {
	return w.enabled
}

// ToMarkdown generates a markdown section with debug information.
// Returns empty string if debug is disabled.
func (w *WorkerDebugInfo) ToMarkdown() string {
	if !w.enabled {
		return ""
	}
	w.mu.Lock()
	defer w.mu.Unlock()

	var sb strings.Builder
	sb.WriteString("\n---\n")
	sb.WriteString("## Worker Debug Info\n\n")
	sb.WriteString(fmt.Sprintf("**Worker Type**: %s\n", w.WorkerType))
	if w.Ticker != "" {
		sb.WriteString(fmt.Sprintf("**Ticker**: %s\n", w.Ticker))
	}
	sb.WriteString(fmt.Sprintf("**Started**: %s\n", w.StartedAt.Format(time.RFC3339)))
	if !w.CompletedAt.IsZero() {
		sb.WriteString(fmt.Sprintf("**Completed**: %s\n", w.CompletedAt.Format(time.RFC3339)))
	}
	sb.WriteString("\n")

	// Timing breakdown
	sb.WriteString("### Timing\n\n")
	sb.WriteString("| Phase | Duration (ms) |\n")
	sb.WriteString("|-------|---------------|\n")
	if w.Timing.APIFetchMs > 0 {
		sb.WriteString(fmt.Sprintf("| API Fetch | %d |\n", w.Timing.APIFetchMs))
	}
	if w.Timing.JSONGenerationMs > 0 {
		sb.WriteString(fmt.Sprintf("| JSON Generation | %d |\n", w.Timing.JSONGenerationMs))
	}
	if w.Timing.MarkdownConversionMs > 0 {
		sb.WriteString(fmt.Sprintf("| Markdown Conversion | %d |\n", w.Timing.MarkdownConversionMs))
	}
	if w.Timing.AIGenerationMs > 0 {
		sb.WriteString(fmt.Sprintf("| AI Generation | %d |\n", w.Timing.AIGenerationMs))
	}
	if w.Timing.ComputationMs > 0 {
		sb.WriteString(fmt.Sprintf("| Computation | %d |\n", w.Timing.ComputationMs))
	}
	sb.WriteString(fmt.Sprintf("| **Total** | **%d** |\n\n", w.Timing.TotalMs))

	// API endpoints
	if len(w.APIEndpoints) > 0 {
		sb.WriteString("### API Calls\n\n")
		sb.WriteString("| Endpoint | Method | Duration (ms) | Status |\n")
		sb.WriteString("|----------|--------|---------------|--------|\n")
		for _, ep := range w.APIEndpoints {
			status := "-"
			if ep.StatusCode > 0 {
				status = fmt.Sprintf("%d", ep.StatusCode)
			}
			sb.WriteString(fmt.Sprintf("| %s | %s | %d | %s |\n", ep.Endpoint, ep.Method, ep.DurationMs, status))
		}
		sb.WriteString("\n")
	}

	// AI source
	if w.AISource != nil {
		sb.WriteString("### AI Source\n\n")
		sb.WriteString(fmt.Sprintf("- **Provider**: %s\n", w.AISource.Provider))
		sb.WriteString(fmt.Sprintf("- **Model**: %s\n", w.AISource.Model))
		sb.WriteString(fmt.Sprintf("- **Input Tokens**: %d\n", w.AISource.InputTokens))
		sb.WriteString(fmt.Sprintf("- **Output Tokens**: %d\n\n", w.AISource.OutputTokens))
	}

	return sb.String()
}
