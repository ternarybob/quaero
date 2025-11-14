// -----------------------------------------------------------------------
// Last Modified: Thursday, 14th November 2025 1:00:00 pm
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

// Package common provides utility functions for key/value reference replacement.
//
// The {key-name} syntax allows configuration values to reference keys stored
// in the key/value store. At runtime, these references are replaced with actual
// values from the store.
//
// Example:
//   Input:  "api_key = {google-api-key}"
//   KV Map: {"google-api-key": "sk-12345"}
//   Output: "api_key = sk-12345"
//
// Replacement is case-sensitive. Missing keys are logged as warnings but not
// treated as errors, allowing graceful degradation.
package common

import (
	"fmt"
	"reflect"
	"regexp"

	"github.com/ternarybob/arbor"
)

// keyRefPattern matches {key-name} references in strings
// Allows alphanumeric characters, hyphens, and underscores
var keyRefPattern = regexp.MustCompile(`\{([a-zA-Z0-9_-]+)\}`)

// ReplaceKeyReferences replaces all {key-name} references in the input string
// with values from the provided KV map. If a key is not found, the reference
// is left unchanged and a warning is logged.
//
// Example:
//   ReplaceKeyReferences("api_key = {google-api-key}", map[string]string{"google-api-key": "sk-123"})
//   Returns: "api_key = sk-123"
func ReplaceKeyReferences(input string, kvMap map[string]string, logger arbor.ILogger) string {
	if input == "" {
		return input
	}

	// Log warnings for unresolved keys before replacement
	logUnresolvedKeys(input, kvMap, logger)

	// Replace all {key-name} references
	result := keyRefPattern.ReplaceAllStringFunc(input, func(match string) string {
		// Extract key name (remove braces)
		keyName := match[1 : len(match)-1]

		// Look up in KV map
		if value, exists := kvMap[keyName]; exists {
			return value
		}

		// Key not found - return unchanged
		return match
	})

	return result
}

// logUnresolvedKeys finds all {key-name} references and logs warnings for missing keys
func logUnresolvedKeys(input string, kvMap map[string]string, logger arbor.ILogger) {
	matches := keyRefPattern.FindAllStringSubmatch(input, -1)
	for _, match := range matches {
		if len(match) > 1 {
			keyName := match[1]
			if _, exists := kvMap[keyName]; !exists {
				logger.Warn().
					Str("reference", match[0]).
					Str("key", keyName).
					Msg("Unresolved key reference - key not found in KV store")
			}
		}
	}
}

// ReplaceInMap recursively replaces {key-name} references in a map structure.
// It handles string values, nested maps, and arrays of strings or maps.
// The map is mutated in-place.
func ReplaceInMap(m map[string]interface{}, kvMap map[string]string, logger arbor.ILogger) error {
	for key, value := range m {
		switch v := value.(type) {
		case string:
			// Replace string value
			oldValue := v
			newValue := ReplaceKeyReferences(v, kvMap, logger)
			if oldValue != newValue {
				m[key] = newValue
				logger.Debug().
					Str("key", key).
					Str("old", oldValue).
					Str("new", newValue).
					Msg("Replaced key reference in map")
			}

		case map[string]interface{}:
			// Recursive call for nested map
			if err := ReplaceInMap(v, kvMap, logger); err != nil {
				return fmt.Errorf("failed to replace in nested map at key '%s': %w", key, err)
			}

		case []interface{}:
			// Handle array elements
			for i, elem := range v {
				switch e := elem.(type) {
				case string:
					oldValue := e
					newValue := ReplaceKeyReferences(e, kvMap, logger)
					if oldValue != newValue {
						v[i] = newValue
						logger.Debug().
							Str("key", key).
							Int("index", i).
							Str("old", oldValue).
							Str("new", newValue).
							Msg("Replaced key reference in array")
					}

				case map[string]interface{}:
					// Recursive call for map in array
					if err := ReplaceInMap(e, kvMap, logger); err != nil {
						return fmt.Errorf("failed to replace in array element at key '%s'[%d]: %w", key, i, err)
					}
				}
			}

		// Other types (int, bool, float, etc.) - skip, no replacement needed
		}
	}

	return nil
}

// ReplaceInStruct uses reflection to recursively replace {key-name} references
// in a struct's string fields. It handles nested structs, maps, and pointer fields.
// The struct must be passed as a pointer for in-place mutation.
func ReplaceInStruct(v interface{}, kvMap map[string]string, logger arbor.ILogger) error {
	// Get the reflect value
	val := reflect.ValueOf(v)

	// Must be a pointer
	if val.Kind() != reflect.Ptr {
		return fmt.Errorf("ReplaceInStruct requires a pointer, got %T", v)
	}

	// Get the element the pointer points to
	val = val.Elem()

	// Must be a struct
	if val.Kind() != reflect.Struct {
		return fmt.Errorf("ReplaceInStruct requires a struct pointer, got pointer to %v", val.Kind())
	}

	return replaceInStructValue(val, kvMap, logger)
}

// replaceInStructValue is the recursive implementation for struct traversal
func replaceInStructValue(val reflect.Value, kvMap map[string]string, logger arbor.ILogger) error {
	typ := val.Type()

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := typ.Field(i)

		// Skip unexported fields
		if !field.CanSet() {
			continue
		}

		switch field.Kind() {
		case reflect.String:
			// Replace string field
			oldValue := field.String()
			newValue := ReplaceKeyReferences(oldValue, kvMap, logger)
			if oldValue != newValue {
				field.SetString(newValue)
				logger.Debug().
					Str("field", fieldType.Name).
					Str("old", oldValue).
					Str("new", newValue).
					Msg("Replaced key reference in struct field")
			}

		case reflect.Struct:
			// Recursive call for nested struct
			if err := replaceInStructValue(field, kvMap, logger); err != nil {
				return fmt.Errorf("failed to replace in nested struct field '%s': %w", fieldType.Name, err)
			}

		case reflect.Ptr:
			// Handle pointer fields
			if !field.IsNil() {
				elem := field.Elem()
				if elem.Kind() == reflect.Struct {
					if err := replaceInStructValue(elem, kvMap, logger); err != nil {
						return fmt.Errorf("failed to replace in pointer field '%s': %w", fieldType.Name, err)
					}
				}
			}

		case reflect.Map:
			// Handle map fields
			if field.Type().Key().Kind() == reflect.String {
				elemKind := field.Type().Elem().Kind()

				if elemKind == reflect.Interface {
					// This is a map[string]interface{} - convert and replace
					mapVal := field.Interface().(map[string]interface{})
					if err := ReplaceInMap(mapVal, kvMap, logger); err != nil {
						return fmt.Errorf("failed to replace in map field '%s': %w", fieldType.Name, err)
					}
				} else if elemKind == reflect.String {
					// This is a map[string]string - iterate and replace string values
					mapVal := field.Interface().(map[string]string)
					for key, value := range mapVal {
						oldValue := value
						newValue := ReplaceKeyReferences(value, kvMap, logger)
						if oldValue != newValue {
							mapVal[key] = newValue
							logger.Debug().
								Str("field", fieldType.Name).
								Str("key", key).
								Str("old", oldValue).
								Str("new", newValue).
								Msg("Replaced key reference in map[string]string field")
						}
					}
				}
			}

		case reflect.Slice:
			// Handle slice of strings (e.g., PreJobs, PostJobs, Tags)
			if field.Type().Elem().Kind() == reflect.String {
				for i := 0; i < field.Len(); i++ {
					elem := field.Index(i)
					oldValue := elem.String()
					newValue := ReplaceKeyReferences(oldValue, kvMap, logger)
					if oldValue != newValue {
						elem.SetString(newValue)
						logger.Debug().
							Str("field", fieldType.Name).
							Int("index", i).
							Str("old", oldValue).
							Str("new", newValue).
							Msg("Replaced key reference in slice field")
					}
				}
			}
		}
	}

	return nil
}
