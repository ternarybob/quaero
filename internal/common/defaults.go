// Package common provides shared utilities and default configuration.
package common

// DefaultKVValue represents a default key/value pair that is seeded on startup.
type DefaultKVValue struct {
	Key         string `json:"key"`
	Value       string `json:"value"`
	Description string `json:"description"`
}

// GetDefaultKVValues returns the list of default KV values seeded on startup.
// This is the single source of truth for default values.
func GetDefaultKVValues() []DefaultKVValue {
	return []DefaultKVValue{
		{
			Key:         "navexa_base_url",
			Value:       "https://api.navexa.com.au",
			Description: "Navexa API base URL",
		},
	}
}
