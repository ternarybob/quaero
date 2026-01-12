// -----------------------------------------------------------------------
// AnnualReportWorker - Extracts structured financial data from annual report PDFs
// Uses PDF extraction + LLM to parse key metrics like ROIC, WACC, debt facilities
// Complements fundamentals_worker by adding data not available from EODHD API
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

// Compile-time assertion
var _ interfaces.DefinitionWorker = (*AnnualReportWorker)(nil)

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
		docInfo, err := w.processTicker(ctx, ticker, stepConfig, &jobDef, stepID, outputTags, managerID)
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
func (w *AnnualReportWorker) processTicker(ctx context.Context, ticker common.Ticker, stepConfig map[string]interface{}, jobDef *models.JobDefinition, stepID string, outputTags []string, managerID string) (*models.Document, error) {
	// For now, create a placeholder document indicating no PDFs were processed
	// Full PDF extraction will be implemented when announcement_download integration is complete
	metrics := &AnnualReportMetrics{
		Schema:               "urn:quaero:annual-report-metrics:v1",
		Ticker:               ticker.String(),
		ManagementOutlook:    "PDF extraction pending - requires annual report PDFs from announcement_download worker",
		ExtractionConfidence: 0,
		ExtractedAt:          time.Now().Format(time.RFC3339),
	}

	// Build markdown content
	var content strings.Builder
	content.WriteString(fmt.Sprintf("# Annual Report Extraction: %s\n\n", ticker.String()))
	content.WriteString(fmt.Sprintf("**Extraction Date:** %s\n", metrics.ExtractedAt))
	content.WriteString(fmt.Sprintf("**Status:** %s\n\n", metrics.ManagementOutlook))
	content.WriteString("*Note: Full PDF extraction will be enabled when annual report PDFs are available from the announcement_download worker.*\n")

	// Build tags
	tags := []string{"annual-report", strings.ToLower(ticker.Code), strings.ToLower(ticker.String())}
	tags = append(tags, fmt.Sprintf("date:%s", time.Now().Format("2006-01-02")))

	if jobDef != nil && len(jobDef.Tags) > 0 {
		tags = append(tags, jobDef.Tags...)
	}
	tags = append(tags, outputTags...)

	// Add cache tags from context
	cacheTags := queue.GetCacheTagsFromContext(ctx)
	if len(cacheTags) > 0 {
		tags = models.MergeTags(tags, cacheTags)
	}

	// Build metadata
	metricsJSON, _ := json.Marshal(metrics)
	var metricsMap map[string]interface{}
	json.Unmarshal(metricsJSON, &metricsMap)

	// Add manager_id to metadata for isolation
	if managerID != "" {
		metricsMap["manager_id"] = managerID
	}

	doc := &models.Document{
		ID:              "doc_" + uuid.New().String(),
		SourceType:      "market_annual_report",
		SourceID:        ticker.String(),
		Title:           fmt.Sprintf("%s Annual Report Metrics", ticker.String()),
		ContentMarkdown: content.String(),
		Tags:            tags,
		Metadata:        metricsMap,
	}

	// Store document
	if err := w.documentStorage.SaveDocument(doc); err != nil {
		return nil, fmt.Errorf("failed to store document: %w", err)
	}

	return doc, nil
}
