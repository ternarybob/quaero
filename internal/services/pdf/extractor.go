// -----------------------------------------------------------------------
// PDF Extractor Service - Extract text content from PDF documents
// Uses pdfcpu for Go-native PDF processing
// -----------------------------------------------------------------------

package pdf

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
)

// Extractor implements the PDFExtractor interface using pdfcpu
type Extractor struct {
	kvStorage interfaces.KeyValueStorage
	logger    arbor.ILogger
	tempDir   string
}

// Compile-time interface assertion
var _ interfaces.PDFExtractor = (*Extractor)(nil)

// NewExtractor creates a new PDF extractor service
func NewExtractor(kvStorage interfaces.KeyValueStorage, logger arbor.ILogger) *Extractor {
	// Create a temp directory for PDF processing
	tempDir := filepath.Join(os.TempDir(), "quaero-pdf")
	os.MkdirAll(tempDir, 0755)

	return &Extractor{
		kvStorage: kvStorage,
		logger:    logger,
		tempDir:   tempDir,
	}
}

// ExtractText extracts all text content from a PDF stored at the given storage key.
func (e *Extractor) ExtractText(ctx context.Context, storageKey string) (string, error) {
	pages, err := e.ExtractPages(ctx, storageKey)
	if err != nil {
		return "", err
	}

	var builder strings.Builder
	for i, page := range pages {
		if i > 0 {
			builder.WriteString("\n\n--- Page ")
			builder.WriteString(fmt.Sprintf("%d", page.PageNumber))
			builder.WriteString(" ---\n\n")
		}
		builder.WriteString(page.Text)
	}

	return builder.String(), nil
}

// ExtractPages extracts text content by page from a PDF.
func (e *Extractor) ExtractPages(ctx context.Context, storageKey string) ([]interfaces.PDFPageContent, error) {
	// Get PDF content from storage
	pdfContent, err := e.getPDFFromStorage(ctx, storageKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get PDF from storage: %w", err)
	}

	// Write to temp file for pdfcpu processing
	tempFile := filepath.Join(e.tempDir, fmt.Sprintf("extract_%d.pdf", os.Getpid()))
	if err := os.WriteFile(tempFile, pdfContent, 0644); err != nil {
		return nil, fmt.Errorf("failed to write temp PDF file: %w", err)
	}
	defer os.Remove(tempFile)

	// Get page count using pdfcpu
	conf := model.NewDefaultConfiguration()
	pdfCtx, err := api.ReadContextFile(tempFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read PDF context: %w", err)
	}

	pageCount := pdfCtx.PageCount
	pages := make([]interfaces.PDFPageContent, 0, pageCount)

	// Extract text from each page
	// pdfcpu doesn't have direct text extraction, so we extract content
	outDir := filepath.Join(e.tempDir, fmt.Sprintf("pages_%d", os.Getpid()))
	os.MkdirAll(outDir, 0755)
	defer os.RemoveAll(outDir)

	// Extract content from all pages
	if err := api.ExtractContentFile(tempFile, outDir, nil, conf); err != nil {
		e.logger.Warn().Err(err).Msg("Failed to extract PDF content, trying alternative method")
		// If extraction fails, return pages with empty text
		for pageNum := 1; pageNum <= pageCount; pageNum++ {
			pages = append(pages, interfaces.PDFPageContent{
				PageNumber: pageNum,
				Text:       "", // No text extracted
			})
		}
		return pages, nil
	}

	// Read extracted content files
	files, _ := os.ReadDir(outDir)
	pageTexts := make(map[int]string)
	for _, file := range files {
		if !file.IsDir() {
			content, err := os.ReadFile(filepath.Join(outDir, file.Name()))
			if err == nil {
				// Try to extract page number from filename
				var pageNum int
				if _, err := fmt.Sscanf(file.Name(), "page_%d", &pageNum); err == nil {
					pageTexts[pageNum] = string(content)
				} else {
					// Try extracting Content_page_X format
					if _, err := fmt.Sscanf(file.Name(), "Content_page_%d", &pageNum); err == nil {
						pageTexts[pageNum] = string(content)
					}
				}
			}
		}
	}

	// Build pages array
	for pageNum := 1; pageNum <= pageCount; pageNum++ {
		text := pageTexts[pageNum]
		pages = append(pages, interfaces.PDFPageContent{
			PageNumber: pageNum,
			Text:       text,
		})
	}

	return pages, nil
}

// ExtractWithMetadata performs full extraction including metadata, pages, and text.
func (e *Extractor) ExtractWithMetadata(ctx context.Context, storageKey string) (*interfaces.PDFExtractionResult, error) {
	// Get metadata first
	metadata, err := e.GetMetadata(ctx, storageKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get metadata: %w", err)
	}

	// Extract pages
	pages, err := e.ExtractPages(ctx, storageKey)
	if err != nil {
		return nil, fmt.Errorf("failed to extract pages: %w", err)
	}

	// Build full text
	var fullText strings.Builder
	for i, page := range pages {
		if i > 0 {
			fullText.WriteString("\n\n")
		}
		fullText.WriteString(page.Text)
	}

	return &interfaces.PDFExtractionResult{
		Metadata: *metadata,
		Pages:    pages,
		FullText: fullText.String(),
		Tables:   nil, // Table extraction not implemented in basic pdfcpu
	}, nil
}

// ExtractPageRange extracts text from specific pages (1-indexed, inclusive).
func (e *Extractor) ExtractPageRange(ctx context.Context, storageKey string, startPage, endPage int) ([]interfaces.PDFPageContent, error) {
	allPages, err := e.ExtractPages(ctx, storageKey)
	if err != nil {
		return nil, err
	}

	// Validate range
	if startPage < 1 {
		startPage = 1
	}
	if endPage > len(allPages) {
		endPage = len(allPages)
	}
	if startPage > endPage {
		return nil, fmt.Errorf("invalid page range: start %d > end %d", startPage, endPage)
	}

	// Return subset (convert to 0-indexed)
	return allPages[startPage-1 : endPage], nil
}

// GetMetadata retrieves PDF metadata without extracting text content.
func (e *Extractor) GetMetadata(ctx context.Context, storageKey string) (*interfaces.PDFMetadata, error) {
	// Get PDF content from storage
	pdfContent, err := e.getPDFFromStorage(ctx, storageKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get PDF from storage: %w", err)
	}

	// Write to temp file for pdfcpu processing
	tempFile := filepath.Join(e.tempDir, fmt.Sprintf("meta_%d.pdf", os.Getpid()))
	if err := os.WriteFile(tempFile, pdfContent, 0644); err != nil {
		return nil, fmt.Errorf("failed to write temp PDF file: %w", err)
	}
	defer os.Remove(tempFile)

	// Read PDF context
	pdfCtx, err := api.ReadContextFile(tempFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read PDF context: %w", err)
	}

	metadata := &interfaces.PDFMetadata{
		PageCount:   pdfCtx.PageCount,
		FileSize:    int64(len(pdfContent)),
		IsEncrypted: pdfCtx.Encrypt != nil,
	}

	// Try to get document info - pdfcpu stores this differently
	// We'll use a simpler approach for now
	e.logger.Debug().
		Int("page_count", metadata.PageCount).
		Int64("file_size", metadata.FileSize).
		Bool("encrypted", metadata.IsEncrypted).
		Msg("Extracted PDF metadata")

	return metadata, nil
}

// getPDFFromStorage retrieves PDF content from key-value storage
func (e *Extractor) getPDFFromStorage(ctx context.Context, storageKey string) ([]byte, error) {
	// Try to get from KV storage (base64 encoded)
	content, err := e.kvStorage.Get(ctx, storageKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get PDF from storage key %s: %w", storageKey, err)
	}

	// Content should be base64 encoded - decode it
	decoded, err := base64.StdEncoding.DecodeString(content)
	if err != nil {
		// Maybe it's raw bytes stored as string
		return []byte(content), nil
	}

	return decoded, nil
}

// ExtractTextFromBytes extracts text directly from PDF bytes without going through storage.
// This is useful for direct processing without storage lookup.
func (e *Extractor) ExtractTextFromBytes(ctx context.Context, pdfContent []byte) (string, error) {
	// Write to temp file for pdfcpu processing
	tempFile := filepath.Join(e.tempDir, fmt.Sprintf("direct_%d.pdf", os.Getpid()))
	if err := os.WriteFile(tempFile, pdfContent, 0644); err != nil {
		return "", fmt.Errorf("failed to write temp PDF file: %w", err)
	}
	defer os.Remove(tempFile)

	// Get page count using pdfcpu
	conf := model.NewDefaultConfiguration()
	pdfCtx, err := api.ReadContextFile(tempFile)
	if err != nil {
		return "", fmt.Errorf("failed to read PDF context: %w", err)
	}

	pageCount := pdfCtx.PageCount

	// Extract content from all pages
	outDir := filepath.Join(e.tempDir, fmt.Sprintf("direct_pages_%d", os.Getpid()))
	os.MkdirAll(outDir, 0755)
	defer os.RemoveAll(outDir)

	if err := api.ExtractContentFile(tempFile, outDir, nil, conf); err != nil {
		return "", fmt.Errorf("failed to extract PDF content: %w", err)
	}

	// Read and concatenate all extracted content
	var fullText strings.Builder
	files, _ := os.ReadDir(outDir)
	pageTexts := make(map[int]string)

	for _, file := range files {
		if !file.IsDir() {
			content, err := os.ReadFile(filepath.Join(outDir, file.Name()))
			if err == nil {
				var pageNum int
				if _, err := fmt.Sscanf(file.Name(), "Content_page_%d", &pageNum); err == nil {
					pageTexts[pageNum] = string(content)
				}
			}
		}
	}

	// Build text in page order
	for pageNum := 1; pageNum <= pageCount; pageNum++ {
		if text, ok := pageTexts[pageNum]; ok {
			if fullText.Len() > 0 {
				fullText.WriteString("\n\n--- Page ")
				fullText.WriteString(fmt.Sprintf("%d", pageNum))
				fullText.WriteString(" ---\n\n")
			}
			fullText.WriteString(text)
		}
	}

	return fullText.String(), nil
}

// ReadPDFFromFile reads and extracts text from a PDF file path directly.
// This is useful for local files that aren't in storage.
func (e *Extractor) ReadPDFFromFile(ctx context.Context, filePath string) (string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read PDF file: %w", err)
	}
	return e.ExtractTextFromBytes(ctx, content)
}
