package github

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/phuslu/log"
	"github.com/ternarybob/quaero/internal/models"
)

// BatchFetcher handles efficient bulk file fetching from GitHub
type BatchFetcher struct {
	connector   *Connector
	batchSize   int // Number of files per GraphQL request
	maxFileSize int // Files larger than this use REST fallback (bytes)
}

// BatchStats contains statistics about the batch fetch operation
type BatchStats struct {
	TotalFiles    int
	SuccessCount  int
	ErrorCount    int
	BytesFetched  int64
	Duration      time.Duration
	BatchCount    int
	FallbackCount int // Files fetched via REST fallback
}

// BatchResult contains the results of a batch fetch operation
type BatchResult struct {
	Documents []*models.Document
	Errors    []FileError
	Stats     BatchStats
}

// FileError represents an error fetching a specific file
type FileError struct {
	Path  string
	Error error
}

// ProgressCallback is called after each batch completes
type ProgressCallback func(processed, total int, currentBatch int)

// NewBatchFetcher creates a new BatchFetcher with default settings
func NewBatchFetcher(connector *Connector) *BatchFetcher {
	return &BatchFetcher{
		connector:   connector,
		batchSize:   50,          // 50 files per GraphQL request
		maxFileSize: 1024 * 1024, // 1MB limit for GraphQL
	}
}

// WithBatchSize sets the batch size
func (bf *BatchFetcher) WithBatchSize(size int) *BatchFetcher {
	if size > 0 && size <= 100 {
		bf.batchSize = size
	}
	return bf
}

// WithMaxFileSize sets the maximum file size for GraphQL fetching
func (bf *BatchFetcher) WithMaxFileSize(size int) *BatchFetcher {
	if size > 0 {
		bf.maxFileSize = size
	}
	return bf
}

// FetchFiles fetches all files and returns documents
func (bf *BatchFetcher) FetchFiles(ctx context.Context, owner, repo, branch string, files []RepoFile) (*BatchResult, error) {
	return bf.FetchFilesWithProgress(ctx, owner, repo, branch, files, nil)
}

// FetchFilesWithProgress fetches files with progress callbacks
func (bf *BatchFetcher) FetchFilesWithProgress(ctx context.Context, owner, repo, branch string, files []RepoFile, progress ProgressCallback) (*BatchResult, error) {
	startTime := time.Now()

	result := &BatchResult{
		Documents: make([]*models.Document, 0, len(files)),
		Errors:    make([]FileError, 0),
		Stats: BatchStats{
			TotalFiles: len(files),
		},
	}

	if len(files) == 0 {
		result.Stats.Duration = time.Since(startTime)
		return result, nil
	}

	// Categorize files
	batchable, oversized := bf.categorizeFiles(files)

	log.Info().
		Int("total_files", len(files)).
		Int("batchable", len(batchable)).
		Int("oversized", len(oversized)).
		Msg("Categorized files for batch fetching")

	// Process batchable files via GraphQL
	if len(batchable) > 0 {
		batchDocs, batchErrs := bf.fetchBatchFiles(ctx, owner, repo, branch, batchable, progress)
		result.Documents = append(result.Documents, batchDocs...)
		result.Errors = append(result.Errors, batchErrs...)
		result.Stats.SuccessCount += len(batchDocs)
		result.Stats.ErrorCount += len(batchErrs)
	}

	// Process oversized files via REST
	if len(oversized) > 0 {
		restDocs, restErrs := bf.fetchOversizedFiles(ctx, owner, repo, branch, oversized)
		result.Documents = append(result.Documents, restDocs...)
		result.Errors = append(result.Errors, restErrs...)
		result.Stats.SuccessCount += len(restDocs)
		result.Stats.ErrorCount += len(restErrs)
		result.Stats.FallbackCount = len(oversized)
	}

	// Calculate bytes fetched
	for _, doc := range result.Documents {
		result.Stats.BytesFetched += int64(len(doc.ContentMarkdown))
	}

	result.Stats.Duration = time.Since(startTime)

	log.Info().
		Int("success", result.Stats.SuccessCount).
		Int("errors", result.Stats.ErrorCount).
		Int64("bytes", result.Stats.BytesFetched).
		Dur("duration", result.Stats.Duration).
		Msg("Batch fetch complete")

	return result, nil
}

// categorizeFiles splits files into batchable (GraphQL) and oversized (REST)
func (bf *BatchFetcher) categorizeFiles(files []RepoFile) (batchable, oversized []RepoFile) {
	batchable = make([]RepoFile, 0, len(files))
	oversized = make([]RepoFile, 0)

	for _, file := range files {
		if file.Size > bf.maxFileSize {
			oversized = append(oversized, file)
		} else {
			batchable = append(batchable, file)
		}
	}

	return batchable, oversized
}

// fetchBatchFiles fetches files via GraphQL in batches
func (bf *BatchFetcher) fetchBatchFiles(ctx context.Context, owner, repo, branch string, files []RepoFile, progress ProgressCallback) ([]*models.Document, []FileError) {
	var allDocs []*models.Document
	var allErrors []FileError

	// Split into batches
	batches := bf.splitIntoBatches(files)
	processed := 0

	for batchIdx, batch := range batches {
		select {
		case <-ctx.Done():
			allErrors = append(allErrors, FileError{
				Path:  "batch",
				Error: ctx.Err(),
			})
			return allDocs, allErrors
		default:
		}

		// Extract paths for this batch
		paths := make([]string, len(batch))
		for i, file := range batch {
			paths[i] = file.Path
		}

		// Fetch via GraphQL
		results, err := bf.connector.BulkGetFileContent(ctx, owner, repo, branch, paths)
		if err != nil {
			log.Error().Err(err).Int("batch", batchIdx).Msg("GraphQL batch failed")
			// Fall back to REST for this batch
			docs, errs := bf.fetchOversizedFiles(ctx, owner, repo, branch, batch)
			allDocs = append(allDocs, docs...)
			allErrors = append(allErrors, errs...)
		} else {
			// Process results
			for i, result := range results {
				if result.Error != nil {
					allErrors = append(allErrors, FileError{
						Path:  result.Path,
						Error: result.Error,
					})
					continue
				}

				// Create document
				doc := bf.createDocument(owner, repo, branch, batch[i], result.Content)
				allDocs = append(allDocs, doc)
			}
		}

		processed += len(batch)
		if progress != nil {
			progress(processed, len(files), batchIdx+1)
		}
	}

	return allDocs, allErrors
}

// fetchOversizedFiles fetches files via REST API concurrently
func (bf *BatchFetcher) fetchOversizedFiles(ctx context.Context, owner, repo, branch string, files []RepoFile) ([]*models.Document, []FileError) {
	if len(files) == 0 {
		return nil, nil
	}

	type result struct {
		doc *models.Document
		err *FileError
	}

	results := make(chan result, len(files))
	var wg sync.WaitGroup

	// Semaphore for rate limiting (5 concurrent requests)
	sem := make(chan struct{}, 5)

	for _, file := range files {
		wg.Add(1)
		go func(f RepoFile) {
			defer wg.Done()

			select {
			case <-ctx.Done():
				results <- result{err: &FileError{Path: f.Path, Error: ctx.Err()}}
				return
			case sem <- struct{}{}:
				defer func() { <-sem }()
			}

			content, err := bf.connector.GetFileContent(ctx, owner, repo, branch, f.Path)
			if err != nil {
				results <- result{err: &FileError{Path: f.Path, Error: err}}
				return
			}

			doc := bf.createDocument(owner, repo, branch, f, content.Content)
			results <- result{doc: doc}
		}(file)
	}

	// Wait and close results channel
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	var docs []*models.Document
	var errs []FileError

	for r := range results {
		if r.err != nil {
			errs = append(errs, *r.err)
		} else if r.doc != nil {
			docs = append(docs, r.doc)
		}
	}

	return docs, errs
}

// splitIntoBatches splits files into batches of batchSize
func (bf *BatchFetcher) splitIntoBatches(files []RepoFile) [][]RepoFile {
	if len(files) == 0 {
		return nil
	}

	numBatches := (len(files) + bf.batchSize - 1) / bf.batchSize
	batches := make([][]RepoFile, 0, numBatches)

	for i := 0; i < len(files); i += bf.batchSize {
		end := i + bf.batchSize
		if end > len(files) {
			end = len(files)
		}
		batches = append(batches, files[i:end])
	}

	return batches
}

// createDocument creates a Document model from file data
func (bf *BatchFetcher) createDocument(owner, repo, branch string, file RepoFile, content string) *models.Document {
	ext := filepath.Ext(file.Name)
	if ext != "" {
		ext = ext[1:] // Remove leading dot
	}

	return &models.Document{
		SourceType:      "github_repo",
		SourceID:        fmt.Sprintf("%s/%s", owner, repo),
		Title:           file.Path,
		ContentMarkdown: content,
		Metadata: map[string]interface{}{
			"owner":     owner,
			"repo":      repo,
			"branch":    branch,
			"path":      file.Path,
			"folder":    file.Folder,
			"sha":       file.SHA,
			"file_type": ext,
		},
	}
}
