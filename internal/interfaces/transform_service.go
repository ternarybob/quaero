package interfaces

// TransformService provides HTML to markdown conversion functionality
type TransformService interface {
	// HTMLToMarkdown converts HTML content to markdown
	// baseURL is used for resolving relative links
	HTMLToMarkdown(html string, baseURL string) (string, error)

	// ValidateHTML checks if the input looks like valid HTML
	ValidateHTML(content string) error
}
