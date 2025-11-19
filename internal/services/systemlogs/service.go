package systemlogs

import (
	"fmt"
	"strings"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/arbor/services/logviewer"
)

type Service struct {
	arborService *logviewer.Service
	logger       arbor.ILogger
}

func NewService(logsDir string, logger arbor.ILogger) *Service {
	return &Service{
		arborService: logviewer.NewService(logsDir),
		logger:       logger,
	}
}

// ListLogFiles returns a list of available log files in the logs directory
func (s *Service) ListLogFiles() ([]LogFile, error) {
	arborFiles, err := s.arborService.ListLogFiles()
	if err != nil {
		return nil, fmt.Errorf("failed to list log files: %w", err)
	}

	var files []LogFile
	for _, f := range arborFiles {
		files = append(files, LogFile{
			Name:    f.Name,
			Size:    f.Size,
			ModTime: f.ModTime,
		})
	}

	return files, nil
}

// GetLogContent reads a log file and returns parsed entries
func (s *Service) GetLogContent(filename string, limit int, levels []string) ([]LogEntry, error) {
	arborEntries, err := s.arborService.GetLogContent(filename, limit, levels)
	if err != nil {
		return nil, fmt.Errorf("failed to get log content: %w", err)
	}

	var entries []LogEntry
	for _, e := range arborEntries {
		// Convert level to 3-letter string
		levelStr := e.Level.String()

		// Map to local LogEntry model
		entries = append(entries, LogEntry{
			Timestamp: e.Timestamp,
			Level:     convertTo3Letter(levelStr),
			Message:   e.Message,
			Raw:       fmt.Sprintf("%s %s > %s", e.Timestamp.Format("15:04:05"), levelStr, e.Message), // Reconstruct raw-ish string
		})
	}

	return entries, nil
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
