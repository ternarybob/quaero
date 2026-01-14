# Step 3: Improve PDF Formatting

## Task
Enhance the `PDFService` to support better formatting.

## Actions
1.  **Table Improvements:**
    -   Implement dynamic column width calculation based on content (up to a max).
    -   Add better border/header styling.
2.  **Text Styling:**
    -   Support Bold (`**text**`) and Italic (`*text*`) instead of stripping them.
    -   *Tech Note:* `fpdf` supports basic HTML-like tags or explicit font switching. Since the input is Markdown, we might need a small parser or convert Markdown -> HTML -> fpdf HTML support (if `fpdf`'s HTML support is good enough).
    -   *Alternative:* Use a markdown parser (like `goldmark` which is already in use) to traverse the AST and drive `fpdf`. This is more robust than regex.
3.  **Links:**
    -   Render links as clickable text in PDF.

## Deliverables
-   Enhanced `internal/services/pdf/service.go`
-   Updated tests.
