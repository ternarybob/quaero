package common

// KeysDirConfig contains configuration for key/value file loading.
// This is separate from AuthDirConfig to maintain clean separation between
// authentication (cookies) and generic key/value storage.
type KeysDirConfig struct {
	// Dir is the directory containing key/value files in TOML format
	// Each TOML file has [section-name] entries with 'value' and optional 'description' fields
	// Default: ./keys
	Dir string `toml:"dir"`
}
