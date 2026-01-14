package interfaces

// PDFService handles PDF generation from various formats
type PDFService interface {
	// ConvertMarkdownToPDF converts markdown content to a PDF byte slice
	ConvertMarkdownToPDF(markdown, title string) ([]byte, error)
}
