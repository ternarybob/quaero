# PDF Service Requirements

## Goals
1.  **Extract PDF Logic:** Move PDF generation logic from `internal/workers/output/email_worker.go` to a reusable service `internal/services/pdf`.
2.  **Improve Formatting:** Enhance the PDF output quality, specifically tables and text styling.

## Functional Requirements
-   **Service Interface:**
    ```go
    type Service interface {
        GeneratePDF(ctx context.Context, content string, options PDFOptions) ([]byte, error)
    }
    ```
-   **Markdown Support:**
    -   Headings (H1-H3)
    -   Lists (Bullet, Numbered)
    -   Tables (with headers and proper cell padding)
    -   Code blocks (monospaced font, background color)
    -   Text styling (Bold, Italic) - *Currently stripped in existing implementation*
    -   Horizontal Rules

## Technical Requirements
-   **Library:** Continue using `github.com/go-pdf/fpdf`.
-   **Architecture:**
    -   Create `internal/services/pdf/` package.
    -   Register service in `cmd/quaero/main.go` (or wherever services are wired).
    -   Inject `pdf.Service` into `EmailWorker`.
-   **Testing:**
    -   Unit tests for `GeneratePDF`.
    -   Integration test ensuring `EmailWorker` produces a valid PDF.

## Current Limitations (to address)
-   Bold/Italic markdown is currently stripped.
-   Tables have fixed/naive column width calculation.
-   Links are stripped to plain text.
