package api

import (
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/ternarybob/quaero/test"
)

func TestConfigEndpoint(t *testing.T) {
	h := test.NewHTTPTestHelper(t, test.MustGetTestServerURL())

	resp, err := h.GET("/api/config")
	if err != nil {
		t.Fatalf("Failed to get config: %v", err)
	}

	// Check status code
	h.AssertStatusCode(resp, http.StatusOK)

	var result map[string]interface{}
	if err := h.ParseJSONResponse(resp, &result); err != nil {
		t.Fatalf("Failed to parse config response: %v", err)
	}

	// Verify version field exists
	version, ok := result["version"].(string)
	if !ok {
		t.Error("Config response missing 'version' field")
	}

	// Read expected version from .version file (if available)
	expectedVersion, _ := readVersionFile()
	if expectedVersion != "" && version != expectedVersion {
		t.Errorf("Expected version '%s', got '%s'", expectedVersion, version)
	}
	// If we can't read .version file, just verify version is not empty
	if version == "" {
		t.Error("Version should not be empty")
	}

	// Verify build field exists
	build, ok := result["build"].(string)
	if !ok {
		t.Error("Config response missing 'build' field")
	}
	t.Logf("Build: %s", build)

	// Verify port field exists
	port, ok := result["port"].(float64) // JSON numbers are float64
	if !ok {
		t.Error("Config response missing 'port' field")
	}

	// Port should match the service port (8085 by default, or from config)
	expectedPort := test.GetExpectedPort()
	if int(port) != expectedPort {
		t.Errorf("Expected port %d, got %d", expectedPort, int(port))
	}

	// Verify config field exists and is an object
	config, ok := result["config"].(map[string]interface{})
	if !ok {
		t.Error("Config response missing 'config' field or not an object")
	}

	// Verify server config exists
	server, ok := config["Server"].(map[string]interface{})
	if !ok {
		t.Error("Config missing 'Server' field")
	}

	// Verify server port matches
	serverPort, ok := server["Port"].(float64)
	if !ok {
		t.Error("Server config missing 'Port' field")
	}
	if int(serverPort) != expectedPort {
		t.Errorf("Server config port %d doesn't match expected port %d", int(serverPort), expectedPort)
	}

	t.Logf("✓ Config endpoint returned correct version (%s) and port (%d)", version, int(port))
}

// readVersionFile reads version from .version file
func readVersionFile() (string, string) {
	// Try multiple paths since tests run from different directories
	paths := []string{
		".version",
		"../../.version",
		"../../../.version",
	}

	var data []byte
	var err error
	for _, path := range paths {
		data, err = os.ReadFile(path)
		if err == nil {
			break
		}
	}

	if err != nil {
		// Can't find .version file, just return empty strings
		// Don't fail the test - version validation is not critical
		return "", ""
	}

	lines := strings.Split(string(data), "\n")
	version := "unknown"
	build := "unknown"

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "version:") {
			version = strings.TrimSpace(strings.TrimPrefix(line, "version:"))
		} else if strings.HasPrefix(line, "build:") {
			build = strings.TrimSpace(strings.TrimPrefix(line, "build:"))
		}
	}

	return version, build
}
