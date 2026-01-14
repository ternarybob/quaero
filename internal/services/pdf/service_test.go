package pdf

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/ternarybob/arbor"
)

func TestConvertMarkdownToPDF(t *testing.T) {
	// Setup
	logger := arbor.NewLogger()
	service := NewService(logger)

	tests := []struct {
		name     string
		markdown string
		title    string
		wantErr  bool
	}{
		{
			name:     "Basic Markdown",
			markdown: "# Title\n\nSome paragraph text.\n\n- Item 1\n- Item 2",
			title:    "Test Document",
			wantErr:  false,
		},
		{
			name:     "Empty Markdown",
			markdown: "",
			title:    "Empty Doc",
			wantErr:  false,
		},
		{
			name: "Complex Markdown with Code and Table",
			markdown: `# Header 1

Some text.

| Col 1 | Col 2 |
|-------|-------|
| Val 1 | Val 2 |

` + "```go\nfunc main() {}\n```",
			title:   "Complex Doc",
			wantErr: false,
		},
		{
			name:     "Bold and Italic",
			markdown: "Normal **Bold** *Italic* ***BoldItalic***",
			title:    "Styling",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pdfBytes, err := service.ConvertMarkdownToPDF(tt.markdown, tt.title)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, pdfBytes)
			assert.NotEmpty(t, pdfBytes)

			// Basic PDF header check
			assert.Equal(t, "%PDF", string(pdfBytes[:4]))
		})
	}
}

func TestConvertMarkdownToPDF_Tables(t *testing.T) {
	logger := arbor.NewLogger()
	service := NewService(logger)

	markdown := `
# Table Test

| ID | Name | Role | Description |
|----|------|------|-------------|
| 1  | Alice| Admin| Super user  |
| 2  | Bob  | User | Normal user |

End of table.
`
	pdfBytes, err := service.ConvertMarkdownToPDF(markdown, "Table Report")
	assert.NoError(t, err)
	assert.NotNil(t, pdfBytes)
	assert.Greater(t, len(pdfBytes), 500) // Ensure substantial content
	assert.Equal(t, "%PDF", string(pdfBytes[:4]))
}
