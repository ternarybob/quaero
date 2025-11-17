package common

// KeysDirConfig contains configuration for variables (key/value pairs) file loading.
// This is separate from AuthDirConfig to maintain clean separation between
// authentication (cookies) and generic variables storage.
// Variables are user-defined key-value pairs (API keys, secrets, config values).
type KeysDirConfig struct {
	// Dir is the directory containing variables files in TOML format (./variables/*.toml)
	// Each TOML file has [[keys]] entries with 'key', 'value', and optional 'description' fields
	// Default storage location: ./variables/ directory
	// File format: Any *.toml file in the variables directory
	Dir string `toml:"dir"`
}
