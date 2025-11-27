package badger

import (
	"bufio"
	"context"
	"os"
	"strings"
)

// LoadEnvFile loads variables from a .env file into the KV store
// Format supported:
//   - KEY=value
//   - KEY="value" or KEY='value' (quotes stripped)
//   - # comments (lines starting with #)
//   - Empty lines are ignored
func (m *Manager) LoadEnvFile(ctx context.Context, filePath string) error {
	m.logger.Debug().Str("file", filePath).Msg("Loading variables from .env file")

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		m.logger.Debug().Str("file", filePath).Msg(".env file does not exist, skipping")
		return nil
	}

	file, err := os.Open(filePath)
	if err != nil {
		m.logger.Warn().Err(err).Str("file", filePath).Msg("Failed to open .env file")
		return nil // Non-fatal
	}
	defer file.Close()

	loadedCount := 0
	skippedCount := 0
	errorCount := 0

	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse KEY=value format
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			m.logger.Warn().
				Str("file", filePath).
				Int("line", lineNum).
				Msg("Invalid line format, expected KEY=value")
			skippedCount++
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Skip empty keys
		if key == "" {
			skippedCount++
			continue
		}

		// Strip surrounding quotes from value
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') ||
				(value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}

		// Skip empty values
		if value == "" {
			m.logger.Warn().
				Str("file", filePath).
				Str("key", key).
				Msg("Skipping variable with empty value")
			skippedCount++
			continue
		}

		// Store in KV store
		description := "Loaded from .env file"
		isNew, err := m.kv.Upsert(ctx, key, value, description)
		if err != nil {
			m.logger.Error().Err(err).Str("key", key).Msg("Failed to store variable from .env")
			errorCount++
			continue
		}

		if isNew {
			m.logger.Debug().Str("key", key).Msg("Loaded new variable from .env")
		} else {
			m.logger.Debug().Str("key", key).Msg("Updated existing variable from .env")
		}
		loadedCount++
	}

	if err := scanner.Err(); err != nil {
		m.logger.Warn().Err(err).Str("file", filePath).Msg("Error reading .env file")
	}

	m.logger.Debug().
		Str("file", filePath).
		Int("loaded", loadedCount).
		Int("skipped", skippedCount).
		Int("errors", errorCount).
		Msg("Finished loading variables from .env file")

	return nil
}
