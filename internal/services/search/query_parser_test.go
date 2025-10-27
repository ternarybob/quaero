package search

import (
	"reflect"
	"testing"
)

func TestQueryParser_Tokenize(t *testing.T) {
	parser := NewQueryParser()

	tests := []struct {
		name     string
		query    string
		expected []Token
	}{
		{
			name:  "Simple terms",
			query: "cat dog",
			expected: []Token{
				{Value: "cat", Type: TokenTypeTerm, Required: false},
				{Value: "dog", Type: TokenTypeTerm, Required: false},
			},
		},
		{
			name:  "Required terms",
			query: "+cat +dog",
			expected: []Token{
				{Value: "cat", Type: TokenTypeTerm, Required: true},
				{Value: "dog", Type: TokenTypeTerm, Required: true},
			},
		},
		{
			name:  "Mixed required and optional",
			query: "+cat dog",
			expected: []Token{
				{Value: "cat", Type: TokenTypeTerm, Required: true},
				{Value: "dog", Type: TokenTypeTerm, Required: false},
			},
		},
		{
			name:  "Quoted phrase",
			query: `"cat on mat"`,
			expected: []Token{
				{Value: "cat on mat", Type: TokenTypePhrase, Required: false},
			},
		},
		{
			name:  "Required quoted phrase",
			query: `+"cat on mat"`,
			expected: []Token{
				{Value: "cat on mat", Type: TokenTypePhrase, Required: true},
			},
		},
		{
			name:  "Qualifier",
			query: "document_type:jira",
			expected: []Token{
				{Value: "document_type:jira", Type: TokenTypeQualifier, Required: false},
			},
		},
		{
			name:  "Qualifier with search term",
			query: "document_type:jira cat",
			expected: []Token{
				{Value: "document_type:jira", Type: TokenTypeQualifier, Required: false},
				{Value: "cat", Type: TokenTypeTerm, Required: false},
			},
		},
		{
			name:  "Multiple qualifiers",
			query: "document_type:jira case:match cat",
			expected: []Token{
				{Value: "document_type:jira", Type: TokenTypeQualifier, Required: false},
				{Value: "case:match", Type: TokenTypeQualifier, Required: false},
				{Value: "cat", Type: TokenTypeTerm, Required: false},
			},
		},
		{
			name:     "Empty query",
			query:    "",
			expected: []Token{},
		},
		{
			name:     "Only whitespace",
			query:    "   ",
			expected: []Token{},
		},
		{
			name:  "Unbalanced quotes",
			query: `"cat dog`,
			expected: []Token{
				{Value: "cat dog", Type: TokenTypePhrase, Required: false},
			},
		},
		{
			name:  "Escaped quote in phrase",
			query: `"cat \"on\" mat"`,
			expected: []Token{
				{Value: `cat "on" mat`, Type: TokenTypePhrase, Required: false},
			},
		},
		{
			name:     "Plus sign alone",
			query:    "+",
			expected: []Token{},
		},
		{
			name:  "Multiple spaces",
			query: "cat    dog",
			expected: []Token{
				{Value: "cat", Type: TokenTypeTerm, Required: false},
				{Value: "dog", Type: TokenTypeTerm, Required: false},
			},
		},
		{
			name:  "Complex query",
			query: `+cat "on the mat" dog document_type:jira`,
			expected: []Token{
				{Value: "cat", Type: TokenTypeTerm, Required: true},
				{Value: "on the mat", Type: TokenTypePhrase, Required: false},
				{Value: "dog", Type: TokenTypeTerm, Required: false},
				{Value: "document_type:jira", Type: TokenTypeQualifier, Required: false},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := parser.Tokenize(tt.query)

			if len(tokens) != len(tt.expected) {
				t.Errorf("Expected %d tokens, got %d", len(tt.expected), len(tokens))
				t.Logf("Expected: %+v", tt.expected)
				t.Logf("Got: %+v", tokens)
				return
			}

			for i, expected := range tt.expected {
				if !reflect.DeepEqual(tokens[i], expected) {
					t.Errorf("Token %d mismatch:\nExpected: %+v\nGot: %+v", i, expected, tokens[i])
				}
			}
		})
	}
}

func TestQueryParser_IsQualifier(t *testing.T) {
	parser := NewQueryParser()

	tests := []struct {
		name     string
		token    string
		expected bool
	}{
		{
			name:     "Valid qualifier",
			token:    "document_type:jira",
			expected: true,
		},
		{
			name:     "Valid qualifier with underscore",
			token:    "case_sensitive:match",
			expected: true,
		},
		{
			name:     "Invalid - no colon",
			token:    "cat",
			expected: false,
		},
		{
			name:     "Invalid - colon at start",
			token:    ":jira",
			expected: false,
		},
		{
			name:     "Invalid - colon at end",
			token:    "document_type:",
			expected: false,
		},
		{
			name:     "Invalid - multiple colons",
			token:    "a:b:c",
			expected: false,
		},
		{
			name:     "Invalid - special characters in key",
			token:    "doc-type:jira",
			expected: false,
		},
		{
			name:     "Valid - numbers in key",
			token:    "type2:value",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.IsQualifier(tt.token)

			if result != tt.expected {
				t.Errorf("Expected IsQualifier(%q) = %v, got %v", tt.token, tt.expected, result)
			}
		})
	}
}

func TestQueryParser_SplitQualifier(t *testing.T) {
	parser := NewQueryParser()

	tests := []struct {
		name          string
		qualifier     string
		expectedKey   string
		expectedValue string
	}{
		{
			name:          "Simple qualifier",
			qualifier:     "document_type:jira",
			expectedKey:   "document_type",
			expectedValue: "jira",
		},
		{
			name:          "Qualifier with complex value",
			qualifier:     "status:in_progress",
			expectedKey:   "status",
			expectedValue: "in_progress",
		},
		{
			name:          "Invalid qualifier",
			qualifier:     "invalid",
			expectedKey:   "",
			expectedValue: "",
		},
		{
			name:          "Empty key",
			qualifier:     ":value",
			expectedKey:   "",
			expectedValue: "value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, value := parser.SplitQualifier(tt.qualifier)

			if key != tt.expectedKey {
				t.Errorf("Expected key %q, got %q", tt.expectedKey, key)
			}

			if value != tt.expectedValue {
				t.Errorf("Expected value %q, got %q", tt.expectedValue, value)
			}
		})
	}
}

func TestQueryParser_EscapeFTS5(t *testing.T) {
	parser := NewQueryParser()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "No special characters",
			input:    "cat",
			expected: "cat",
		},
		{
			name:     "Double quotes",
			input:    `cat "on" mat`,
			expected: `cat ""on"" mat`,
		},
		{
			name:     "Multiple double quotes",
			input:    `"cat" "dog"`,
			expected: `""cat"" ""dog""`,
		},
		{
			name:     "Hyphen (allowed)",
			input:    "cat-dog",
			expected: "cat-dog",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.EscapeFTS5(tt.input)

			if result != tt.expected {
				t.Errorf("Expected EscapeFTS5(%q) = %q, got %q", tt.input, tt.expected, result)
			}
		})
	}
}

func TestQueryParser_BuildFTS5Query(t *testing.T) {
	parser := NewQueryParser()

	tests := []struct {
		name     string
		tokens   []Token
		expected string
	}{
		{
			name: "Simple OR",
			tokens: []Token{
				{Value: "cat", Type: TokenTypeTerm, Required: false},
				{Value: "dog", Type: TokenTypeTerm, Required: false},
			},
			expected: "cat OR dog",
		},
		{
			name: "Simple AND",
			tokens: []Token{
				{Value: "cat", Type: TokenTypeTerm, Required: true},
				{Value: "dog", Type: TokenTypeTerm, Required: true},
			},
			expected: "cat AND dog",
		},
		{
			name: "Mixed AND/OR",
			tokens: []Token{
				{Value: "cat", Type: TokenTypeTerm, Required: true},
				{Value: "dog", Type: TokenTypeTerm, Required: false},
				{Value: "mat", Type: TokenTypeTerm, Required: false},
			},
			expected: "cat AND (dog OR mat)",
		},
		{
			name: "Quoted phrase",
			tokens: []Token{
				{Value: "cat on mat", Type: TokenTypePhrase, Required: false},
			},
			expected: `"cat on mat"`,
		},
		{
			name: "Required phrase",
			tokens: []Token{
				{Value: "cat on mat", Type: TokenTypePhrase, Required: true},
			},
			expected: `"cat on mat"`,
		},
		{
			name: "Qualifiers only",
			tokens: []Token{
				{Value: "document_type:jira", Type: TokenTypeQualifier, Required: false},
			},
			expected: "",
		},
		{
			name:     "Empty tokens",
			tokens:   []Token{},
			expected: "",
		},
		{
			name: "Complex query",
			tokens: []Token{
				{Value: "cat", Type: TokenTypeTerm, Required: true},
				{Value: "on the mat", Type: TokenTypePhrase, Required: false},
				{Value: "dog", Type: TokenTypeTerm, Required: false},
			},
			expected: `cat AND ("on the mat" OR dog)`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.BuildFTS5Query(tt.tokens)

			if result != tt.expected {
				t.Errorf("Expected BuildFTS5Query = %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestQueryParser_ExtractQualifiers(t *testing.T) {
	parser := NewQueryParser()

	tests := []struct {
		name     string
		tokens   []Token
		expected map[string]string
	}{
		{
			name: "Single qualifier",
			tokens: []Token{
				{Value: "document_type:jira", Type: TokenTypeQualifier, Required: false},
				{Value: "cat", Type: TokenTypeTerm, Required: false},
			},
			expected: map[string]string{
				"document_type": "jira",
			},
		},
		{
			name: "Multiple qualifiers",
			tokens: []Token{
				{Value: "document_type:jira", Type: TokenTypeQualifier, Required: false},
				{Value: "case:match", Type: TokenTypeQualifier, Required: false},
			},
			expected: map[string]string{
				"document_type": "jira",
				"case":          "match",
			},
		},
		{
			name: "Qualifier aliases",
			tokens: []Token{
				{Value: "type:confluence", Type: TokenTypeQualifier, Required: false},
			},
			expected: map[string]string{
				"document_type": "confluence",
			},
		},
		{
			name: "No qualifiers",
			tokens: []Token{
				{Value: "cat", Type: TokenTypeTerm, Required: false},
			},
			expected: map[string]string{},
		},
		{
			name:     "Empty tokens",
			tokens:   []Token{},
			expected: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.ExtractQualifiers(tt.tokens)

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("Expected qualifiers %+v, got %+v", tt.expected, result)
			}
		})
	}
}

func TestQueryParser_SpecialCharacters(t *testing.T) {
	parser := NewQueryParser()

	t.Run("Terms with FTS5 special characters", func(t *testing.T) {
		// Test terms containing characters that may have special meaning in FTS5
		tokens := parser.Tokenize(`cat* dog^`)

		if len(tokens) != 2 {
			t.Errorf("Expected 2 tokens, got %d", len(tokens))
		}

		// Ensure special characters are preserved in tokens
		if len(tokens) >= 2 {
			if tokens[0].Value != "cat*" {
				t.Errorf("Expected 'cat*', got %q", tokens[0].Value)
			}
			if tokens[1].Value != "dog^" {
				t.Errorf("Expected 'dog^', got %q", tokens[1].Value)
			}
		}
	})

	t.Run("Escaped quotes inside phrase", func(t *testing.T) {
		// Test phrase with escaped quotes: "cat \"on\" mat"
		tokens := parser.Tokenize(`"cat \"on\" mat"`)

		if len(tokens) != 1 {
			t.Errorf("Expected 1 token, got %d", len(tokens))
		}

		if len(tokens) > 0 {
			if tokens[0].Type != TokenTypePhrase {
				t.Errorf("Expected TokenTypePhrase, got %v", tokens[0].Type)
			}
			// The escaped quotes should be unescaped in the token value
			if tokens[0].Value != `cat "on" mat` {
				t.Errorf("Expected 'cat \"on\" mat', got %q", tokens[0].Value)
			}
		}
	})

	t.Run("BuildFTS5 with special characters", func(t *testing.T) {
		// Ensure EscapeFTS5 is applied to terms with quotes AND auto-quoting wraps the term
		tokens := []Token{
			{Value: `cat "quoted"`, Type: TokenTypeTerm, Required: false},
		}

		fts5Query := parser.BuildFTS5Query(tokens)

		// Quotes should be doubled for FTS5 escaping, and term should be auto-quoted due to containing quotes
		expected := `"cat ""quoted"""`
		if fts5Query != expected {
			t.Errorf("Expected %q, got %q", expected, fts5Query)
		}
	})

	t.Run("Multiple escaped quotes in phrase", func(t *testing.T) {
		tokens := parser.Tokenize(`"say \"hello\" and \"goodbye\""`)

		if len(tokens) != 1 {
			t.Errorf("Expected 1 token, got %d", len(tokens))
		}

		if len(tokens) > 0 {
			expected := `say "hello" and "goodbye"`
			if tokens[0].Value != expected {
				t.Errorf("Expected %q, got %q", expected, tokens[0].Value)
			}
		}
	})

	t.Run("Phrase with FTS5 operators", func(t *testing.T) {
		// Test phrase containing FTS5 operators like AND, OR
		tokens := parser.Tokenize(`"cat AND dog OR mat"`)

		if len(tokens) != 1 {
			t.Errorf("Expected 1 token, got %d", len(tokens))
		}

		if len(tokens) > 0 {
			if tokens[0].Type != TokenTypePhrase {
				t.Errorf("Expected TokenTypePhrase, got %v", tokens[0].Type)
			}
			// FTS5 operators inside quotes should be treated as literals
			if tokens[0].Value != "cat AND dog OR mat" {
				t.Errorf("Expected 'cat AND dog OR mat', got %q", tokens[0].Value)
			}
		}
	})
}

func TestQueryParser_UnicodeSupport(t *testing.T) {
	parser := NewQueryParser()

	t.Run("Russian characters", func(t *testing.T) {
		tokens := parser.Tokenize("собака кошка")

		if len(tokens) != 2 {
			t.Errorf("Expected 2 tokens, got %d", len(tokens))
		}

		if len(tokens) >= 2 {
			if tokens[0].Value != "собака" {
				t.Errorf("Expected 'собака', got %q", tokens[0].Value)
			}
			if tokens[1].Value != "кошка" {
				t.Errorf("Expected 'кошка', got %q", tokens[1].Value)
			}
		}
	})

	t.Run("Chinese characters", func(t *testing.T) {
		tokens := parser.Tokenize("猫 狗")

		if len(tokens) != 2 {
			t.Errorf("Expected 2 tokens, got %d", len(tokens))
		}

		if len(tokens) >= 2 {
			if tokens[0].Value != "猫" {
				t.Errorf("Expected '猫', got %q", tokens[0].Value)
			}
			if tokens[1].Value != "狗" {
				t.Errorf("Expected '狗', got %q", tokens[1].Value)
			}
		}
	})

	t.Run("Emoji", func(t *testing.T) {
		tokens := parser.Tokenize("🐱 🐶")

		if len(tokens) != 2 {
			t.Errorf("Expected 2 tokens, got %d", len(tokens))
		}

		if len(tokens) >= 2 {
			if tokens[0].Value != "🐱" {
				t.Errorf("Expected '🐱', got %q", tokens[0].Value)
			}
			if tokens[1].Value != "🐶" {
				t.Errorf("Expected '🐶', got %q", tokens[1].Value)
			}
		}
	})

	t.Run("Unicode in quotes", func(t *testing.T) {
		tokens := parser.Tokenize(`"собака на коврике"`)

		if len(tokens) != 1 {
			t.Errorf("Expected 1 token, got %d", len(tokens))
		}

		if len(tokens) > 0 {
			if tokens[0].Type != TokenTypePhrase {
				t.Errorf("Expected TokenTypePhrase, got %v", tokens[0].Type)
			}
			if tokens[0].Value != "собака на коврике" {
				t.Errorf("Expected 'собака на коврике', got %q", tokens[0].Value)
			}
		}
	})

	t.Run("Required Unicode term", func(t *testing.T) {
		tokens := parser.Tokenize("+猫 狗")

		if len(tokens) != 2 {
			t.Errorf("Expected 2 tokens, got %d", len(tokens))
		}

		if len(tokens) >= 1 {
			if tokens[0].Value != "猫" {
				t.Errorf("Expected '猫', got %q", tokens[0].Value)
			}
			if !tokens[0].Required {
				t.Error("Expected first token to be required")
			}
		}

		if len(tokens) >= 2 {
			if tokens[1].Value != "狗" {
				t.Errorf("Expected '狗', got %q", tokens[1].Value)
			}
			if tokens[1].Required {
				t.Error("Expected second token to not be required")
			}
		}
	})

	t.Run("Escaped quotes with Unicode", func(t *testing.T) {
		tokens := parser.Tokenize(`"кошка \"сказала\" мяу"`)

		if len(tokens) != 1 {
			t.Errorf("Expected 1 token, got %d", len(tokens))
		}

		if len(tokens) > 0 {
			expected := `кошка "сказала" мяу`
			if tokens[0].Value != expected {
				t.Errorf("Expected %q, got %q", expected, tokens[0].Value)
			}
		}
	})

	t.Run("Mixed ASCII and Unicode", func(t *testing.T) {
		tokens := parser.Tokenize("cat 猫 dog собака")

		if len(tokens) != 4 {
			t.Errorf("Expected 4 tokens, got %d", len(tokens))
		}

		expected := []string{"cat", "猫", "dog", "собака"}
		for i, exp := range expected {
			if i < len(tokens) {
				if tokens[i].Value != exp {
					t.Errorf("Token %d: expected %q, got %q", i, exp, tokens[i].Value)
				}
			}
		}
	})
}

func TestQueryParser_EdgeCases(t *testing.T) {
	parser := NewQueryParser()

	t.Run("Consecutive plus signs", func(t *testing.T) {
		tokens := parser.Tokenize("++cat")

		// Should treat ++ as a single + prefix
		if len(tokens) != 1 {
			t.Errorf("Expected 1 token, got %d", len(tokens))
		}

		if len(tokens) > 0 && tokens[0].Value != "cat" {
			t.Errorf("Expected 'cat', got %q", tokens[0].Value)
		}
	})

	t.Run("Plus sign in middle of term", func(t *testing.T) {
		tokens := parser.Tokenize("cat+dog")

		// Should treat as single term
		if len(tokens) != 1 {
			t.Errorf("Expected 1 token, got %d", len(tokens))
		}

		if len(tokens) > 0 && tokens[0].Value != "cat+dog" {
			t.Errorf("Expected 'cat+dog', got %q", tokens[0].Value)
		}
	})

	t.Run("Quote in middle of term", func(t *testing.T) {
		tokens := parser.Tokenize(`cat"dog`)

		// Should start a phrase at the quote
		if len(tokens) < 1 {
			t.Fatal("Expected at least 1 token")
		}
	})

	t.Run("Empty phrase", func(t *testing.T) {
		tokens := parser.Tokenize(`""`)

		// Empty phrases should be handled gracefully
		if len(tokens) > 1 {
			t.Errorf("Expected 0 or 1 token, got %d", len(tokens))
		}
	})

	t.Run("Qualifier with empty value", func(t *testing.T) {
		tokens := parser.Tokenize("document_type:")

		// Should not be recognized as qualifier (no value)
		if len(tokens) > 0 && tokens[0].Type == TokenTypeQualifier {
			t.Error("Empty qualifier value should not be recognized")
		}
	})

	t.Run("Very long query", func(t *testing.T) {
		longQuery := ""
		for i := 0; i < 100; i++ {
			longQuery += "term" + string(rune('0'+i%10)) + " "
		}

		tokens := parser.Tokenize(longQuery)

		if len(tokens) != 100 {
			t.Errorf("Expected 100 tokens, got %d", len(tokens))
		}
	})
}

func TestQueryParser_AutoQuoting(t *testing.T) {
	parser := NewQueryParser()

	t.Run("needsQuoting helper", func(t *testing.T) {
		tests := []struct {
			term     string
			expected bool
		}{
			{"cat", false},
			{"cat-dog", true},  // hyphen
			{"cat dog", true},  // space
			{"cat:dog", true},  // colon
			{"cat(dog)", true}, // parentheses
			{"cat*", true},     // asterisk
			{"cat^", true},     // caret
			{"cat+dog", true},  // plus
			{"simple", false},  // no special chars
			{"猫", false},       // Unicode, no special chars
			{"test-123", true}, // hyphen
		}

		for _, tt := range tests {
			result := parser.needsQuoting(tt.term)
			if result != tt.expected {
				t.Errorf("needsQuoting(%q) = %v, expected %v", tt.term, result, tt.expected)
			}
		}
	})

	t.Run("BuildFTS5Query auto-quotes terms with special characters", func(t *testing.T) {
		tests := []struct {
			name     string
			tokens   []Token
			expected string
		}{
			{
				name: "Term with hyphen",
				tokens: []Token{
					{Value: "cat-dog", Type: TokenTypeTerm, Required: false},
				},
				expected: `"cat-dog"`,
			},
			{
				name: "Term with space (should be phrase, but testing auto-quoting)",
				tokens: []Token{
					{Value: "cat dog", Type: TokenTypeTerm, Required: false},
				},
				expected: `"cat dog"`,
			},
			{
				name: "Term with colon",
				tokens: []Token{
					{Value: "test:value", Type: TokenTypeTerm, Required: false},
				},
				expected: `"test:value"`,
			},
			{
				name: "Multiple terms with special characters",
				tokens: []Token{
					{Value: "cat-dog", Type: TokenTypeTerm, Required: false},
					{Value: "test*", Type: TokenTypeTerm, Required: false},
				},
				expected: `"cat-dog" OR "test*"`,
			},
			{
				name: "Mixed: simple term and term with special character",
				tokens: []Token{
					{Value: "cat", Type: TokenTypeTerm, Required: false},
					{Value: "test-123", Type: TokenTypeTerm, Required: false},
				},
				expected: `cat OR "test-123"`,
			},
			{
				name: "Required term with hyphen",
				tokens: []Token{
					{Value: "cat-dog", Type: TokenTypeTerm, Required: true},
				},
				expected: `"cat-dog"`,
			},
			{
				name: "Mixed required and optional with special characters",
				tokens: []Token{
					{Value: "cat-dog", Type: TokenTypeTerm, Required: true},
					{Value: "test*", Type: TokenTypeTerm, Required: false},
				},
				expected: `"cat-dog" AND ("test*")`,
			},
			{
				name: "Term with parentheses",
				tokens: []Token{
					{Value: "test(123)", Type: TokenTypeTerm, Required: false},
				},
				expected: `"test(123)"`,
			},
			{
				name: "Already-quoted phrase not affected",
				tokens: []Token{
					{Value: "cat on mat", Type: TokenTypePhrase, Required: false},
				},
				expected: `"cat on mat"`,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := parser.BuildFTS5Query(tt.tokens)
				if result != tt.expected {
					t.Errorf("Expected %q, got %q", tt.expected, result)
				}
			})
		}
	})

	t.Run("Auto-quoting with escaping", func(t *testing.T) {
		// Terms with both special characters and quotes should be escaped AND quoted
		tokens := []Token{
			{Value: `cat-"dog"`, Type: TokenTypeTerm, Required: false},
		}

		result := parser.BuildFTS5Query(tokens)
		// Should escape quotes (doubling) AND auto-quote due to hyphen
		expected := `"cat-""dog"""`
		if result != expected {
			t.Errorf("Expected %q, got %q", expected, result)
		}
	})
}

func TestQueryParser_ReservedKeywords(t *testing.T) {
	parser := NewQueryParser()

	t.Run("isReservedWord helper", func(t *testing.T) {
		tests := []struct {
			term     string
			expected bool
		}{
			{"AND", true},
			{"OR", true},
			{"NOT", true},
			{"NEAR", true},
			{"and", true},   // case-insensitive
			{"or", true},    // case-insensitive
			{"Not", true},   // case-insensitive
			{"near", true},  // case-insensitive
			{"cat", false},  // not a reserved word
			{"dog", false},  // not a reserved word
			{"ANDY", false}, // similar but not reserved
			{"ORAL", false}, // similar but not reserved
		}

		for _, tt := range tests {
			result := parser.isReservedWord(tt.term)
			if result != tt.expected {
				t.Errorf("isReservedWord(%q) = %v, expected %v", tt.term, result, tt.expected)
			}
		}
	})

	t.Run("needsQuoting detects reserved words", func(t *testing.T) {
		tests := []struct {
			term     string
			expected bool
		}{
			{"AND", true},
			{"OR", true},
			{"NOT", true},
			{"NEAR", true},
			{"and", true},
			{"or", true},
			{"not", true},
			{"near", true},
			{"cat", false},
			{"dog", false},
		}

		for _, tt := range tests {
			result := parser.needsQuoting(tt.term)
			if result != tt.expected {
				t.Errorf("needsQuoting(%q) = %v, expected %v", tt.term, result, tt.expected)
			}
		}
	})

	t.Run("BuildFTS5Query quotes reserved keywords", func(t *testing.T) {
		tests := []struct {
			name     string
			tokens   []Token
			expected string
		}{
			{
				name: "Single AND term",
				tokens: []Token{
					{Value: "AND", Type: TokenTypeTerm, Required: false},
				},
				expected: `"AND"`,
			},
			{
				name: "Single OR term",
				tokens: []Token{
					{Value: "OR", Type: TokenTypeTerm, Required: false},
				},
				expected: `"OR"`,
			},
			{
				name: "Single NOT term",
				tokens: []Token{
					{Value: "NOT", Type: TokenTypeTerm, Required: false},
				},
				expected: `"NOT"`,
			},
			{
				name: "Single NEAR term",
				tokens: []Token{
					{Value: "NEAR", Type: TokenTypeTerm, Required: false},
				},
				expected: `"NEAR"`,
			},
			{
				name: "Lowercase reserved words",
				tokens: []Token{
					{Value: "and", Type: TokenTypeTerm, Required: false},
					{Value: "or", Type: TokenTypeTerm, Required: false},
				},
				expected: `"and" OR "or"`,
			},
			{
				name: "Mixed case reserved words",
				tokens: []Token{
					{Value: "Not", Type: TokenTypeTerm, Required: false},
					{Value: "near", Type: TokenTypeTerm, Required: false},
				},
				expected: `"Not" OR "near"`,
			},
			{
				name: "Reserved word with regular term",
				tokens: []Token{
					{Value: "cat", Type: TokenTypeTerm, Required: false},
					{Value: "AND", Type: TokenTypeTerm, Required: false},
				},
				expected: `cat OR "AND"`,
			},
			{
				name: "Required reserved word",
				tokens: []Token{
					{Value: "OR", Type: TokenTypeTerm, Required: true},
				},
				expected: `"OR"`,
			},
			{
				name: "Reserved word in phrase (should stay as phrase)",
				tokens: []Token{
					{Value: "cat AND dog", Type: TokenTypePhrase, Required: false},
				},
				expected: `"cat AND dog"`,
			},
			{
				name: "Complex query with reserved word as literal",
				tokens: []Token{
					{Value: "cat", Type: TokenTypeTerm, Required: true},
					{Value: "AND", Type: TokenTypeTerm, Required: false},
					{Value: "dog", Type: TokenTypeTerm, Required: false},
				},
				expected: `cat AND ("AND" OR dog)`,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := parser.BuildFTS5Query(tt.tokens)
				if result != tt.expected {
					t.Errorf("Expected %q, got %q", tt.expected, result)
				}
			})
		}
	})

	t.Run("Tokenize and BuildFTS5Query end-to-end with reserved words", func(t *testing.T) {
		tests := []struct {
			name     string
			query    string
			expected string
		}{
			{
				name:     "Literal AND search",
				query:    "AND",
				expected: `"AND"`,
			},
			{
				name:     "Literal OR search",
				query:    "or",
				expected: `"or"`,
			},
			{
				name:     "Searching for 'not'",
				query:    "not",
				expected: `"not"`,
			},
			{
				name:     "Searching for 'near'",
				query:    "near",
				expected: `"near"`,
			},
			{
				name:     "Reserved word with other terms",
				query:    "cat and dog",
				expected: `cat OR "and" OR dog`,
			},
			{
				name:     "Required term and literal reserved word",
				query:    "+cat AND",
				expected: `cat AND ("AND")`,
			},
			{
				name:     "Phrase containing reserved word",
				query:    `"cat and dog"`,
				expected: `"cat and dog"`,
			},
			{
				name:     "Multiple reserved words",
				query:    "and or not",
				expected: `"and" OR "or" OR "not"`,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				tokens := parser.Tokenize(tt.query)
				result := parser.BuildFTS5Query(tokens)
				if result != tt.expected {
					t.Errorf("Query %q: expected %q, got %q", tt.query, tt.expected, result)
				}
			})
		}
	})
}
