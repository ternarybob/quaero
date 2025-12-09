# Step 3: Remove extract_structure worker registration from app.go
Model: sonnet | Status: ✅

## Done
- Deleted: Lines 699-706 that created and registered extractStructureWorker
- Removed: Worker instantiation with NewExtractStructureWorker()
- Removed: RegisterWorker() call for extractStructureWorker
- Removed: Debug logging for extract structure worker registration

## Files Changed
- `internal/app/app.go` - Removed extract_structure worker registration (8 lines deleted)

## Build Check
Build: ⏳ | Tests: ⏭️
