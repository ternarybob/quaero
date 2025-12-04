// -----------------------------------------------------------------------
// Image Storage Service
// Downloads and stores images locally for crawled documents
// -----------------------------------------------------------------------

package crawler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/ternarybob/arbor"
)

// ImageStorageConfig holds configuration for image storage
type ImageStorageConfig struct {
	// BaseDir is the base directory for storing images (e.g., "./data/images")
	BaseDir string

	// MaxImageSize is the maximum image size to download (default: 10MB)
	MaxImageSize int64

	// Timeout for downloading each image
	DownloadTimeout time.Duration

	// Concurrency for parallel downloads
	Concurrency int

	// UserAgent for HTTP requests
	UserAgent string
}

// DefaultImageStorageConfig returns sensible defaults
func DefaultImageStorageConfig() ImageStorageConfig {
	return ImageStorageConfig{
		BaseDir:         "./data/images",
		MaxImageSize:    10 * 1024 * 1024, // 10MB
		DownloadTimeout: 30 * time.Second,
		Concurrency:     5,
		UserAgent:       "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	}
}

// StoredImage represents a downloaded and stored image
type StoredImage struct {
	OriginalURL string `json:"original_url"`
	LocalPath   string `json:"local_path"` // Relative path from BaseDir
	FullPath    string `json:"full_path"`  // Absolute path
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
	Hash        string `json:"hash"` // SHA256 hash for deduplication
	Error       string `json:"error,omitempty"`
}

// ImageStorageService handles downloading and storing images
type ImageStorageService struct {
	config ImageStorageConfig
	logger arbor.ILogger
	client *http.Client

	// Cache of hash -> local path for deduplication
	hashCache   map[string]string
	hashCacheMu sync.RWMutex
}

// NewImageStorageService creates a new image storage service
func NewImageStorageService(config ImageStorageConfig, logger arbor.ILogger) (*ImageStorageService, error) {
	// Ensure base directory exists
	if err := os.MkdirAll(config.BaseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create image directory: %w", err)
	}

	client := &http.Client{
		Timeout: config.DownloadTimeout,
	}

	return &ImageStorageService{
		config:    config,
		logger:    logger,
		client:    client,
		hashCache: make(map[string]string),
	}, nil
}

// ProcessHTMLImages extracts images from HTML, downloads them, and returns modified HTML with local paths
func (s *ImageStorageService) ProcessHTMLImages(ctx context.Context, html string, pageURL string, cookies []*http.Cookie) (string, []StoredImage, error) {
	// Parse base URL
	baseURL, err := url.Parse(pageURL)
	if err != nil {
		return html, nil, fmt.Errorf("invalid page URL: %w", err)
	}

	// Extract image URLs from HTML
	imageURLs := s.extractImageURLs(html, baseURL)

	if len(imageURLs) == 0 {
		return html, nil, nil
	}

	s.logger.Debug().
		Str("page_url", pageURL).
		Int("image_count", len(imageURLs)).
		Msg("Processing images from page")

	// Download images concurrently
	storedImages := s.downloadImages(ctx, imageURLs, baseURL, cookies)

	// Replace URLs in HTML
	modifiedHTML := html
	for _, img := range storedImages {
		if img.Error == "" && img.LocalPath != "" {
			// Replace original URL with local path
			// Use relative path from data directory
			localURL := "/data/images/" + img.LocalPath
			modifiedHTML = strings.ReplaceAll(modifiedHTML, img.OriginalURL, localURL)
		}
	}

	return modifiedHTML, storedImages, nil
}

// extractImageURLs extracts all image URLs from HTML
func (s *ImageStorageService) extractImageURLs(html string, baseURL *url.URL) []string {
	var urls []string
	seen := make(map[string]bool)

	// Match src attributes in img tags
	imgSrcRegex := regexp.MustCompile(`<img[^>]+src=["']([^"']+)["']`)
	matches := imgSrcRegex.FindAllStringSubmatch(html, -1)

	for _, match := range matches {
		if len(match) > 1 {
			imgURL := match[1]

			// Skip data URLs (already embedded)
			if strings.HasPrefix(imgURL, "data:") {
				continue
			}

			// Resolve relative URLs
			absoluteURL := s.resolveURL(imgURL, baseURL)
			if absoluteURL != "" && !seen[absoluteURL] {
				seen[absoluteURL] = true
				urls = append(urls, absoluteURL)
			}
		}
	}

	// Also match srcset attributes
	srcsetRegex := regexp.MustCompile(`srcset=["']([^"']+)["']`)
	srcsetMatches := srcsetRegex.FindAllStringSubmatch(html, -1)

	for _, match := range srcsetMatches {
		if len(match) > 1 {
			// srcset can contain multiple URLs with descriptors
			srcset := match[1]
			parts := strings.Split(srcset, ",")
			for _, part := range parts {
				part = strings.TrimSpace(part)
				// Extract URL (before space and descriptor)
				if idx := strings.Index(part, " "); idx > 0 {
					part = part[:idx]
				}
				if strings.HasPrefix(part, "data:") {
					continue
				}
				absoluteURL := s.resolveURL(part, baseURL)
				if absoluteURL != "" && !seen[absoluteURL] {
					seen[absoluteURL] = true
					urls = append(urls, absoluteURL)
				}
			}
		}
	}

	// Match background-image in style attributes
	bgImageRegex := regexp.MustCompile(`background-image:\s*url\(['"]?([^'")\s]+)['"]?\)`)
	bgMatches := bgImageRegex.FindAllStringSubmatch(html, -1)

	for _, match := range bgMatches {
		if len(match) > 1 {
			imgURL := match[1]
			if strings.HasPrefix(imgURL, "data:") {
				continue
			}
			absoluteURL := s.resolveURL(imgURL, baseURL)
			if absoluteURL != "" && !seen[absoluteURL] {
				seen[absoluteURL] = true
				urls = append(urls, absoluteURL)
			}
		}
	}

	return urls
}

// resolveURL resolves a potentially relative URL against a base URL
func (s *ImageStorageService) resolveURL(imgURL string, baseURL *url.URL) string {
	// Already absolute
	if strings.HasPrefix(imgURL, "http://") || strings.HasPrefix(imgURL, "https://") {
		return imgURL
	}

	// Protocol-relative URL
	if strings.HasPrefix(imgURL, "//") {
		return baseURL.Scheme + ":" + imgURL
	}

	// Resolve relative URL
	ref, err := url.Parse(imgURL)
	if err != nil {
		return ""
	}

	resolved := baseURL.ResolveReference(ref)
	return resolved.String()
}

// downloadImages downloads images concurrently
func (s *ImageStorageService) downloadImages(ctx context.Context, urls []string, baseURL *url.URL, cookies []*http.Cookie) []StoredImage {
	results := make([]StoredImage, len(urls))
	var wg sync.WaitGroup

	// Semaphore for concurrency control
	sem := make(chan struct{}, s.config.Concurrency)

	for i, imgURL := range urls {
		wg.Add(1)
		go func(idx int, imageURL string) {
			defer wg.Done()

			sem <- struct{}{}        // Acquire
			defer func() { <-sem }() // Release

			results[idx] = s.downloadImage(ctx, imageURL, baseURL, cookies)
		}(i, imgURL)
	}

	wg.Wait()
	return results
}

// downloadImage downloads a single image and stores it
func (s *ImageStorageService) downloadImage(ctx context.Context, imageURL string, baseURL *url.URL, cookies []*http.Cookie) StoredImage {
	result := StoredImage{
		OriginalURL: imageURL,
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", imageURL, nil)
	if err != nil {
		result.Error = fmt.Sprintf("create request: %v", err)
		return result
	}

	// Set headers
	req.Header.Set("User-Agent", s.config.UserAgent)
	req.Header.Set("Referer", baseURL.String())

	// Add cookies for authentication
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}

	// Download
	resp, err := s.client.Do(req)
	if err != nil {
		result.Error = fmt.Sprintf("download: %v", err)
		return result
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		result.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
		return result
	}

	// Check content type
	contentType := resp.Header.Get("Content-Type")
	if !s.isImageContentType(contentType) {
		result.Error = fmt.Sprintf("not an image: %s", contentType)
		return result
	}
	result.ContentType = contentType

	// Read body with size limit
	limitReader := io.LimitReader(resp.Body, s.config.MaxImageSize+1)
	data, err := io.ReadAll(limitReader)
	if err != nil {
		result.Error = fmt.Sprintf("read: %v", err)
		return result
	}

	if int64(len(data)) > s.config.MaxImageSize {
		result.Error = "image too large"
		return result
	}

	result.Size = int64(len(data))

	// Calculate hash for deduplication
	hash := sha256.Sum256(data)
	result.Hash = hex.EncodeToString(hash[:])

	// Check if we already have this image
	s.hashCacheMu.RLock()
	existingPath, exists := s.hashCache[result.Hash]
	s.hashCacheMu.RUnlock()

	if exists {
		result.LocalPath = existingPath
		result.FullPath = filepath.Join(s.config.BaseDir, existingPath)
		s.logger.Debug().
			Str("url", imageURL).
			Str("path", existingPath).
			Msg("Image already cached (deduplicated)")
		return result
	}

	// Generate filename from hash and extension
	ext := s.getExtensionFromContentType(contentType)
	if ext == "" {
		ext = s.getExtensionFromURL(imageURL)
	}
	if ext == "" {
		ext = ".bin"
	}

	// Organize by first 2 chars of hash for directory distribution
	subDir := result.Hash[:2]
	filename := result.Hash + ext
	localPath := filepath.Join(subDir, filename)
	fullPath := filepath.Join(s.config.BaseDir, localPath)

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		result.Error = fmt.Sprintf("create dir: %v", err)
		return result
	}

	// Write file
	if err := os.WriteFile(fullPath, data, 0644); err != nil {
		result.Error = fmt.Sprintf("write: %v", err)
		return result
	}

	result.LocalPath = localPath
	result.FullPath = fullPath

	// Cache the hash
	s.hashCacheMu.Lock()
	s.hashCache[result.Hash] = localPath
	s.hashCacheMu.Unlock()

	s.logger.Debug().
		Str("url", imageURL).
		Str("path", localPath).
		Int64("size", result.Size).
		Msg("Image downloaded and stored")

	return result
}

// isImageContentType checks if content type is an image
func (s *ImageStorageService) isImageContentType(contentType string) bool {
	contentType = strings.ToLower(contentType)
	return strings.HasPrefix(contentType, "image/")
}

// getExtensionFromContentType returns file extension for content type
func (s *ImageStorageService) getExtensionFromContentType(contentType string) string {
	contentType = strings.ToLower(contentType)
	contentType = strings.Split(contentType, ";")[0] // Remove parameters

	switch contentType {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "image/svg+xml":
		return ".svg"
	case "image/bmp":
		return ".bmp"
	case "image/ico", "image/x-icon", "image/vnd.microsoft.icon":
		return ".ico"
	default:
		return ""
	}
}

// getExtensionFromURL extracts extension from URL path
func (s *ImageStorageService) getExtensionFromURL(imageURL string) string {
	parsed, err := url.Parse(imageURL)
	if err != nil {
		return ""
	}

	ext := filepath.Ext(parsed.Path)
	ext = strings.ToLower(ext)

	// Only return known image extensions
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg", ".bmp", ".ico":
		return ext
	default:
		return ""
	}
}

// GetStoredImagePath returns the full path for a stored image by hash
func (s *ImageStorageService) GetStoredImagePath(hash string) (string, bool) {
	s.hashCacheMu.RLock()
	defer s.hashCacheMu.RUnlock()

	localPath, exists := s.hashCache[hash]
	if !exists {
		return "", false
	}

	return filepath.Join(s.config.BaseDir, localPath), true
}

// CleanupOrphanedImages removes images not referenced by any document
// This should be called periodically or manually
func (s *ImageStorageService) CleanupOrphanedImages(ctx context.Context, referencedHashes map[string]bool) (int, error) {
	removed := 0

	err := filepath.Walk(s.config.BaseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// Extract hash from filename
		filename := filepath.Base(path)
		ext := filepath.Ext(filename)
		hash := strings.TrimSuffix(filename, ext)

		// Check if referenced
		if !referencedHashes[hash] {
			if err := os.Remove(path); err != nil {
				s.logger.Warn().Err(err).Str("path", path).Msg("Failed to remove orphaned image")
			} else {
				removed++
				s.logger.Debug().Str("path", path).Msg("Removed orphaned image")
			}
		}

		return nil
	})

	return removed, err
}
