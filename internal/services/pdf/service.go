package pdf

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/go-pdf/fpdf"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// Service implements interfaces.PDFService
type Service struct {
	logger arbor.ILogger
}

// Compile-time assertion
var _ interfaces.PDFService = (*Service)(nil)

// NewService creates a new PDF service
func NewService(logger arbor.ILogger) *Service {
	return &Service{
		logger: logger,
	}
}

// ConvertMarkdownToPDF converts markdown content to a PDF byte slice
func (s *Service) ConvertMarkdownToPDF(markdown, title string) ([]byte, error) {
	s.logger.Debug().
		Int("markdown_len", len(markdown)).
		Str("title", title).
		Msg("Converting markdown to PDF")

	// Strip YAML frontmatter if present (e.g., email instructions)
	markdown = stripFrontmatter(markdown)

	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(10, 10, 10)
	pdf.SetAutoPageBreak(true, 10)
	pdf.AddPage()

	// Default font (reduced from 11pt to 9pt)
	pdf.SetFont("Arial", "", 9)

	// Note: Title is expected to be in the markdown content as H1 heading.
	// We don't add a separate title here to avoid duplication.
	// The title parameter is kept for PDF metadata purposes (e.g., document properties).
	_ = title // Acknowledge parameter for future PDF metadata use

	// Configure goldmark
	md := goldmark.New(
		goldmark.WithExtensions(extension.Table, extension.Strikethrough, extension.Linkify),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
	)

	source := []byte(markdown)
	doc := md.Parser().Parse(text.NewReader(source))

	renderer := &pdfRenderer{
		pdf:    pdf,
		source: source,
		logger: s.logger,
		font:   "Arial",
		size:   9,
	}

	if err := renderer.render(doc); err != nil {
		s.logger.Error().Err(err).Msg("Failed to generate PDF")
		return nil, fmt.Errorf("failed to generate PDF: %w", err)
	}

	// Get PDF bytes
	var buf bytes.Buffer
	err := pdf.Output(&buf)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to generate PDF output")
		return nil, fmt.Errorf("failed to generate PDF output: %w", err)
	}

	s.logger.Debug().Int("pdf_size", buf.Len()).Msg("PDF generated successfully")
	return buf.Bytes(), nil
}

type pdfRenderer struct {
	pdf       *fpdf.Fpdf
	source    []byte
	logger    arbor.ILogger
	font      string
	size      float64
	bold      bool
	italic    bool
	inList    bool
	listLevel int
}

func (r *pdfRenderer) render(node ast.Node) error {
	return ast.Walk(node, r.walk)
}

func (r *pdfRenderer) updateFont() {
	style := ""
	if r.bold {
		style += "B"
	}
	if r.italic {
		style += "I"
	}
	r.pdf.SetFont(r.font, style, r.size)
}

func (r *pdfRenderer) walk(n ast.Node, entering bool) (ast.WalkStatus, error) {
	switch n.Kind() {
	case ast.KindHeading:
		return r.handleHeading(n.(*ast.Heading), entering)
	case ast.KindParagraph:
		return r.handleParagraph(n.(*ast.Paragraph), entering)
	case ast.KindText:
		return r.handleText(n.(*ast.Text), entering)
	case ast.KindEmphasis:
		return r.handleEmphasis(n.(*ast.Emphasis), entering)
	case ast.KindCodeSpan:
		return r.handleCodeSpan(n.(*ast.CodeSpan), entering)
	case ast.KindFencedCodeBlock:
		return r.handleFencedCodeBlock(n.(*ast.FencedCodeBlock), entering)
	case ast.KindCodeBlock:
		return r.handleCodeBlock(n.(*ast.CodeBlock), entering)
	case ast.KindList:
		return r.handleList(n.(*ast.List), entering)
	case ast.KindListItem:
		return r.handleListItem(n.(*ast.ListItem), entering)
	case ast.KindThematicBreak:
		if entering {
			r.pdf.Ln(2)
			r.pdf.Line(15, r.pdf.GetY(), 195, r.pdf.GetY())
			r.pdf.Ln(2)
		}
	case extast.KindTable:
		return r.handleTable(n.(*extast.Table), entering)
	}
	return ast.WalkContinue, nil
}

func (r *pdfRenderer) handleHeading(n *ast.Heading, entering bool) (ast.WalkStatus, error) {
	if entering {
		r.pdf.Ln(6)
		size := 14.0
		switch n.Level {
		case 1:
			size = 14
		case 2:
			size = 12
		case 3:
			size = 11
		default:
			size = 10
		}
		r.pdf.SetFont("Arial", "B", size)
	} else {
		r.pdf.Ln(6)
		// Reset
		r.updateFont()
	}
	return ast.WalkContinue, nil
}

func (r *pdfRenderer) handleParagraph(n *ast.Paragraph, entering bool) (ast.WalkStatus, error) {
	if entering {
		// Ensure we are at the start of a line or add spacing
		if !r.inList {
			// r.pdf.Ln(1)
		}
	} else {
		r.pdf.Ln(7)
	}
	return ast.WalkContinue, nil
}

func (r *pdfRenderer) handleText(n *ast.Text, entering bool) (ast.WalkStatus, error) {
	if entering {
		text := string(n.Text(r.source))
		r.pdf.Write(5, text)
	}
	return ast.WalkContinue, nil
}

func (r *pdfRenderer) handleEmphasis(n *ast.Emphasis, entering bool) (ast.WalkStatus, error) {
	if n.Level == 2 {
		r.bold = entering
	} else {
		r.italic = entering
	}
	r.updateFont()
	return ast.WalkContinue, nil
}

func (r *pdfRenderer) handleCodeSpan(n *ast.CodeSpan, entering bool) (ast.WalkStatus, error) {
	if entering {
		r.pdf.SetFont("Courier", "", 10)
		// CodeSpan is an inline element - iterate through children to get text
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			if textNode, ok := c.(*ast.Text); ok {
				r.pdf.Write(5, string(textNode.Segment.Value(r.source)))
			}
		}
	} else {
		r.updateFont() // Restore
	}
	return ast.WalkSkipChildren, nil
}

func (r *pdfRenderer) handleFencedCodeBlock(n *ast.FencedCodeBlock, entering bool) (ast.WalkStatus, error) {
	if entering {
		r.renderCodeBlock(n.Lines())
		return ast.WalkSkipChildren, nil
	}
	return ast.WalkContinue, nil
}

func (r *pdfRenderer) handleCodeBlock(n *ast.CodeBlock, entering bool) (ast.WalkStatus, error) {
	if entering {
		r.renderCodeBlock(n.Lines())
		return ast.WalkSkipChildren, nil
	}
	return ast.WalkContinue, nil
}

func (r *pdfRenderer) renderCodeBlock(lines *text.Segments) {
	r.pdf.Ln(2)
	r.pdf.SetFont("Courier", "", 9)
	r.pdf.SetFillColor(245, 245, 245)

	for i := 0; i < lines.Len(); i++ {
		line := lines.At(i)
		txt := string(line.Value(r.source))
		r.pdf.MultiCell(0, 5, txt, "", "L", true)
	}

	r.pdf.SetFillColor(255, 255, 255)
	r.updateFont()
	r.pdf.Ln(2)
}

func (r *pdfRenderer) handleList(n *ast.List, entering bool) (ast.WalkStatus, error) {
	if entering {
		r.inList = true
		r.listLevel++
	} else {
		r.listLevel--
		if r.listLevel == 0 {
			r.inList = false
			r.pdf.Ln(2)
		}
	}
	return ast.WalkContinue, nil
}

func (r *pdfRenderer) handleListItem(n *ast.ListItem, entering bool) (ast.WalkStatus, error) {
	if entering {
		// Ensure we start on a new line before drawing the bullet
		// This prevents list items from overlapping when the previous content
		// didn't add a line break
		r.pdf.Ln(5)
		// Draw bullet
		indent := float64(r.listLevel) * 5.0
		r.pdf.SetX(15 + indent)
		r.pdf.Write(5, "- ")
	}
	return ast.WalkContinue, nil
}

func (r *pdfRenderer) handleTable(n *extast.Table, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}

	var rows [][]string

	// Recursive helper to find rows
	var findRows func(node ast.Node)
	findRows = func(node ast.Node) {
		for child := node.FirstChild(); child != nil; child = child.NextSibling() {
			if tr, ok := child.(*extast.TableRow); ok {
				rows = append(rows, r.extractRow(tr))
			} else if _, ok := child.(*extast.TableHeader); ok {
				findRows(child)
			}
		}
	}
	findRows(n)

	r.renderTable(rows)
	return ast.WalkSkipChildren, nil
}

func (r *pdfRenderer) extractRow(n *extast.TableRow) []string {
	var row []string
	for cell := n.FirstChild(); cell != nil; cell = cell.NextSibling() {
		if _, ok := cell.(*extast.TableCell); ok {
			row = append(row, string(cell.Text(r.source)))
		}
	}
	return row
}

func (r *pdfRenderer) renderTable(rows [][]string) {
	if len(rows) == 0 {
		return
	}

	r.pdf.Ln(2)

	pageWidth := 180.0
	numCols := len(rows[0])
	if numCols == 0 {
		return
	}

	// Use smaller font for tables to fit more content
	fontSize := 8.0
	lineHeight := 4.0

	// Calculate column widths based on actual content widths
	colWidths := r.calculateTableColumnWidths(rows, numCols, pageWidth, fontSize)

	for i, row := range rows {
		if i == 0 {
			r.pdf.SetFont("Arial", "B", fontSize)
			r.pdf.SetFillColor(230, 230, 230)
		} else {
			r.pdf.SetFont("Arial", "", fontSize)
			r.pdf.SetFillColor(255, 255, 255)
		}

		// Calculate max lines needed for this row using actual string widths
		maxLines := 1
		for j, cell := range row {
			if j < numCols {
				lines := r.calculateLinesNeededWithWidth(cell, colWidths[j]-2) // -2 for padding
				if lines > maxLines {
					maxLines = lines
				}
			}
		}

		// Limit max lines to prevent huge rows
		if maxLines > 8 {
			maxLines = 8
		}

		rowHeight := float64(maxLines)*lineHeight + 2 // +2 for padding
		startY := r.pdf.GetY()
		startX := r.pdf.GetX()

		// Check for page break
		pageHeight := 297.0 - 15.0 // A4 height minus margin
		if startY+rowHeight > pageHeight {
			r.pdf.AddPage()
			startY = r.pdf.GetY()
		}

		// Render each cell with word wrap
		for j, cell := range row {
			if j < numCols {
				x := startX
				for k := 0; k < j; k++ {
					x += colWidths[k]
				}

				// Draw cell background and border
				if i == 0 {
					r.pdf.SetFillColor(230, 230, 230)
					r.pdf.Rect(x, startY, colWidths[j], rowHeight, "FD")
				} else {
					r.pdf.Rect(x, startY, colWidths[j], rowHeight, "D")
				}

				// Render cell text with word wrap
				r.pdf.SetXY(x+1, startY+1)
				r.renderCellTextWithWidth(cell, colWidths[j]-2, lineHeight, maxLines)
			}
		}

		// Move to next row
		r.pdf.SetXY(startX, startY+rowHeight)
	}

	r.pdf.Ln(3)
	r.updateFont()
}

// calculateTableColumnWidths calculates optimal column widths for a table
// using actual string width measurements from the PDF library.
func (r *pdfRenderer) calculateTableColumnWidths(rows [][]string, numCols int, pageWidth float64, fontSize float64) []float64 {
	colWidths := make([]float64, numCols)

	// Set font for measurement
	r.pdf.SetFont("Arial", "", fontSize)

	// Calculate max width needed for each column using actual string widths
	for _, row := range rows {
		for i, cell := range row {
			if i < numCols {
				// Use actual string width measurement + padding
				cellWidth := r.pdf.GetStringWidth(cell) + 4
				if cellWidth > colWidths[i] {
					colWidths[i] = cellWidth
				}
			}
		}
	}

	// Also measure header widths with bold font
	if len(rows) > 0 {
		r.pdf.SetFont("Arial", "B", fontSize)
		for i, cell := range rows[0] {
			if i < numCols {
				cellWidth := r.pdf.GetStringWidth(cell) + 4
				if cellWidth > colWidths[i] {
					colWidths[i] = cellWidth
				}
			}
		}
		r.pdf.SetFont("Arial", "", fontSize)
	}

	// Apply min/max constraints
	minWidth := 12.0
	maxWidth := pageWidth / 3.0 // No column more than 1/3 of page (for tables with many columns)

	for i := range colWidths {
		if colWidths[i] < minWidth {
			colWidths[i] = minWidth
		}
		if colWidths[i] > maxWidth {
			colWidths[i] = maxWidth
		}
	}

	// Calculate total width
	totalWidth := 0.0
	for _, w := range colWidths {
		totalWidth += w
	}

	// If total exceeds page width, scale all columns proportionally
	if totalWidth > pageWidth {
		scale := pageWidth / totalWidth
		for i := range colWidths {
			colWidths[i] *= scale
			// Enforce minimum width even after scaling
			if colWidths[i] < minWidth*0.8 {
				colWidths[i] = minWidth * 0.8
			}
		}
	} else if totalWidth < pageWidth*0.9 {
		// If total is much less than page width, expand columns proportionally
		scale := (pageWidth * 0.95) / totalWidth
		if scale > 1.5 {
			scale = 1.5 // Don't expand too much
		}
		for i := range colWidths {
			colWidths[i] *= scale
		}
	}

	return colWidths
}

// calculateLinesNeeded estimates how many lines of text will be needed
// Deprecated: Use calculateLinesNeededWithWidth for accurate measurement
func (r *pdfRenderer) calculateLinesNeeded(text string, width, fontSize float64) int {
	return r.calculateLinesNeededWithWidth(text, width)
}

// calculateLinesNeededWithWidth calculates lines needed using actual string width
func (r *pdfRenderer) calculateLinesNeededWithWidth(text string, width float64) int {
	if text == "" || width <= 0 {
		return 1
	}

	// Split text into words and calculate how many lines are needed
	words := splitIntoWords(text)
	if len(words) == 0 {
		return 1
	}

	lines := 1
	currentLineWidth := 0.0
	spaceWidth := r.pdf.GetStringWidth(" ")

	for _, word := range words {
		wordWidth := r.pdf.GetStringWidth(word)

		if currentLineWidth == 0 {
			// First word on line
			currentLineWidth = wordWidth
		} else if currentLineWidth+spaceWidth+wordWidth <= width {
			// Word fits on current line
			currentLineWidth += spaceWidth + wordWidth
		} else {
			// Need new line
			lines++
			currentLineWidth = wordWidth
		}
	}

	return lines
}

// renderCellTextWithWidth renders text with word wrapping within a cell using actual string widths
func (r *pdfRenderer) renderCellTextWithWidth(text string, width, lineHeight float64, maxLines int) {
	if text == "" {
		return
	}

	// Split text into words
	words := splitIntoWords(text)
	if len(words) == 0 {
		return
	}

	// Build lines with word wrapping using actual string widths
	var lines []string
	currentLine := ""
	currentWidth := 0.0
	spaceWidth := r.pdf.GetStringWidth(" ")

	for _, word := range words {
		wordWidth := r.pdf.GetStringWidth(word)

		if currentLine == "" {
			// First word on line
			currentLine = word
			currentWidth = wordWidth
		} else if currentWidth+spaceWidth+wordWidth <= width {
			// Word fits on current line
			currentLine += " " + word
			currentWidth += spaceWidth + wordWidth
		} else {
			// Need new line
			lines = append(lines, currentLine)
			currentLine = word
			currentWidth = wordWidth
		}
	}
	if currentLine != "" {
		lines = append(lines, currentLine)
	}

	// Render lines (limited to maxLines)
	for i := 0; i < len(lines) && i < maxLines; i++ {
		line := lines[i]
		// If this is the last line and there's more content, add ellipsis
		if i == maxLines-1 && len(lines) > maxLines {
			// Truncate and add ellipsis
			for r.pdf.GetStringWidth(line+"...") > width && len(line) > 3 {
				line = line[:len(line)-1]
			}
			line += "..."
		}
		r.pdf.CellFormat(width, lineHeight, line, "", 2, "L", false, 0, "")
	}
}

// renderCellText renders text with word wrapping within a cell
// Deprecated: Use renderCellTextWithWidth for accurate measurement
func (r *pdfRenderer) renderCellText(text string, width, lineHeight float64, maxLines int) {
	r.renderCellTextWithWidth(text, width, lineHeight, maxLines)
}

// splitIntoWords splits text into words for word wrapping
func splitIntoWords(text string) []string {
	var words []string
	current := ""
	for _, c := range text {
		if c == ' ' || c == '\t' || c == '\n' {
			if current != "" {
				words = append(words, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		words = append(words, current)
	}
	return words
}

// stripFrontmatter removes YAML frontmatter from markdown content.
// Frontmatter is delimited by --- at the start of the content.
// This ensures PDF output doesn't include email instructions or other metadata.
func stripFrontmatter(markdown string) string {
	if !strings.HasPrefix(markdown, "---\n") {
		return markdown
	}

	// Find the end of frontmatter (---\n after the opening ---)
	endIdx := strings.Index(markdown[4:], "\n---\n")
	if endIdx == -1 {
		// No closing frontmatter delimiter found
		return markdown
	}

	// Return content after the frontmatter, trimmed
	return strings.TrimSpace(markdown[4+endIdx+5:])
}
