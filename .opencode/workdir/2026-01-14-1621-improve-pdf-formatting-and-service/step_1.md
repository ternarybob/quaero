# Step 1: Create PDF Service

## Task
Create a new `internal/services/pdf` package and implement the `PDFService`.

## Actions
1.  **Create Package:** `mkdir -p internal/services/pdf`
2.  **Define Interface:** `internal/interfaces/pdf_service.go` (or keep it in the service package if it's the only consumer, but `internal/interfaces` is consistent with project structure). *Correction:* Project seems to use `internal/interfaces` for shared interfaces.
3.  **Implement Service:**
    -   Copy logic from `email_worker.go` (`convertMarkdownToPDF` and helpers).
    -   Refactor to match the new interface.
    -   Ensure `fpdf` is imported correctly.
4.  **Unit Tests:** Create `internal/services/pdf/service_test.go` to verify PDF generation (check for non-empty bytes, maybe some content checks if possible).

## Deliverables
-   `internal/services/pdf/service.go`
-   `internal/services/pdf/service_test.go`
