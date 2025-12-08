package pkg

import (
	"fmt"
	"strings"
	"time"
)

// ProcessData processes input data and returns a formatted result
func ProcessData(input string) string {
	cleaned := strings.TrimSpace(input)
	timestamp := time.Now().Format(time.RFC3339)

	return fmt.Sprintf("Processed: %s at %s", cleaned, timestamp)
}

// ValidateInput checks if the input meets basic requirements
func ValidateInput(input string) bool {
	if len(input) == 0 {
		return false
	}
	if len(input) > 1000 {
		return false
	}
	return true
}

// FormatOutput formats data for display
func FormatOutput(data map[string]interface{}) string {
	var builder strings.Builder

	builder.WriteString("Output:\n")
	for key, value := range data {
		builder.WriteString(fmt.Sprintf("  %s: %v\n", key, value))
	}

	return builder.String()
}

// CalculateMetrics computes basic metrics from data
func CalculateMetrics(values []float64) map[string]float64 {
	if len(values) == 0 {
		return map[string]float64{
			"count": 0,
			"sum":   0,
			"avg":   0,
		}
	}

	sum := 0.0
	for _, v := range values {
		sum += v
	}

	return map[string]float64{
		"count": float64(len(values)),
		"sum":   sum,
		"avg":   sum / float64(len(values)),
	}
}
