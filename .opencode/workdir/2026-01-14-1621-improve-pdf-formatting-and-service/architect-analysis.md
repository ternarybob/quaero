# Architect Analysis

## Context
The `EmailWorker` currently contains tightly coupled, manual PDF generation logic. This violates the Single Responsibility Principle and makes the code harder to test and reuse. The current implementation uses `github.com/go-pdf/fpdf` with a custom line-by-line Markdown parser that strips rich formatting (bold, italics, links) and has basic table support.

## Proposed Architecture
1.  **New Component:** `PDFService` (`internal/services/pdf`).
    -   **Responsibility:** Convert Markdown content to PDF bytes.
    -   **Dependencies:** `github.com/go-pdf/fpdf`.
2.  **Integration:**
    -   `EmailWorker` depends on `PDFService` via interface.
    -   DI via the main application composition root.

## Implementation Strategy
1.  **Extraction (Phase 1):** Lift and shift the existing logic into the new service to establish the boundary without changing behavior.
2.  **Refactoring (Phase 2):** Connect the worker to the service.
3.  **Enhancement (Phase 3):** Improve the internal logic of the `PDFService`.
    -   **Parsing:** Consider switching from line-by-line regex parsing to an AST-based approach using `goldmark` (already a dependency). This allows robust handling of bold, italics, and nesting.
    -   **Tables:** Implement auto-layout for tables.

## Risks & Mitigations
-   **Risk:** Formatting regression during extraction.
    -   *Mitigation:* Create a golden file test for the current implementation before refactoring.
-   **Risk:** `fpdf` complexity for rich text.
    -   *Mitigation:* Start with basic styling (bold/italic) before attempting complex layouts. Use `fpdf.HTMLBasicNew()` if applicable, or manual font switching.

## Testing Strategy
-   **Unit:** Test `GeneratePDF` with various markdown inputs.
-   **Integration:** Verify `EmailWorker` produces attachments.
