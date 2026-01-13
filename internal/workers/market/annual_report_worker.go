// -----------------------------------------------------------------------
// AnnualReportWorker - Extracts structured financial data from annual report PDFs
// Uses PDF extraction + LLM to parse key metrics like ROIC, WACC, debt facilities
// Complements fundamentals_worker by adding data not available from EODHD API
//
// WORKFLOW:
// 1. Query announcement_download documents by ticker
// 2. Retrieve stored PDFs via storage keys
// 3. Extract text with PDF extractor
// 4. Parse financial metrics with LLM using structured JSON schema
// 5. Create summary document with extracted metrics
// -----------------------------------------------------------------------

package market

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
	"github.com/ternarybob/quaero/internal/services/llm"
	"github.com/ternarybob/quaero/internal/workers/workerutil"
)

// AnnualReportMetrics represents extracted data from an annual report
type AnnualReportMetrics struct {
	Schema     string `json:"$schema"`
	Ticker     string `json:"ticker"`
	FiscalYear string `json:"fiscal_year"`
	ReportDate string `json:"report_date"`

	// Capital Efficiency
	ROICReported  *float64 `json:"roic_reported,omitempty"`
	WACCDisclosed *float64 `json:"wacc_disclosed,omitempty"`
	ROICSource    string   `json:"roic_source,omitempty"`

	// Debt Details
	DebtFacilities []DebtFacility `json:"debt_facilities,omitempty"`

	// Capital Raises
	CapitalRaises []CapitalRaiseDetail `json:"capital_raises,omitempty"`

	// Management Guidance
	RevenueGuidance   *GuidanceRange `json:"revenue_guidance,omitempty"`
	EBITDAGuidance    *GuidanceRange `json:"ebitda_guidance,omitempty"`
	ManagementOutlook string         `json:"management_outlook,omitempty"`

	// Segment Data
	Segments []SegmentData `json:"segments,omitempty"`

	// Extraction Metadata
	ExtractionConfidence float64 `json:"extraction_confidence"`
	PagesProcessed       int     `json:"pages_processed"`
	SourceDocumentID     string  `json:"source_document_id"`
	ExtractedAt          string  `json:"extracted_at"`
}

// DebtFacility represents a debt facility from the annual report
type DebtFacility struct {
	Name         string  `json:"name"`
	Type         string  `json:"type"`
	Limit        float64 `json:"limit"`
	Drawn        float64 `json:"drawn"`
	Maturity     string  `json:"maturity"`
	InterestRate string  `json:"interest_rate"`
}

// CapitalRaiseDetail represents a capital raise from the annual report
type CapitalRaiseDetail struct {
	Date    string  `json:"date"`
	Type    string  `json:"type"`
	Amount  float64 `json:"amount"`
	Purpose string  `json:"purpose"`
	Shares  int64   `json:"shares_issued,omitempty"`
}

// GuidanceRange represents management guidance with range
type GuidanceRange struct {
	Low      float64 `json:"low,omitempty"`
	High     float64 `json:"high,omitempty"`
	Midpoint float64 `json:"midpoint,omitempty"`
	Notes    string  `json:"notes,omitempty"`
}

// SegmentData represents segment-level financial data
type SegmentData struct {
	Name     string  `json:"name"`
	Revenue  float64 `json:"revenue,omitempty"`
	EBITDA   float64 `json:"ebitda,omitempty"`
	Margin   float64 `json:"margin,omitempty"`
	Growth   float64 `json:"growth,omitempty"`
	Comments string  `json:"comments,omitempty"`
}

// AnnualReportWorker extracts structured financial data from downloaded annual report PDFs.
type AnnualReportWorker struct {
	documentStorage interfaces.DocumentStorage
	kvStorage       interfaces.KeyValueStorage
	pdfExtractor    interfaces.PDFExtractor
	providerFactory *llm.ProviderFactory
	logger          arbor.ILogger
	jobMgr          *queue.Manager
	debugEnabled    bool
}

// Compile-time assertions
var _ interfaces.DefinitionWorker = (*AnnualReportWorker)(nil)
var _ interfaces.DocumentProvider = (*AnnualReportWorker)(nil)

// NewAnnualReportWorker creates a new annual report extraction worker.
func NewAnnualReportWorker(
	documentStorage interfaces.DocumentStorage,
	searchService interfaces.SearchService, // Keep for API compatibility, not used yet
	kvStorage interfaces.KeyValueStorage,
	pdfExtractor interfaces.PDFExtractor,
	providerFactory *llm.ProviderFactory,
	logger arbor.ILogger,
	jobMgr *queue.Manager,
	debugEnabled bool,
) *AnnualReportWorker {
	return &AnnualReportWorker{
		documentStorage: documentStorage,
		kvStorage:       kvStorage,
		pdfExtractor:    pdfExtractor,
		providerFactory: providerFactory,
		logger:          logger,
		jobMgr:          jobMgr,
		debugEnabled:    debugEnabled,
	}
}

// GetType returns the worker type identifier
func (w *AnnualReportWorker) GetType() models.WorkerType {
	return models.WorkerTypeMarketAnnualReport
}

// ValidateConfig validates step configuration
func (w *AnnualReportWorker) ValidateConfig(step models.JobStep) error {
	return nil
}

// ReturnsChildJobs returns false since this executes synchronously
func (w *AnnualReportWorker) ReturnsChildJobs() bool {
	return false
}

// Init initializes the worker and returns work items
func (w *AnnualReportWorker) Init(ctx context.Context, step models.JobStep, jobDef models.JobDefinition) (*interfaces.WorkerInitResult, error) {
	stepConfig := step.Config
	if stepConfig == nil {
		stepConfig = make(map[string]interface{})
	}

	// Extract ticker from config or job variables
	tickers := collectTickersWithJobDef(stepConfig, jobDef)
	if len(tickers) == 0 {
		return nil, fmt.Errorf("ticker is required in step config or job variables")
	}

	// Maximum reports to process per ticker
	maxReports := 1
	if m, ok := stepConfig["max_reports"].(float64); ok && m > 0 {
		maxReports = int(m)
	}

	workItems := make([]interfaces.WorkItem, len(tickers))
	for i, ticker := range tickers {
		workItems[i] = interfaces.WorkItem{
			ID:   ticker.String(),
			Name: fmt.Sprintf("Extract annual report for %s", ticker.String()),
		}
	}

	w.logger.Info().
		Str("phase", "init").
		Str("step_name", step.Name).
		Int("ticker_count", len(tickers)).
		Int("max_reports", maxReports).
		Msg("Annual report worker initialized")

	return &interfaces.WorkerInitResult{
		WorkItems: workItems,
		Metadata: map[string]interface{}{
			"tickers":     tickers,
			"max_reports": maxReports,
			"step_config": stepConfig,
		},
	}, nil
}

// CreateJobs creates queue jobs for the step
func (w *AnnualReportWorker) CreateJobs(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string, initResult *interfaces.WorkerInitResult) (string, error) {
	if initResult == nil {
		var err error
		initResult, err = w.Init(ctx, step, jobDef)
		if err != nil {
			return "", fmt.Errorf("failed to initialize annual_report worker: %w", err)
		}
	}

	// Get tickers from metadata
	tickers, _ := initResult.Metadata["tickers"].([]common.Ticker)
	stepConfig, _ := initResult.Metadata["step_config"].(map[string]interface{})
	maxReports, _ := initResult.Metadata["max_reports"].(int)
	if maxReports == 0 {
		maxReports = 1
	}

	// Extract output_tags
	var outputTags []string
	if stepConfig != nil {
		if tags, ok := stepConfig["output_tags"].([]interface{}); ok {
			for _, tag := range tags {
				if tagStr, ok := tag.(string); ok && tagStr != "" {
					outputTags = append(outputTags, tagStr)
				}
			}
		} else if tags, ok := stepConfig["output_tags"].([]string); ok {
			outputTags = tags
		}
	}

	// Get manager_id for document isolation
	managerID := workerutil.GetManagerID(ctx, w.jobMgr, stepID)

	processedCount := 0
	errorCount := 0
	var allDocIDs []string
	var allTags []string
	var allSourceIDs []string
	var allErrors []string
	tagsSeen := make(map[string]bool)
	byTicker := make(map[string]*interfaces.TickerResult)

	for _, ticker := range tickers {
		docInfo, err := w.processTicker(ctx, ticker, maxReports, &jobDef, stepID, outputTags, managerID)
		if err != nil {
			errMsg := fmt.Sprintf("%s: %v", ticker.String(), err)
			w.logger.Error().Err(err).Str("ticker", ticker.String()).Msg("Failed to process ticker")
			allErrors = append(allErrors, errMsg)
			errorCount++
			byTicker[ticker.String()] = &interfaces.TickerResult{
				DocumentsCreated: 0,
			}
			continue
		}

		if docInfo == nil {
			// No document created (no PDFs found)
			byTicker[ticker.String()] = &interfaces.TickerResult{
				DocumentsCreated: 0,
			}
			continue
		}

		processedCount++
		allDocIDs = append(allDocIDs, docInfo.ID)
		allSourceIDs = append(allSourceIDs, ticker.String())

		for _, tag := range docInfo.Tags {
			if !tagsSeen[tag] {
				tagsSeen[tag] = true
				allTags = append(allTags, tag)
			}
		}

		byTicker[ticker.String()] = &interfaces.TickerResult{
			DocumentsCreated: 1,
			DocumentIDs:      []string{docInfo.ID},
		}
	}

	// Build result
	result := &interfaces.WorkerResult{
		DocumentsCreated: processedCount,
		DocumentIDs:      allDocIDs,
		Tags:             allTags,
		SourceType:       "market_annual_report",
		SourceIDs:        allSourceIDs,
		Errors:           allErrors,
		ByTicker:         byTicker,
	}

	// Store result in KV for the manager to retrieve
	resultJSON, _ := json.Marshal(result)
	if err := w.kvStorage.Set(ctx, fmt.Sprintf("step_result:%s", stepID), string(resultJSON), "Annual report worker result"); err != nil {
		w.logger.Warn().Err(err).Str("step_id", stepID).Msg("Failed to store step result in KV")
	}

	w.logger.Info().
		Str("step_id", stepID).
		Int("processed", processedCount).
		Int("errors", errorCount).
		Int("total_docs", len(allDocIDs)).
		Msg("Annual report worker completed")

	return stepID, nil
}

// processTicker processes a single ticker and returns the document info
func (w *AnnualReportWorker) processTicker(ctx context.Context, ticker common.Ticker, maxReports int, jobDef *models.JobDefinition, stepID string, outputTags []string, managerID string) (*models.Document, error) {
	// Step 1: Find announcement_download document for this ticker
	downloadDoc, err := w.findAnnouncementDownloadDocument(ctx, ticker)
	if err != nil {
		w.logger.Debug().Err(err).Str("ticker", ticker.String()).Msg("No announcement_download document found")
		return nil, nil // Not an error - just no PDFs to process
	}

	// Step 2: Extract storage keys from the download document
	storageKeys := w.extractStorageKeys(downloadDoc, maxReports)
	if len(storageKeys) == 0 {
		w.logger.Info().Str("ticker", ticker.String()).Msg("No downloaded PDFs found in announcement_download document")
		return nil, nil
	}

	w.logger.Info().
		Str("ticker", ticker.String()).
		Int("pdf_count", len(storageKeys)).
		Msg("Found downloaded PDFs to process")

	// Step 3: Extract text from PDFs
	var combinedText strings.Builder
	pagesProcessed := 0
	for i, storageKey := range storageKeys {
		text, err := w.extractPDFText(ctx, storageKey)
		if err != nil {
			w.logger.Warn().Err(err).Str("storage_key", storageKey).Msg("Failed to extract PDF text")
			continue
		}

		if text != "" {
			if i > 0 {
				combinedText.WriteString("\n\n--- NEW DOCUMENT ---\n\n")
			}
			combinedText.WriteString(text)
			pagesProcessed++ // Approximate - count documents as "pages"
		}
	}

	extractedText := combinedText.String()
	if extractedText == "" {
		w.logger.Info().Str("ticker", ticker.String()).Msg("No text extracted from PDFs")
		return nil, nil
	}

	// Step 4: Extract metrics with LLM
	metrics, err := w.extractMetricsWithLLM(ctx, ticker, extractedText, downloadDoc.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to extract metrics with LLM: %w", err)
	}

	metrics.PagesProcessed = pagesProcessed
	metrics.SourceDocumentID = downloadDoc.ID
	metrics.ExtractedAt = time.Now().Format(time.RFC3339)

	// Step 5: Build and save document
	doc := w.createDocument(ticker, metrics, jobDef, outputTags, managerID)

	if err := w.documentStorage.SaveDocument(doc); err != nil {
		return nil, fmt.Errorf("failed to store document: %w", err)
	}

	w.logger.Info().
		Str("ticker", ticker.String()).
		Str("doc_id", doc.ID).
		Float64("confidence", metrics.ExtractionConfidence).
		Msg("Created annual report metrics document")

	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info",
			fmt.Sprintf("Extracted annual report metrics for %s (confidence: %.0f%%)", ticker.String(), metrics.ExtractionConfidence*100))
	}

	return doc, nil
}

// findAnnouncementDownloadDocument finds the announcement_download document for a ticker
func (w *AnnualReportWorker) findAnnouncementDownloadDocument(ctx context.Context, ticker common.Ticker) (*models.Document, error) {
	sourceType := "announcement_download"
	sourceID := fmt.Sprintf("%s:%s:announcement_download", ticker.Exchange, ticker.Code)

	doc, err := w.documentStorage.GetDocumentBySource(sourceType, sourceID)
	if err != nil {
		return nil, fmt.Errorf("failed to find announcement_download document: %w", err)
	}

	if doc == nil {
		return nil, fmt.Errorf("no announcement_download document found for %s", ticker.String())
	}

	return doc, nil
}

// extractStorageKeys extracts storage keys for downloaded PDFs from announcement_download document
func (w *AnnualReportWorker) extractStorageKeys(doc *models.Document, maxReports int) []string {
	if doc.Metadata == nil {
		return nil
	}

	// Get announcements array from metadata
	annsData, ok := doc.Metadata["announcements"]
	if !ok {
		return nil
	}

	var storageKeys []string

	// Handle different storage formats
	switch v := annsData.(type) {
	case []interface{}:
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				if downloaded, ok := m["downloaded"].(bool); ok && downloaded {
					if storageKey, ok := m["storage_key"].(string); ok && storageKey != "" {
						storageKeys = append(storageKeys, storageKey)
						if len(storageKeys) >= maxReports {
							break
						}
					}
				}
			}
		}
	case []map[string]interface{}:
		for _, m := range v {
			if downloaded, ok := m["downloaded"].(bool); ok && downloaded {
				if storageKey, ok := m["storage_key"].(string); ok && storageKey != "" {
					storageKeys = append(storageKeys, storageKey)
					if len(storageKeys) >= maxReports {
						break
					}
				}
			}
		}
	}

	return storageKeys
}

// extractPDFText extracts text from a PDF stored at the given storage key
func (w *AnnualReportWorker) extractPDFText(ctx context.Context, storageKey string) (string, error) {
	if w.pdfExtractor == nil {
		return "", fmt.Errorf("PDF extractor not available")
	}

	text, err := w.pdfExtractor.ExtractText(ctx, storageKey)
	if err != nil {
		return "", fmt.Errorf("failed to extract text from PDF: %w", err)
	}

	// Truncate if too long for LLM context (100K chars max)
	const maxChars = 100000
	if len(text) > maxChars {
		w.logger.Info().
			Str("storage_key", storageKey).
			Int("original_len", len(text)).
			Int("truncated_len", maxChars).
			Msg("Truncating PDF text for LLM context")
		text = text[:maxChars]
	}

	return text, nil
}

// extractMetricsWithLLM uses LLM to extract structured metrics from PDF text
func (w *AnnualReportWorker) extractMetricsWithLLM(ctx context.Context, ticker common.Ticker, pdfText string, sourceDocID string) (*AnnualReportMetrics, error) {
	if w.providerFactory == nil {
		return nil, fmt.Errorf("LLM provider factory not available")
	}

	// Build system prompt
	systemPrompt := w.buildExtractionPrompt(ticker)

	// Build output schema for structured JSON response
	outputSchema := w.buildOutputSchema()

	// Create request with schema
	request := &llm.ContentRequest{
		Messages: []interfaces.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: fmt.Sprintf("Extract financial metrics from the following annual report text:\n\n%s", pdfText)},
		},
		Temperature:  0.1, // Low temperature for factual extraction
		OutputSchema: outputSchema,
	}

	// Generate content
	response, err := w.providerFactory.GenerateContent(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("LLM generation failed: %w", err)
	}

	// Parse JSON response
	metrics := &AnnualReportMetrics{
		Schema: "urn:quaero:annual-report-metrics:v1",
		Ticker: ticker.String(),
	}

	if err := json.Unmarshal([]byte(response.Text), metrics); err != nil {
		w.logger.Warn().
			Err(err).
			Str("response", response.Text[:min(500, len(response.Text))]).
			Msg("Failed to parse LLM response as JSON, using defaults")
		metrics.ExtractionConfidence = 0.1
		metrics.ManagementOutlook = "Failed to parse LLM extraction response"
	}

	// Ensure schema is set correctly
	metrics.Schema = "urn:quaero:annual-report-metrics:v1"
	metrics.Ticker = ticker.String()

	return metrics, nil
}

// buildExtractionPrompt builds the system prompt for LLM extraction
func (w *AnnualReportWorker) buildExtractionPrompt(ticker common.Ticker) string {
	return fmt.Sprintf(`You are a financial analyst extracting structured data from an annual report.

## Company
Ticker: %s

## Extraction Tasks

1. **ROIC (Return on Invested Capital)**
   - Search for: "return on invested capital", "ROIC", "return on capital employed", "ROCE"
   - Expected location: Directors Report, Financial Highlights
   - Extract the percentage value if disclosed

2. **WACC (Weighted Average Cost of Capital)**
   - Search for: "weighted average cost of capital", "WACC", "discount rate", "hurdle rate"
   - Expected location: Notes to Financial Statements (Impairment testing)
   - Extract the percentage value if disclosed

3. **Debt Facilities**
   - Search for: "borrowings", "debt facilities", "bank facilities", "syndicated facility"
   - Expected location: Notes to Financial Statements
   - Extract: facility name, limit, amount drawn, maturity date, interest rate

4. **Capital Raises (past 12 months)**
   - Search for: "share placement", "share purchase plan", "SPP", "capital raising"
   - Extract: date, type, amount raised, purpose stated

5. **Management Guidance**
   - Search for: "outlook", "guidance", "FY expectations", "forecast"
   - Extract any revenue/EBITDA guidance ranges

6. **Business Segments**
   - Search for: "segment information", "operating segments"
   - Extract: segment name, revenue, EBITDA if available

## Response Instructions
- Set extraction_confidence between 0.0 and 1.0 based on how clearly the data was found
- Use null for fields where data was not found
- For fiscal_year, use format "FY24" or "2024"
- For report_date, use ISO format "YYYY-MM-DD"
- All monetary amounts should be in millions (e.g., 150.5 for $150.5M)
`, ticker.String())
}

// buildOutputSchema builds the JSON schema for LLM structured output
func (w *AnnualReportWorker) buildOutputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"ticker":      map[string]interface{}{"type": "string"},
			"fiscal_year": map[string]interface{}{"type": "string"},
			"report_date": map[string]interface{}{"type": "string"},
			"roic_reported": map[string]interface{}{
				"type":        "number",
				"description": "Return on Invested Capital as a percentage (e.g., 15.5 for 15.5%)",
			},
			"wacc_disclosed": map[string]interface{}{
				"type":        "number",
				"description": "Weighted Average Cost of Capital as a percentage",
			},
			"roic_source": map[string]interface{}{
				"type":        "string",
				"description": "Where in the document ROIC was found",
			},
			"debt_facilities": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"name":          map[string]interface{}{"type": "string"},
						"type":          map[string]interface{}{"type": "string"},
						"limit":         map[string]interface{}{"type": "number"},
						"drawn":         map[string]interface{}{"type": "number"},
						"maturity":      map[string]interface{}{"type": "string"},
						"interest_rate": map[string]interface{}{"type": "string"},
					},
				},
			},
			"capital_raises": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"date":          map[string]interface{}{"type": "string"},
						"type":          map[string]interface{}{"type": "string"},
						"amount":        map[string]interface{}{"type": "number"},
						"purpose":       map[string]interface{}{"type": "string"},
						"shares_issued": map[string]interface{}{"type": "integer"},
					},
				},
			},
			"revenue_guidance": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"low":      map[string]interface{}{"type": "number"},
					"high":     map[string]interface{}{"type": "number"},
					"midpoint": map[string]interface{}{"type": "number"},
					"notes":    map[string]interface{}{"type": "string"},
				},
			},
			"ebitda_guidance": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"low":      map[string]interface{}{"type": "number"},
					"high":     map[string]interface{}{"type": "number"},
					"midpoint": map[string]interface{}{"type": "number"},
					"notes":    map[string]interface{}{"type": "string"},
				},
			},
			"management_outlook": map[string]interface{}{
				"type":        "string",
				"description": "Summary of management's outlook and guidance",
			},
			"segments": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"name":     map[string]interface{}{"type": "string"},
						"revenue":  map[string]interface{}{"type": "number"},
						"ebitda":   map[string]interface{}{"type": "number"},
						"margin":   map[string]interface{}{"type": "number"},
						"growth":   map[string]interface{}{"type": "number"},
						"comments": map[string]interface{}{"type": "string"},
					},
				},
			},
			"extraction_confidence": map[string]interface{}{
				"type":        "number",
				"description": "Confidence score 0.0-1.0 based on data clarity",
				"minimum":     0,
				"maximum":     1,
			},
		},
		"required": []string{"ticker", "extraction_confidence"},
	}
}

// createDocument creates the output document with metrics
func (w *AnnualReportWorker) createDocument(ticker common.Ticker, metrics *AnnualReportMetrics, jobDef *models.JobDefinition, outputTags []string, managerID string) *models.Document {
	// Build markdown content
	content := w.buildMarkdownContent(metrics)

	// Build tags
	tags := []string{
		"annual-report",
		strings.ToLower(ticker.Code),
		strings.ToLower(ticker.String()),
		fmt.Sprintf("ticker:%s", ticker.String()),
		fmt.Sprintf("source_type:%s", models.WorkerTypeMarketAnnualReport.String()),
		fmt.Sprintf("date:%s", time.Now().Format("2006-01-02")),
	}

	if jobDef != nil && len(jobDef.Tags) > 0 {
		tags = append(tags, jobDef.Tags...)
	}
	tags = append(tags, outputTags...)

	// Build metadata
	metricsJSON, _ := json.Marshal(metrics)
	var metricsMap map[string]interface{}
	json.Unmarshal(metricsJSON, &metricsMap)

	// Add manager_id to metadata for isolation
	if managerID != "" {
		metricsMap["manager_id"] = managerID
	}

	// Build jobs array for job isolation
	var jobs []string
	if managerID != "" {
		jobs = []string{managerID}
	}

	now := time.Now()
	return &models.Document{
		ID:              "doc_" + uuid.New().String(),
		SourceType:      "market_annual_report",
		SourceID:        fmt.Sprintf("%s:%s:annual_report", ticker.Exchange, ticker.Code),
		Title:           fmt.Sprintf("%s Annual Report Metrics", ticker.String()),
		ContentMarkdown: content,
		Tags:            tags,
		Jobs:            jobs,
		Metadata:        metricsMap,
		CreatedAt:       now,
		UpdatedAt:       now,
		LastSynced:      &now,
	}
}

// buildMarkdownContent builds the markdown content for the document
func (w *AnnualReportWorker) buildMarkdownContent(metrics *AnnualReportMetrics) string {
	var content strings.Builder

	content.WriteString(fmt.Sprintf("# Annual Report Metrics: %s\n\n", metrics.Ticker))

	// Extraction metadata
	content.WriteString(fmt.Sprintf("**Extracted:** %s\n", metrics.ExtractedAt))
	content.WriteString(fmt.Sprintf("**Confidence:** %.0f%%\n", metrics.ExtractionConfidence*100))
	if metrics.FiscalYear != "" {
		content.WriteString(fmt.Sprintf("**Fiscal Year:** %s\n", metrics.FiscalYear))
	}
	content.WriteString("\n")

	// Capital Efficiency
	content.WriteString("## Capital Efficiency\n\n")
	if metrics.ROICReported != nil {
		content.WriteString(fmt.Sprintf("- **ROIC:** %.2f%%\n", *metrics.ROICReported))
		if metrics.ROICSource != "" {
			content.WriteString(fmt.Sprintf("  - Source: %s\n", metrics.ROICSource))
		}
	} else {
		content.WriteString("- **ROIC:** Not disclosed\n")
	}
	if metrics.WACCDisclosed != nil {
		content.WriteString(fmt.Sprintf("- **WACC:** %.2f%%\n", *metrics.WACCDisclosed))
	} else {
		content.WriteString("- **WACC:** Not disclosed\n")
	}
	content.WriteString("\n")

	// Debt Facilities
	if len(metrics.DebtFacilities) > 0 {
		content.WriteString("## Debt Facilities\n\n")
		content.WriteString("| Facility | Type | Limit ($M) | Drawn ($M) | Maturity | Rate |\n")
		content.WriteString("|----------|------|------------|------------|----------|------|\n")
		for _, df := range metrics.DebtFacilities {
			content.WriteString(fmt.Sprintf("| %s | %s | %.1f | %.1f | %s | %s |\n",
				df.Name, df.Type, df.Limit, df.Drawn, df.Maturity, df.InterestRate))
		}
		content.WriteString("\n")
	}

	// Capital Raises
	if len(metrics.CapitalRaises) > 0 {
		content.WriteString("## Capital Raises\n\n")
		content.WriteString("| Date | Type | Amount ($M) | Purpose |\n")
		content.WriteString("|------|------|-------------|----------|\n")
		for _, cr := range metrics.CapitalRaises {
			purpose := cr.Purpose
			if len(purpose) > 50 {
				purpose = purpose[:47] + "..."
			}
			content.WriteString(fmt.Sprintf("| %s | %s | %.1f | %s |\n",
				cr.Date, cr.Type, cr.Amount, purpose))
		}
		content.WriteString("\n")
	}

	// Management Guidance
	content.WriteString("## Management Guidance\n\n")
	if metrics.RevenueGuidance != nil {
		content.WriteString(fmt.Sprintf("- **Revenue Guidance:** $%.1fM - $%.1fM\n",
			metrics.RevenueGuidance.Low, metrics.RevenueGuidance.High))
		if metrics.RevenueGuidance.Notes != "" {
			content.WriteString(fmt.Sprintf("  - %s\n", metrics.RevenueGuidance.Notes))
		}
	}
	if metrics.EBITDAGuidance != nil {
		content.WriteString(fmt.Sprintf("- **EBITDA Guidance:** $%.1fM - $%.1fM\n",
			metrics.EBITDAGuidance.Low, metrics.EBITDAGuidance.High))
		if metrics.EBITDAGuidance.Notes != "" {
			content.WriteString(fmt.Sprintf("  - %s\n", metrics.EBITDAGuidance.Notes))
		}
	}
	if metrics.ManagementOutlook != "" {
		content.WriteString(fmt.Sprintf("\n**Outlook:** %s\n", metrics.ManagementOutlook))
	}
	content.WriteString("\n")

	// Segments
	if len(metrics.Segments) > 0 {
		content.WriteString("## Business Segments\n\n")
		content.WriteString("| Segment | Revenue ($M) | EBITDA ($M) | Margin | Growth |\n")
		content.WriteString("|---------|--------------|-------------|--------|--------|\n")
		for _, seg := range metrics.Segments {
			marginStr := "-"
			if seg.Margin > 0 {
				marginStr = fmt.Sprintf("%.1f%%", seg.Margin)
			}
			growthStr := "-"
			if seg.Growth != 0 {
				growthStr = fmt.Sprintf("%+.1f%%", seg.Growth)
			}
			content.WriteString(fmt.Sprintf("| %s | %.1f | %.1f | %s | %s |\n",
				seg.Name, seg.Revenue, seg.EBITDA, marginStr, growthStr))
		}
		content.WriteString("\n")
	}

	return content.String()
}

// -----------------------------------------------------------------------------
// DocumentProvider Implementation - Worker-to-Worker Communication Pattern
// -----------------------------------------------------------------------------

// GetDocument ensures an annual report metrics document exists for a single ticker.
// This implements the DocumentProvider interface for worker-to-worker communication.
func (w *AnnualReportWorker) GetDocument(ctx context.Context, identifier string, opts ...interfaces.DocumentOption) (*interfaces.DocumentResult, error) {
	options := interfaces.ApplyDocumentOptions(opts...)

	ticker := common.ParseTicker(identifier)
	if ticker.Code == "" {
		return &interfaces.DocumentResult{
			Identifier: identifier,
			Error:      fmt.Errorf("invalid ticker identifier: %s", identifier),
		}, fmt.Errorf("invalid ticker identifier: %s", identifier)
	}

	sourceType := "market_annual_report"
	sourceID := fmt.Sprintf("%s:%s:annual_report", ticker.Exchange, ticker.Code)

	result := &interfaces.DocumentResult{
		Identifier: identifier,
	}

	// Check for cached document if not forcing refresh
	if !options.ForceRefresh && options.CacheHours > 0 {
		existingDoc, err := w.documentStorage.GetDocumentBySource(sourceType, sourceID)
		if err == nil && existingDoc != nil && existingDoc.LastSynced != nil {
			if time.Since(*existingDoc.LastSynced) < time.Duration(options.CacheHours)*time.Hour {
				w.logger.Debug().
					Str("ticker", ticker.String()).
					Str("doc_id", existingDoc.ID).
					Str("last_synced", existingDoc.LastSynced.Format("2006-01-02 15:04")).
					Msg("GetDocument: using cached annual report document")

				result.DocumentID = existingDoc.ID
				result.Tags = existingDoc.Tags
				result.Fresh = true
				result.Created = false
				return result, nil
			}
		}
	}

	// Cache miss or stale - run extraction
	w.logger.Debug().
		Str("ticker", ticker.String()).
		Bool("force_refresh", options.ForceRefresh).
		Msg("GetDocument: running annual report extraction")

	// Process ticker (reuse the main logic)
	doc, err := w.processTicker(ctx, ticker, 1, nil, "", options.OutputTags, options.ManagerID)
	if err != nil {
		result.Error = fmt.Errorf("failed to process ticker: %w", err)
		return result, result.Error
	}

	if doc == nil {
		w.logger.Info().Str("ticker", ticker.String()).Msg("GetDocument: no annual report PDFs found")
		return result, nil // No error, but no document created
	}

	result.DocumentID = doc.ID
	result.Tags = doc.Tags
	result.Fresh = false
	result.Created = true

	w.logger.Debug().
		Str("ticker", ticker.String()).
		Str("doc_id", doc.ID).
		Msg("GetDocument: created fresh annual report document")

	return result, nil
}

// GetDocuments ensures annual report metrics documents exist for multiple tickers.
// This implements the DocumentProvider interface for worker-to-worker communication.
func (w *AnnualReportWorker) GetDocuments(ctx context.Context, identifiers []string, opts ...interfaces.DocumentOption) ([]*interfaces.DocumentResult, error) {
	results := make([]*interfaces.DocumentResult, len(identifiers))
	var lastErr error
	successCount := 0

	for i, id := range identifiers {
		result, err := w.GetDocument(ctx, id, opts...)
		results[i] = result
		if err != nil {
			lastErr = err
			w.logger.Warn().
				Err(err).
				Str("identifier", id).
				Msg("GetDocuments: failed to provision document")
		} else if result.DocumentID != "" {
			successCount++
		}
	}

	w.logger.Debug().
		Int("total", len(identifiers)).
		Int("success", successCount).
		Msg("GetDocuments: completed batch provisioning")

	// Return error only if ALL identifiers failed
	if successCount == 0 && lastErr != nil {
		return results, fmt.Errorf("all identifiers failed, last error: %w", lastErr)
	}

	return results, nil
}
