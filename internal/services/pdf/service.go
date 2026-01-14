package pdf

import (
	"bytes"
	"fmt"

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

	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.SetAutoPageBreak(true, 15)
	pdf.AddPage()

	// Default font
	pdf.SetFont("Arial", "", 11)

	// Set title
	if title != "" {
		pdf.SetFont("Arial", "B", 16)
		pdf.MultiCell(0, 8, title, "", "L", false)
		pdf.Ln(5)
		pdf.SetFont("Arial", "", 11)
	}

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
		size:   11,
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
		r.pdf.Ln(4)
		size := 16.0
		switch n.Level {
		case 1:
			size = 16
		case 2:
			size = 14
		case 3:
			size = 13
		default:
			size = 12
		}
		r.pdf.SetFont("Arial", "B", size)
	} else {
		r.pdf.Ln(4)
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
		r.pdf.Ln(5)
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
		for i := 0; i < n.Lines().Len(); i++ {
			line := n.Lines().At(i)
			r.pdf.Write(5, string(line.Value(r.source)))
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

	// Simple dynamic width
	colMaxLens := make([]int, numCols)
	totalLen := 0

	for _, row := range rows {
		for i, cell := range row {
			if i < numCols {
				l := len(cell)
				if l > colMaxLens[i] {
					colMaxLens[i] = l
				}
			}
		}
	}

	for _, l := range colMaxLens {
		totalLen += l
	}

	colWidths := make([]float64, numCols)
	if totalLen == 0 {
		// Equal distribution
		w := pageWidth / float64(numCols)
		for i := range colWidths {
			colWidths[i] = w
		}
	} else {
		// Proportional distribution
		minWidth := 20.0

		for i, l := range colMaxLens {
			ratio := float64(l) / float64(totalLen)
			w := ratio * pageWidth
			if w < minWidth {
				w = minWidth
			}
			colWidths[i] = w
		}

		// Normalize
		currentTotal := 0.0
		for _, w := range colWidths {
			currentTotal += w
		}

		scale := pageWidth / currentTotal
		for i := range colWidths {
			colWidths[i] *= scale
		}
	}

	// Render
	r.pdf.SetFont("Arial", "", 10)

	for i, row := range rows {
		if i == 0 {
			r.pdf.SetFont("Arial", "B", 10)
			r.pdf.SetFillColor(230, 230, 230)
		} else {
			r.pdf.SetFont("Arial", "", 10)
			r.pdf.SetFillColor(255, 255, 255)
		}

		maxHeight := 6.0

		for j, cell := range row {
			if j < numCols {
				w := colWidths[j]
				// Basic truncation
				maxChars := int(w / 2.0)
				display := cell
				if len(display) > maxChars && maxChars > 3 {
					display = display[:maxChars-3] + "..."
				}

				r.pdf.CellFormat(w, maxHeight, display, "1", 0, "L", i == 0, 0, "")
			}
		}
		r.pdf.Ln(-1)
	}

	r.pdf.Ln(3)
	r.updateFont()
}
