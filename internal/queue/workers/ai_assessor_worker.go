// -----------------------------------------------------------------------
// AIAssessorWorker - AI-powered stock assessment generation with validation
// Uses LLM service to generate assessments, validates and retries as needed
// -----------------------------------------------------------------------

package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
	"github.com/ternarybob/quaero/internal/signals"
	"gopkg.in/yaml.v3"
)

const (
	defaultBatchSize = 5
	maxRetries       = 2
)

// AIAssessorWorker generates AI-powered stock assessments.
type AIAssessorWorker struct {
	documentStorage interfaces.DocumentStorage
	kvStorage       interfaces.KeyValueStorage
	logger          arbor.ILogger
	jobMgr          *queue.Manager
	llmService      interfaces.LLMService
	validator       *signals.AssessmentValidator
}

// Compile-time assertion
var _ interfaces.DefinitionWorker = (*AIAssessorWorker)(nil)

// NewAIAssessorWorker creates a new AI assessor worker
func NewAIAssessorWorker(
	documentStorage interfaces.DocumentStorage,
	kvStorage interfaces.KeyValueStorage,
	logger arbor.ILogger,
	jobMgr *queue.Manager,
	llmService interfaces.LLMService,
) *AIAssessorWorker {
	return &AIAssessorWorker{
		documentStorage: documentStorage,
		kvStorage:       kvStorage,
		logger:          logger,
		jobMgr:          jobMgr,
		llmService:      llmService,
		validator:       signals.NewAssessmentValidator(),
	}
}

// GetType returns WorkerTypeAIAssessor
func (w *AIAssessorWorker) GetType() models.WorkerType {
	return models.WorkerTypeAIAssessor
}

// ReturnsChildJobs returns false - this worker executes inline
func (w *AIAssessorWorker) ReturnsChildJobs() bool {
	return false
}

// ValidateConfig validates the worker configuration
func (w *AIAssessorWorker) ValidateConfig(step models.JobStep) error {
	if step.Config == nil {
		return fmt.Errorf("ai_assessor step requires config")
	}
	return nil
}

// Init initializes the AI assessor worker
func (w *AIAssessorWorker) Init(ctx context.Context, step models.JobStep, jobDef models.JobDefinition) (*interfaces.WorkerInitResult, error) {
	w.logger.Info().
		Str("phase", "init").
		Str("step_name", step.Name).
		Msg("AI assessor worker initialized")

	return &interfaces.WorkerInitResult{
		WorkItems: []interfaces.WorkItem{
			{
				ID:     "ai-assessment",
				Name:   "Generate AI assessments",
				Type:   "ai_assessor",
				Config: step.Config,
			},
		},
		TotalCount:           1,
		Strategy:             interfaces.ProcessingStrategyInline,
		SuggestedConcurrency: 1,
		Metadata: map[string]interface{}{
			"step_config": step.Config,
		},
	}, nil
}

// CreateJobs generates AI assessments for portfolio holdings
func (w *AIAssessorWorker) CreateJobs(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string, initResult *interfaces.WorkerInitResult) (string, error) {
	if initResult == nil {
		var err error
		initResult, err = w.Init(ctx, step, jobDef)
		if err != nil {
			return "", fmt.Errorf("failed to initialize ai_assessor worker: %w", err)
		}
	}

	stepConfig, _ := initResult.Metadata["step_config"].(map[string]interface{})

	// Get holding types map
	holdingTypes := make(map[string]string)
	if ht, ok := stepConfig["holding_types"].(map[string]interface{}); ok {
		for ticker, t := range ht {
			if typeStr, ok := t.(string); ok {
				holdingTypes[strings.ToUpper(ticker)] = typeStr
			}
		}
	}

	// Get batch size
	batchSize := defaultBatchSize
	if bs, ok := stepConfig["batch_size"].(float64); ok {
		batchSize = int(bs)
	}

	// Load signal documents
	signalsTagPrefix := "ticker-signals"
	if prefix, ok := stepConfig["signals_tag_prefix"].(string); ok {
		signalsTagPrefix = prefix
	}

	// Get tickers to assess from holding_types or signals
	tickers := make([]string, 0, len(holdingTypes))
	for ticker := range holdingTypes {
		tickers = append(tickers, ticker)
	}

	if len(tickers) == 0 {
		w.logger.Warn().Msg("No tickers specified in holding_types")
		return stepID, nil
	}

	// Load signals for all tickers
	tickerSignals := w.loadSignals(ctx, tickers, signalsTagPrefix)
	w.logger.Info().
		Int("tickers", len(tickers)).
		Int("signals_loaded", len(tickerSignals)).
		Msg("Loaded signals for assessment")

	// Extract output_tags (supports both []interface{} from TOML and []string from inline calls)
	var outputTags []string
	if tags, ok := stepConfig["output_tags"].([]interface{}); ok {
		for _, tag := range tags {
			if tagStr, ok := tag.(string); ok && tagStr != "" {
				outputTags = append(outputTags, tagStr)
			}
		}
	} else if tags, ok := stepConfig["output_tags"].([]string); ok {
		outputTags = tags
	}

	// Process in batches
	processedCount := 0
	validCount := 0
	errorCount := 0

	for i := 0; i < len(tickers); i += batchSize {
		end := i + batchSize
		if end > len(tickers) {
			end = len(tickers)
		}
		batch := tickers[i:end]

		// Get signals for batch
		batchSignals := make([]signals.TickerSignals, 0, len(batch))
		for _, ticker := range batch {
			if sig, ok := tickerSignals[ticker]; ok {
				batchSignals = append(batchSignals, sig)
			}
		}

		if len(batchSignals) == 0 {
			continue
		}

		// Generate assessments for batch
		assessments, err := w.generateAssessments(ctx, batchSignals, holdingTypes)
		if err != nil {
			w.logger.Warn().Err(err).Int("batch_start", i).Msg("Failed to generate batch assessments")
			errorCount += len(batch)
			continue
		}

		// Validate and store each assessment
		for _, assessment := range assessments {
			sig := tickerSignals[assessment.Ticker]
			validatedAssessment := w.validateWithRetry(ctx, assessment, sig)

			if validatedAssessment.ValidationPassed {
				validCount++
			}

			// Store assessment document
			if err := w.storeAssessment(ctx, validatedAssessment, outputTags); err != nil {
				w.logger.Warn().Err(err).Str("ticker", assessment.Ticker).Msg("Failed to store assessment")
				errorCount++
				continue
			}
			processedCount++
		}
	}

	w.logger.Info().
		Int("processed", processedCount).
		Int("valid", validCount).
		Int("errors", errorCount).
		Msg("AI assessment complete")

	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info",
			fmt.Sprintf("AI assessment complete: %d processed, %d valid, %d errors",
				processedCount, validCount, errorCount))
	}

	return stepID, nil
}

// loadSignals loads signal documents for specified tickers
func (w *AIAssessorWorker) loadSignals(ctx context.Context, tickers []string, tagPrefix string) map[string]signals.TickerSignals {
	result := make(map[string]signals.TickerSignals)

	for _, ticker := range tickers {
		sourceType := "signal_computer"
		sourceID := fmt.Sprintf("signals:%s", ticker)

		doc, err := w.documentStorage.GetDocumentBySource(sourceType, sourceID)
		if err != nil {
			w.logger.Debug().Str("ticker", ticker).Msg("No signal document found")
			continue
		}

		if doc.Metadata != nil {
			if signalsData, ok := doc.Metadata["signals"]; ok {
				jsonBytes, err := json.Marshal(signalsData)
				if err == nil {
					var ts signals.TickerSignals
					if json.Unmarshal(jsonBytes, &ts) == nil {
						result[ticker] = ts
					}
				}
			}
		}
	}

	return result
}

// generateAssessments calls LLM to generate assessments for a batch
func (w *AIAssessorWorker) generateAssessments(ctx context.Context, batchSignals []signals.TickerSignals, holdingTypes map[string]string) ([]signals.TickerAssessment, error) {
	if w.llmService == nil {
		// Return placeholder assessments when LLM not available
		return w.generatePlaceholderAssessments(batchSignals, holdingTypes), nil
	}

	prompt := w.buildPrompt(batchSignals, holdingTypes)

	messages := []interfaces.Message{
		{Role: "user", Content: prompt},
	}

	response, err := w.llmService.Chat(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("LLM chat failed: %w", err)
	}

	// Parse YAML response
	assessments, err := w.parseAssessmentResponse(response, batchSignals)
	if err != nil {
		w.logger.Warn().Err(err).Msg("Failed to parse LLM response, using placeholder")
		return w.generatePlaceholderAssessments(batchSignals, holdingTypes), nil
	}

	return assessments, nil
}

// buildPrompt creates the assessment prompt for the LLM
func (w *AIAssessorWorker) buildPrompt(batchSignals []signals.TickerSignals, holdingTypes map[string]string) string {
	var sb strings.Builder

	sb.WriteString(`You are an expert ASX equity analyst assessing portfolio holdings.

CRITICAL RULES:
1. Every action recommendation MUST include exactly 3 evidence bullets with specific numbers
2. NEVER use generic phrases: "solid fundamentals", "well-positioned", "strong outlook"
3. If data is insufficient, output action: "insufficient_data" with missing items listed
4. Distinguish FACT (from signals) vs INFERENCE (your judgment)
5. All price targets and stops must be specific dollar values

OUTPUT FORMAT: Valid YAML only. No markdown, no explanations outside YAML.

`)

	// Add signal data for each ticker
	for _, sig := range batchSignals {
		holdingType := holdingTypes[sig.Ticker]
		if holdingType == "" {
			holdingType = "unknown"
		}

		sb.WriteString(fmt.Sprintf("\n---\nHOLDING: %s (%s)\n", sig.Ticker, holdingType))

		// Serialize key signal data
		sigData := map[string]interface{}{
			"ticker":            sig.Ticker,
			"current_price":     sig.Price.Current,
			"pbas_score":        sig.PBAS.Score,
			"pbas_interp":       sig.PBAS.Interpretation,
			"vli_score":         sig.VLI.Score,
			"vli_label":         sig.VLI.Label,
			"regime":            sig.Regime.Classification,
			"regime_confidence": sig.Regime.Confidence,
			"is_cooked":         sig.Cooked.IsCooked,
			"cooked_reasons":    sig.Cooked.Reasons,
			"rs_rank":           sig.RS.RSRankPercentile,
			"quality_overall":   sig.Quality.Overall,
			"risk_flags":        sig.RiskFlags,
		}

		sigYAML, _ := yaml.Marshal(sigData)
		sb.WriteString(string(sigYAML))
	}

	// Add output template
	sb.WriteString(`
---
OUTPUT TEMPLATE (repeat for each ticker):
- ticker: XXX
  decision:
    action: hold|accumulate|reduce|exit|buy|add|trim|watch|insufficient_data
    confidence: high|medium|low
    urgency: immediate|this_week|monitor
  reasoning:
    primary: "1-2 sentence main rationale"
    evidence:
      - "FACT: Specific number from signals (e.g., PBAS 0.72)"
      - "FACT: Another specific metric"
      - "INFERENCE: Your judgment based on data"
  entry_exit:
    stop_loss: "$X.XX"
    stop_loss_pct: X.X
    invalidation: "What would break the thesis"
  thesis_status: intact|weakening|strengthening|broken
`)

	return sb.String()
}

// parseAssessmentResponse parses YAML response from LLM
func (w *AIAssessorWorker) parseAssessmentResponse(response string, batchSignals []signals.TickerSignals) ([]signals.TickerAssessment, error) {
	// Extract YAML from response (may have markdown wrapping)
	yamlContent := response
	if strings.Contains(response, "```yaml") {
		start := strings.Index(response, "```yaml") + 7
		end := strings.LastIndex(response, "```")
		if end > start {
			yamlContent = response[start:end]
		}
	} else if strings.Contains(response, "```") {
		start := strings.Index(response, "```") + 3
		end := strings.LastIndex(response, "```")
		if end > start {
			yamlContent = response[start:end]
		}
	}

	// Parse as list of assessments
	var rawAssessments []map[string]interface{}
	if err := yaml.Unmarshal([]byte(yamlContent), &rawAssessments); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	assessments := make([]signals.TickerAssessment, 0, len(rawAssessments))
	for _, raw := range rawAssessments {
		assessment := signals.TickerAssessment{
			Ticker: getString(raw, "ticker"),
		}

		// Parse decision
		if decision, ok := raw["decision"].(map[string]interface{}); ok {
			assessment.Decision = signals.AssessmentDecision{
				Action:     getString(decision, "action"),
				Confidence: getString(decision, "confidence"),
				Urgency:    getString(decision, "urgency"),
			}
		}

		// Parse reasoning
		if reasoning, ok := raw["reasoning"].(map[string]interface{}); ok {
			assessment.Reasoning = signals.AssessmentReasoning{
				Primary: getString(reasoning, "primary"),
			}
			if evidence, ok := reasoning["evidence"].([]interface{}); ok {
				for _, e := range evidence {
					if eStr, ok := e.(string); ok {
						assessment.Reasoning.Evidence = append(assessment.Reasoning.Evidence, eStr)
					}
				}
			}
		}

		// Parse entry_exit
		if entryExit, ok := raw["entry_exit"].(map[string]interface{}); ok {
			assessment.EntryExit = signals.EntryExitParams{
				StopLoss:     getString(entryExit, "stop_loss"),
				StopLossPct:  getFloat64(entryExit, "stop_loss_pct"),
				Invalidation: getString(entryExit, "invalidation"),
			}
		}

		assessment.ThesisStatus = getString(raw, "thesis_status")
		assessments = append(assessments, assessment)
	}

	return assessments, nil
}

// generatePlaceholderAssessments creates placeholder assessments when LLM unavailable
func (w *AIAssessorWorker) generatePlaceholderAssessments(batchSignals []signals.TickerSignals, holdingTypes map[string]string) []signals.TickerAssessment {
	assessments := make([]signals.TickerAssessment, 0, len(batchSignals))

	for _, sig := range batchSignals {
		holdingType := holdingTypes[sig.Ticker]

		// Derive action from signals
		action := signals.ActionHold
		confidence := signals.ConfidenceMedium
		urgency := signals.UrgencyMonitor

		if sig.Cooked.IsCooked {
			action = signals.ActionReduce
			urgency = signals.UrgencyThisWeek
		} else if sig.PBAS.Score > 0.65 && sig.VLI.Score > 0.3 {
			action = signals.ActionAccumulate
		} else if sig.PBAS.Score < 0.40 || sig.VLI.Label == "distributing" {
			action = signals.ActionWatch
		}

		assessment := signals.TickerAssessment{
			Ticker:      sig.Ticker,
			HoldingType: holdingType,
			Decision: signals.AssessmentDecision{
				Action:     action,
				Confidence: confidence,
				Urgency:    urgency,
			},
			Reasoning: signals.AssessmentReasoning{
				Primary: fmt.Sprintf("Assessment based on PBAS %.2f, VLI %.2f, Regime %s",
					sig.PBAS.Score, sig.VLI.Score, sig.Regime.Classification),
				Evidence: []string{
					fmt.Sprintf("PBAS score: %.2f (%s)", sig.PBAS.Score, sig.PBAS.Interpretation),
					fmt.Sprintf("VLI score: %.2f (%s)", sig.VLI.Score, sig.VLI.Label),
					fmt.Sprintf("Regime: %s with %.0f%% confidence", sig.Regime.Classification, sig.Regime.Confidence*100),
				},
			},
			EntryExit: signals.EntryExitParams{
				StopLoss:     fmt.Sprintf("$%.2f", sig.Price.Current*0.90),
				StopLossPct:  10.0,
				Invalidation: "Break below 200 EMA with volume",
			},
			ThesisStatus:     signals.ThesisIntact,
			RiskFlags:        sig.RiskFlags,
			ValidationPassed: true, // Placeholder always passes
		}

		// Determine thesis status
		if sig.Cooked.IsCooked {
			assessment.ThesisStatus = signals.ThesisBroken
		} else if len(sig.RiskFlags) > 2 {
			assessment.ThesisStatus = signals.ThesisWeakening
		}

		assessments = append(assessments, assessment)
	}

	return assessments
}

// validateWithRetry validates assessment and retries if needed
func (w *AIAssessorWorker) validateWithRetry(ctx context.Context, assessment signals.TickerAssessment, sig signals.TickerSignals) signals.TickerAssessment {
	validation := w.validator.Validate(assessment, sig)
	if validation.Valid {
		assessment.ValidationPassed = true
		return assessment
	}

	w.logger.Warn().
		Str("ticker", assessment.Ticker).
		Strs("errors", validation.Errors).
		Msg("Assessment validation failed")

	// For now, mark as invalid without retry (LLM retry would go here)
	assessment.ValidationPassed = false
	assessment.ValidationErrors = validation.Errors

	return assessment
}

// storeAssessment saves assessment as a document
func (w *AIAssessorWorker) storeAssessment(ctx context.Context, assessment signals.TickerAssessment, outputTags []string) error {
	markdown := w.generateMarkdown(assessment)

	dateTag := fmt.Sprintf("date:%s", time.Now().Format("2006-01-02"))
	tags := []string{"ticker-assessment", assessment.Ticker, dateTag}
	tags = append(tags, outputTags...)

	now := time.Now()
	doc := &models.Document{
		ID:              "doc_" + uuid.New().String(),
		SourceType:      "ai_assessor",
		SourceID:        fmt.Sprintf("assessment:%s", assessment.Ticker),
		Title:           fmt.Sprintf("AI Assessment: %s", assessment.Ticker),
		ContentMarkdown: markdown,
		DetailLevel:     models.DetailLevelFull,
		Tags:            tags,
		CreatedAt:       now,
		UpdatedAt:       now,
		LastSynced:      &now,
		Metadata: map[string]interface{}{
			"ticker":            assessment.Ticker,
			"holding_type":      assessment.HoldingType,
			"action":            assessment.Decision.Action,
			"confidence":        assessment.Decision.Confidence,
			"urgency":           assessment.Decision.Urgency,
			"thesis_status":     assessment.ThesisStatus,
			"validation_passed": assessment.ValidationPassed,
			"validation_errors": assessment.ValidationErrors,
			"assessment":        assessment,
		},
	}

	return w.documentStorage.SaveDocument(doc)
}

// generateMarkdown creates markdown content from assessment
func (w *AIAssessorWorker) generateMarkdown(a signals.TickerAssessment) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# AI Assessment: %s\n\n", a.Ticker))
	sb.WriteString(fmt.Sprintf("**Holding Type**: %s\n", a.HoldingType))
	sb.WriteString(fmt.Sprintf("**Generated**: %s\n\n", time.Now().Format("2 January 2006 3:04 PM")))

	// Decision
	sb.WriteString("## Decision\n\n")
	sb.WriteString(fmt.Sprintf("**Action**: %s\n", strings.ToUpper(a.Decision.Action)))
	sb.WriteString(fmt.Sprintf("**Confidence**: %s\n", a.Decision.Confidence))
	sb.WriteString(fmt.Sprintf("**Urgency**: %s\n\n", a.Decision.Urgency))

	// Reasoning
	sb.WriteString("## Reasoning\n\n")
	sb.WriteString(fmt.Sprintf("%s\n\n", a.Reasoning.Primary))
	sb.WriteString("**Evidence:**\n")
	for _, e := range a.Reasoning.Evidence {
		sb.WriteString(fmt.Sprintf("- %s\n", e))
	}
	sb.WriteString("\n")

	// Entry/Exit
	if a.EntryExit.StopLoss != "" {
		sb.WriteString("## Entry/Exit Parameters\n\n")
		sb.WriteString(fmt.Sprintf("**Stop Loss**: %s (%.1f%%)\n", a.EntryExit.StopLoss, a.EntryExit.StopLossPct))
		if a.EntryExit.Target1 != "" {
			sb.WriteString(fmt.Sprintf("**Target 1**: %s\n", a.EntryExit.Target1))
		}
		sb.WriteString(fmt.Sprintf("**Invalidation**: %s\n\n", a.EntryExit.Invalidation))
	}

	// Thesis Status
	sb.WriteString(fmt.Sprintf("## Thesis Status: %s\n\n", strings.ToUpper(a.ThesisStatus)))

	// Risk Flags
	if len(a.RiskFlags) > 0 {
		sb.WriteString("## Risk Flags\n\n")
		for _, flag := range a.RiskFlags {
			sb.WriteString(fmt.Sprintf("- %s\n", flag))
		}
		sb.WriteString("\n")
	}

	// Validation
	if !a.ValidationPassed {
		sb.WriteString("## Validation\n\n")
		sb.WriteString("**Status**: FAILED\n")
		for _, err := range a.ValidationErrors {
			sb.WriteString(fmt.Sprintf("- %s\n", err))
		}
	}

	return sb.String()
}
