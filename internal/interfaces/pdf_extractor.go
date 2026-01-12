// -----------------------------------------------------------------------
// PDF Extractor Interface - Extract text content from PDF documents
// -----------------------------------------------------------------------

package interfaces

import (
	"context"
)

// PDFPageContent represents extracted content from a single PDF page
type PDFPageContent struct {
	PageNumber int    `json:"page_number"`
	Text       string `json:"text"`
}

// PDFTableData represents extracted tabular data from a PDF
type PDFTableData struct {
	PageNumber int        `json:"page_number"`
	Headers    []string   `json:"headers,omitempty"`
	Rows       [][]string `json:"rows"`
}

// PDFMetadata contains metadata about a PDF document
type PDFMetadata struct {
	Title       string `json:"title,omitempty"`
	Author      string `json:"author,omitempty"`
	Subject     string `json:"subject,omitempty"`
	Creator     string `json:"creator,omitempty"`
	Producer    string `json:"producer,omitempty"`
	PageCount   int    `json:"page_count"`
	FileSize    int64  `json:"file_size"`
	IsEncrypted bool   `json:"is_encrypted"`
}

// PDFExtractionResult contains the complete extraction result
type PDFExtractionResult struct {
	Metadata PDFMetadata      `json:"metadata"`
	Pages    []PDFPageContent `json:"pages"`
	FullText string           `json:"full_text"`
	Tables   []PDFTableData   `json:"tables,omitempty"`
}

// PDFExtractor defines the interface for extracting content from PDF documents.
// This interface abstracts the PDF extraction implementation, allowing different
// backends (pdfcpu, Apache Tika, AWS Textract, etc.) to be used interchangeably.
type PDFExtractor interface {
	// ExtractText extracts all text content from a PDF stored at the given storage key.
	// Returns the full text content concatenated from all pages.
	ExtractText(ctx context.Context, storageKey string) (string, error)

	// ExtractPages extracts text content by page from a PDF.
	// Returns a slice of PDFPageContent with page numbers and text.
	ExtractPages(ctx context.Context, storageKey string) ([]PDFPageContent, error)

	// ExtractWithMetadata performs full extraction including metadata, pages, and text.
	// This is useful when you need complete information about the PDF.
	ExtractWithMetadata(ctx context.Context, storageKey string) (*PDFExtractionResult, error)

	// ExtractPageRange extracts text from specific pages (1-indexed, inclusive).
	// Useful for large documents where only certain sections are needed.
	ExtractPageRange(ctx context.Context, storageKey string, startPage, endPage int) ([]PDFPageContent, error)

	// GetMetadata retrieves PDF metadata without extracting text content.
	// This is a lightweight operation useful for checking document properties.
	GetMetadata(ctx context.Context, storageKey string) (*PDFMetadata, error)
}
