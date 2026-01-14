# Step 2: Refactor EmailWorker

## Task
Update `EmailWorker` to use the new `PDFService`.

## Actions
1.  **Update Struct:** Add `pdfService *pdf.Service` (or interface) to `EmailWorker` struct.
2.  **Update Constructor:** `NewEmailWorker` should accept the PDF service.
3.  **Update Usage:** Replace calls to `convertMarkdownToPDF` with `w.pdfService.GeneratePDF(...)`.
4.  **Cleanup:** Remove the old `convertMarkdownToPDF`, `renderPDFTable`, and related helper functions from `email_worker.go`.
5.  **Wiring:** Update `cmd/quaero/main.go` (or `wire.go` / `app.go`) to initialize `PDFService` and pass it to `EmailWorker`.

## Deliverables
-   Modified `internal/workers/output/email_worker.go`
-   Modified wiring code (likely in `cmd/quaero/` or `internal/app/`).
