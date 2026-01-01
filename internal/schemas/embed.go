package schemas

import (
	"embed"
)

//go:embed *.json
var fs embed.FS

// GetSchema returns the content of a schema file by name
func GetSchema(name string) ([]byte, error) {
	return fs.ReadFile(name)
}
