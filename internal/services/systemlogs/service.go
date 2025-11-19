package systemlogs

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ternarybob/arbor"
)

type Service struct {
	logsDir string
	logger  arbor.ILogger
}

func NewService(logsDir string, logger arbor.ILogger) *Service {
	return &Service{
		logsDir: logsDir,
		logger:  logger,
	}
}

// ListLogFiles returns a list of available log files in the logs directory
func (s *Service) ListLogFiles() ([]LogFile, error) {
	entries, err := os.ReadDir(s.logsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read logs directory: %w", err)
	}

	var files []LogFile
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".log") {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			files = append(files, LogFile{
				Name:    entry.Name(),
				Size:    info.Size(),
				ModTime: info.ModTime(),
			})
		}
	}

	// Sort by modification time (newest first)
	sort.Slice(files, func(i, j int) bool {
		return files[i].ModTime.After(files[j].ModTime)
	})

	return files, nil
}

// GetLogContent reads a log file and returns parsed entries
// limit: max number of lines to read from the END of the file
// levels: filter by log levels (e.g. "info", "warn", "error")
func (s *Service) GetLogContent(filename string, limit int, levels []string) ([]LogEntry, error) {
	// Sanitize filename to prevent directory traversal
	filename = filepath.Base(filename)
	path := filepath.Join(s.logsDir, filename)

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}
	defer file.Close()

	// Create a level map for faster lookup - normalize to 3-letter codes
	levelMap := make(map[string]bool)
	for _, l := range levels {
		// Convert filter level to 3-letter code
		normalized := convertTo3Letter(l)
		levelMap[normalized] = true
	}
	filterLevels := len(levelMap) > 0

	var entries []LogEntry
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		entry := parseLogLine(line)

		if filterLevels {
			// Entry.Level is already in 3-letter format from parseLogLine
			if !levelMap[entry.Level] {
				continue
			}
		}

		entries = append(entries, entry)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading log file: %w", err)
	}

	// Apply limit (take last N)
	if limit > 0 && len(entries) > limit {
		entries = entries[len(entries)-limit:]
	}

	return entries, nil
}

func parseLogLine(line string) LogEntry {
	// Arbor logs are typically JSON
	// Example: {"level":"info","time":"15:04:05","message":"..."}

	var rawMap map[string]interface{}
	if err := json.Unmarshal([]byte(line), &rawMap); err == nil {
		entry := LogEntry{
			Raw: line,
		}

		// Convert level to 3-letter format
		if lvl, ok := rawMap["level"].(string); ok {
			entry.Level = convertTo3Letter(lvl)
		} else {
			entry.Level = "INF" // Default to INF if missing
		}

		if msg, ok := rawMap["message"].(string); ok {
			entry.Message = msg
		}

		if timeStr, ok := rawMap["time"].(string); ok {
			if t, err := time.Parse(time.RFC3339, timeStr); err == nil {
				entry.Timestamp = t
			} else if t, err := time.Parse("15:04:05", timeStr); err == nil {
				now := time.Now()
				entry.Timestamp = time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), t.Second(), 0, time.Local)
			}
		}

		return entry
	}

	// Try to parse standard Arbor text format: "15:04:05 INF > message"
	parts := strings.Fields(line)
	if len(parts) >= 3 && parts[2] == ">" {
		// Extract 3-letter level code (already in correct format)
		level := parts[1]

		// Normalize to 3-letter format
		switch level {
		case "INFO":
			level = "INF"
		case "WARN", "WARNING":
			level = "WRN"
		case "ERROR":
			level = "ERR"
		case "DEBUG":
			level = "DBG"
		}

		// Parse timestamp if possible
		var timestamp time.Time
		if t, err := time.Parse("15:04:05", parts[0]); err == nil {
			now := time.Now()
			timestamp = time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), t.Second(), 0, time.Local)
		}

		// Extract message (everything after the ">")
		// Find the position of ">" and take everything after it
		idx := strings.Index(line, ">")
		message := line
		if idx >= 0 && idx+1 < len(line) {
			message = strings.TrimSpace(line[idx+1:])
		}

		return LogEntry{
			Raw:       line,
			Level:     level,
			Message:   message,
			Timestamp: timestamp,
		}
	}

	// Fallback for non-JSON, non-standard lines
	return LogEntry{
		Raw:     line,
		Level:   "INF", // Default to INF so it shows up in the UI
		Message: line,
	}
}

// convertTo3Letter converts full level names to 3-letter codes
func convertTo3Letter(level string) string {
	switch strings.ToUpper(level) {
	case "INFO":
		return "INF"
	case "WARN", "WARNING":
		return "WRN"
	case "ERROR":
		return "ERR"
	case "DEBUG":
		return "DBG"
	default:
		// If already 3 letters, return as-is (uppercase)
		if len(level) == 3 {
			return strings.ToUpper(level)
		}
		return "INF"
	}
}
