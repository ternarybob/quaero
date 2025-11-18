package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ternarybob/arbor"
)

// createTestLogger creates a logger for testing
func createTestLogger() arbor.ILogger {
	return arbor.NewLogger()
}

// createTestKVMap returns a standard test KV map
func createTestKVMap() map[string]string {
	return map[string]string{
		"google-api-key": "sk-12345",
		"agent-key":      "sk-agent-789",
		"llm-key":        "sk-llm-111",
		"url1":           "http://example1.com",
		"url2":           "http://example2.com",
		"key1":           "val1",
		"key2":           "val2",
		"key3":           "val3",
		"action":         "stop_all",
		"database-url":   "postgres://localhost/db",
		"secret-token":   "token-abc-xyz",
	}
}

func TestReplaceKeyReferences_Simple(t *testing.T) {
	logger := createTestLogger()
	kvMap := map[string]string{"google-api-key": "sk-12345"}

	input := "api_key = {google-api-key}"
	expected := "api_key = sk-12345"

	result := ReplaceKeyReferences(input, kvMap, logger)
	assert.Equal(t, expected, result)
}

func TestReplaceKeyReferences_Multiple(t *testing.T) {
	logger := createTestLogger()
	kvMap := map[string]string{
		"key1": "val1",
		"key2": "val2",
		"key3": "val3",
	}

	input := "key1={key1}, key2={key2}, key3={key3}"
	expected := "key1=val1, key2=val2, key3=val3"

	result := ReplaceKeyReferences(input, kvMap, logger)
	assert.Equal(t, expected, result)
}

func TestReplaceKeyReferences_MissingKey(t *testing.T) {
	logger := createTestLogger()
	kvMap := map[string]string{"other-key": "value"}

	input := "api_key = {missing-key}"
	expected := "api_key = {missing-key}" // Unchanged

	result := ReplaceKeyReferences(input, kvMap, logger)
	assert.Equal(t, expected, result)
}

func TestReplaceKeyReferences_InvalidSyntax(t *testing.T) {
	logger := createTestLogger()
	kvMap := map[string]string{"invalid key": "value"}

	// Space in key name - doesn't match regex
	input := "api_key = {invalid key}"
	expected := "api_key = {invalid key}" // Unchanged

	result := ReplaceKeyReferences(input, kvMap, logger)
	assert.Equal(t, expected, result)
}

func TestReplaceKeyReferences_EmptyInput(t *testing.T) {
	logger := createTestLogger()
	kvMap := map[string]string{"key": "value"}

	input := ""
	expected := ""

	result := ReplaceKeyReferences(input, kvMap, logger)
	assert.Equal(t, expected, result)
}

func TestReplaceKeyReferences_NoReferences(t *testing.T) {
	logger := createTestLogger()
	kvMap := map[string]string{"key": "value"}

	input := "api_key = static-value"
	expected := "api_key = static-value"

	result := ReplaceKeyReferences(input, kvMap, logger)
	assert.Equal(t, expected, result)
}

func TestReplaceInMap_SimpleString(t *testing.T) {
	logger := createTestLogger()
	kvMap := map[string]string{"google-api-key": "sk-12345"}

	m := map[string]interface{}{
		"api_key": "{google-api-key}",
	}

	err := ReplaceInMap(m, kvMap, logger)
	require.NoError(t, err)

	assert.Equal(t, "sk-12345", m["api_key"])
}

func TestReplaceInMap_NestedMap(t *testing.T) {
	logger := createTestLogger()
	kvMap := map[string]string{"google-api-key": "sk-12345"}

	m := map[string]interface{}{
		"llm": map[string]interface{}{
			"api_key": "{google-api-key}",
		},
	}

	err := ReplaceInMap(m, kvMap, logger)
	require.NoError(t, err)

	llm := m["llm"].(map[string]interface{})
	assert.Equal(t, "sk-12345", llm["api_key"])
}

func TestReplaceInMap_MixedTypes(t *testing.T) {
	logger := createTestLogger()
	kvMap := map[string]string{
		"key1": "val1",
		"key2": "val2",
	}

	m := map[string]interface{}{
		"string": "{key1}",
		"int":    42,
		"bool":   true,
		"nested": map[string]interface{}{
			"key": "{key2}",
		},
	}

	err := ReplaceInMap(m, kvMap, logger)
	require.NoError(t, err)

	assert.Equal(t, "val1", m["string"])
	assert.Equal(t, 42, m["int"])
	assert.Equal(t, true, m["bool"])

	nested := m["nested"].(map[string]interface{})
	assert.Equal(t, "val2", nested["key"])
}

func TestReplaceInMap_ArrayOfStrings(t *testing.T) {
	logger := createTestLogger()
	kvMap := map[string]string{
		"url1": "http://example1.com",
		"url2": "http://example2.com",
	}

	m := map[string]interface{}{
		"urls": []interface{}{"{url1}", "{url2}", "static-url"},
	}

	err := ReplaceInMap(m, kvMap, logger)
	require.NoError(t, err)

	urls := m["urls"].([]interface{})
	assert.Equal(t, "http://example1.com", urls[0])
	assert.Equal(t, "http://example2.com", urls[1])
	assert.Equal(t, "static-url", urls[2])
}

func TestReplaceInMap_ArrayWithNestedMaps(t *testing.T) {
	logger := createTestLogger()
	kvMap := map[string]string{"key1": "val1"}

	m := map[string]interface{}{
		"items": []interface{}{
			map[string]interface{}{
				"field": "{key1}",
			},
		},
	}

	err := ReplaceInMap(m, kvMap, logger)
	require.NoError(t, err)

	items := m["items"].([]interface{})
	item := items[0].(map[string]interface{})
	assert.Equal(t, "val1", item["field"])
}

func TestReplaceInMap_EmptyMap(t *testing.T) {
	logger := createTestLogger()
	kvMap := map[string]string{"key": "value"}

	m := map[string]interface{}{}

	err := ReplaceInMap(m, kvMap, logger)
	require.NoError(t, err)

	assert.Empty(t, m)
}

func TestReplaceInStruct_SimpleFields(t *testing.T) {
	logger := createTestLogger()
	kvMap := map[string]string{"google-api-key": "sk-12345"}

	type LLMConfig struct {
		GoogleAPIKey string
	}

	type Config struct {
		LLM LLMConfig
	}

	config := &Config{
		LLM: LLMConfig{
			GoogleAPIKey: "{google-api-key}",
		},
	}

	err := ReplaceInStruct(config, kvMap, logger)
	require.NoError(t, err)

	assert.Equal(t, "sk-12345", config.LLM.GoogleAPIKey)
}

func TestReplaceInStruct_MultipleFields(t *testing.T) {
	logger := createTestLogger()
	kvMap := map[string]string{
		"llm-key":   "sk-111",
		"agent-key": "sk-222",
	}

	type LLMConfig struct {
		GoogleAPIKey string
	}

	type AgentConfig struct {
		GoogleAPIKey string
	}

	type Config struct {
		LLM   LLMConfig
		Agent AgentConfig
	}

	config := &Config{
		LLM: LLMConfig{
			GoogleAPIKey: "{llm-key}",
		},
		Agent: AgentConfig{
			GoogleAPIKey: "{agent-key}",
		},
	}

	err := ReplaceInStruct(config, kvMap, logger)
	require.NoError(t, err)

	assert.Equal(t, "sk-111", config.LLM.GoogleAPIKey)
	assert.Equal(t, "sk-222", config.Agent.GoogleAPIKey)
}

func TestReplaceInStruct_UnexportedFields(t *testing.T) {
	logger := createTestLogger()
	kvMap := map[string]string{"key": "value"}

	type TestStruct struct {
		Exported   string
		unexported string // Should be skipped
	}

	testStruct := &TestStruct{
		Exported:   "{key}",
		unexported: "{key}",
	}

	err := ReplaceInStruct(testStruct, kvMap, logger)
	require.NoError(t, err)

	assert.Equal(t, "value", testStruct.Exported)
	assert.Equal(t, "{key}", testStruct.unexported) // Unchanged
}

func TestReplaceInStruct_PointerFields(t *testing.T) {
	logger := createTestLogger()
	kvMap := map[string]string{"action": "stop_all"}

	type ErrorTolerance struct {
		FailureAction string
	}

	type Config struct {
		ErrorTolerance *ErrorTolerance
	}

	config := &Config{
		ErrorTolerance: &ErrorTolerance{
			FailureAction: "{action}",
		},
	}

	err := ReplaceInStruct(config, kvMap, logger)
	require.NoError(t, err)

	assert.Equal(t, "stop_all", config.ErrorTolerance.FailureAction)
}

func TestReplaceInStruct_NilPointer(t *testing.T) {
	logger := createTestLogger()
	kvMap := map[string]string{"action": "stop_all"}

	type ErrorTolerance struct {
		FailureAction string
	}

	type Config struct {
		Name           string
		ErrorTolerance *ErrorTolerance
	}

	config := &Config{
		Name:           "{action}",
		ErrorTolerance: nil, // Nil pointer should be handled gracefully
	}

	err := ReplaceInStruct(config, kvMap, logger)
	require.NoError(t, err)

	assert.Equal(t, "stop_all", config.Name)
	assert.Nil(t, config.ErrorTolerance)
}

func TestReplaceInStruct_MapField(t *testing.T) {
	logger := createTestLogger()
	kvMap := map[string]string{"key1": "val1"}

	type Config struct {
		Name    string
		Options map[string]interface{}
	}

	config := &Config{
		Name: "test",
		Options: map[string]interface{}{
			"field": "{key1}",
		},
	}

	err := ReplaceInStruct(config, kvMap, logger)
	require.NoError(t, err)

	assert.Equal(t, "val1", config.Options["field"])
}

func TestReplaceInStruct_NotPointer(t *testing.T) {
	logger := createTestLogger()
	kvMap := map[string]string{"key": "value"}

	type Config struct {
		Name string
	}

	config := Config{Name: "{key}"}

	// Should return error because not a pointer
	err := ReplaceInStruct(config, kvMap, logger)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires a pointer")
}

func TestReplaceInStruct_NotStruct(t *testing.T) {
	logger := createTestLogger()
	kvMap := map[string]string{"key": "value"}

	str := "test"

	// Should return error because not a struct pointer
	err := ReplaceInStruct(&str, kvMap, logger)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires a struct pointer")
}

func TestReplaceInStruct_DeepNesting(t *testing.T) {
	logger := createTestLogger()
	kvMap := map[string]string{
		"key1": "val1",
		"key2": "val2",
		"key3": "val3",
	}

	type Level3 struct {
		Field string
	}

	type Level2 struct {
		Field  string
		Nested Level3
	}

	type Level1 struct {
		Field  string
		Nested Level2
	}

	type Config struct {
		Field  string
		Nested Level1
	}

	config := &Config{
		Field: "{key1}",
		Nested: Level1{
			Field: "{key2}",
			Nested: Level2{
				Field: "{key3}",
				Nested: Level3{
					Field: "static",
				},
			},
		},
	}

	err := ReplaceInStruct(config, kvMap, logger)
	require.NoError(t, err)

	assert.Equal(t, "val1", config.Field)
	assert.Equal(t, "val2", config.Nested.Field)
	assert.Equal(t, "val3", config.Nested.Nested.Field)
	assert.Equal(t, "static", config.Nested.Nested.Nested.Field)
}

func TestReplaceKeyReferences_MultipleOccurrences(t *testing.T) {
	logger := createTestLogger()
	kvMap := map[string]string{"key": "value"}

	input := "{key} and {key} and {key}"
	expected := "value and value and value"

	result := ReplaceKeyReferences(input, kvMap, logger)
	assert.Equal(t, expected, result)
}

func TestReplaceKeyReferences_PartialMatch(t *testing.T) {
	logger := createTestLogger()
	kvMap := map[string]string{
		"key":     "value",
		"fullkey": "fullvalue",
	}

	input := "{key} and {fullkey}"
	expected := "value and fullvalue"

	result := ReplaceKeyReferences(input, kvMap, logger)
	assert.Equal(t, expected, result)
}

func TestReplaceKeyReferences_NumbersInKeyName(t *testing.T) {
	logger := createTestLogger()
	kvMap := map[string]string{
		"key123":  "value1",
		"123key":  "value2",
		"key-123": "value3",
		"key_123": "value4",
	}

	input := "{key123} {123key} {key-123} {key_123}"
	expected := "value1 value2 value3 value4"

	result := ReplaceKeyReferences(input, kvMap, logger)
	assert.Equal(t, expected, result)
}
