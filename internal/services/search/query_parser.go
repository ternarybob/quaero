package search

import (
	"strings"
	"unicode"
)

// TokenType represents the type of a query token
type TokenType int

const (
	// TokenTypeTerm represents a regular search term
	TokenTypeTerm TokenType = iota
	// TokenTypePhrase represents a quoted phrase
	TokenTypePhrase
	// TokenTypeQualifier represents a key:value pair
	TokenTypeQualifier
	// TokenTypeOperator represents special operators (future: NOT, etc.)
	TokenTypeOperator
)

// Token represents a parsed token from the query
type Token struct {
	Value    string
	Type     TokenType
	Required bool // True if prefixed with +
}

// QueryParser handles parsing of Google-style queries into FTS5 syntax
// Stateless parser that can be reused across multiple queries
type QueryParser struct{}

// NewQueryParser creates a new query parser instance
func NewQueryParser() *QueryParser {
	return &QueryParser{}
}

// Tokenize breaks a query string into tokens, respecting quotes and operators
// Uses rune-safe iteration to properly handle Unicode characters
// Handles:
// - Quoted phrases: "cat on mat" → single PHRASE token
// - Required terms: +cat → TERM token with Required=true
// - Qualifiers: document_type:jira → QUALIFIER token
// - Regular terms: cat dog → separate TERM tokens
// - Unicode: handles multi-byte characters correctly (e.g., 猫, собака, emoji)
func (p *QueryParser) Tokenize(query string) []Token {
	var tokens []Token
	var current strings.Builder
	var inQuote bool
	var escaped bool
	var required bool

	query = strings.TrimSpace(query)

	// Use rune-safe iteration to handle multi-byte Unicode characters
	for _, ch := range query {
		// Handle escape sequences
		if escaped {
			current.WriteRune(ch)
			escaped = false
			continue
		}

		if ch == '\\' && inQuote {
			escaped = true
			continue
		}

		// Handle quoted phrases
		if ch == '"' {
			if inQuote {
				// End of phrase
				if current.Len() > 0 {
					tokens = append(tokens, Token{
						Value:    current.String(),
						Type:     TokenTypePhrase,
						Required: required,
					})
					current.Reset()
				}
				inQuote = false
				required = false
			} else {
				// Start of phrase - save any pending term first
				if current.Len() > 0 {
					p.flushTerm(&tokens, &current, &required)
				}
				inQuote = true
			}
			continue
		}

		// Inside quotes, accumulate all characters
		if inQuote {
			current.WriteRune(ch)
			continue
		}

		// Handle + prefix for required terms
		if ch == '+' && current.Len() == 0 {
			required = true
			continue
		}

		// Handle whitespace as term delimiter (Unicode-aware)
		if unicode.IsSpace(ch) {
			if current.Len() > 0 {
				p.flushTerm(&tokens, &current, &required)
			}
			continue
		}

		// Accumulate regular characters
		current.WriteRune(ch)
	}

	// Flush any remaining term
	if current.Len() > 0 {
		if inQuote {
			// Unclosed quote - treat as phrase anyway
			tokens = append(tokens, Token{
				Value:    current.String(),
				Type:     TokenTypePhrase,
				Required: required,
			})
		} else {
			p.flushTerm(&tokens, &current, &required)
		}
	}

	return tokens
}

// flushTerm adds the current term to tokens and resets the builder
// Determines if term is a qualifier (contains :) or regular term
func (p *QueryParser) flushTerm(tokens *[]Token, current *strings.Builder, required *bool) {
	value := current.String()
	tokenType := TokenTypeTerm

	// Check if this is a qualifier (key:value)
	if p.IsQualifier(value) {
		tokenType = TokenTypeQualifier
	}

	*tokens = append(*tokens, Token{
		Value:    value,
		Type:     tokenType,
		Required: *required,
	})

	current.Reset()
	*required = false
}

// IsQualifier checks if a token matches the key:value pattern
// Valid qualifiers: document_type:jira, case:match
func (p *QueryParser) IsQualifier(token string) bool {
	// Must contain exactly one colon
	colonIdx := strings.Index(token, ":")
	if colonIdx == -1 || colonIdx == 0 || colonIdx == len(token)-1 {
		return false
	}

	// Check for multiple colons (not a valid qualifier)
	if strings.Count(token, ":") > 1 {
		return false
	}

	// Key must be alphanumeric with underscores
	key := token[:colonIdx]
	for _, ch := range key {
		if !unicode.IsLetter(ch) && !unicode.IsDigit(ch) && ch != '_' {
			return false
		}
	}

	return true
}

// SplitQualifier splits a qualifier token into (key, value)
// Example: "document_type:jira" → ("document_type", "jira")
func (p *QueryParser) SplitQualifier(qualifier string) (string, string) {
	parts := strings.SplitN(qualifier, ":", 2)
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}

// EscapeFTS5 escapes special characters for FTS5 queries
// FTS5 special characters that need escaping: " * ^
// Note: + and - are handled separately in query building
func (p *QueryParser) EscapeFTS5(term string) string {
	// Replace double quotes with escaped quotes
	term = strings.ReplaceAll(term, `"`, `""`)

	// FTS5 doesn't require escaping * or ^ in regular terms
	// They only have special meaning in specific contexts
	// If issues arise, add escaping here

	return term
}

// isReservedWord checks if a term is an FTS5 reserved keyword
// Reserved words: AND, OR, NOT, NEAR (case-insensitive)
// These must be quoted to be treated as literals rather than operators
func (p *QueryParser) isReservedWord(term string) bool {
	reserved := []string{"AND", "OR", "NOT", "NEAR"}

	for _, keyword := range reserved {
		if strings.EqualFold(term, keyword) {
			return true
		}
	}

	return false
}

// needsQuoting checks if a term contains FTS5 special characters that require quoting
// Special characters include: space, hyphen, colon, parentheses, operators
// Also checks for FTS5 reserved keywords (AND, OR, NOT, NEAR)
// Terms with these characteristics must be quoted to be treated as literals in FTS5
func (p *QueryParser) needsQuoting(term string) bool {
	// Check for reserved keywords first
	if p.isReservedWord(term) {
		return true
	}

	// Check for characters that have special meaning in FTS5 queries
	specialChars := []rune{' ', '-', ':', '(', ')', '*', '^', '+'}

	for _, ch := range term {
		for _, special := range specialChars {
			if ch == special {
				return true
			}
		}
	}

	return false
}

// BuildFTS5Query converts tokens into FTS5 query syntax
// Handles:
// - Empty query → ""
// - Required terms: +cat +dog → "cat AND dog"
// - Optional terms: cat dog → "cat OR dog"
// - Mixed: +cat dog mat → "cat AND (dog OR mat)"
// - Phrases: "cat on mat" → preserved as-is
func (p *QueryParser) BuildFTS5Query(tokens []Token) string {
	var requiredTerms []string
	var optionalTerms []string

	for _, token := range tokens {
		// Skip qualifiers - they're already extracted
		if token.Type == TokenTypeQualifier {
			continue
		}

		var termValue string
		if token.Type == TokenTypePhrase {
			// Preserve quoted phrases
			termValue = `"` + p.EscapeFTS5(token.Value) + `"`
		} else {
			// Auto-quote regular terms with special characters
			escapedTerm := p.EscapeFTS5(token.Value)
			if p.needsQuoting(token.Value) {
				termValue = `"` + escapedTerm + `"`
			} else {
				termValue = escapedTerm
			}
		}

		if token.Required {
			requiredTerms = append(requiredTerms, termValue)
		} else {
			optionalTerms = append(optionalTerms, termValue)
		}
	}

	// Build FTS5 query
	if len(requiredTerms) == 0 && len(optionalTerms) == 0 {
		return ""
	}

	var parts []string

	// Add required terms with AND
	if len(requiredTerms) > 0 {
		parts = append(parts, strings.Join(requiredTerms, " AND "))
	}

	// Add optional terms with OR
	if len(optionalTerms) > 0 {
		optionalQuery := strings.Join(optionalTerms, " OR ")

		// If we have both required and optional, wrap optional in parentheses
		if len(requiredTerms) > 0 {
			optionalQuery = "(" + optionalQuery + ")"
		}

		parts = append(parts, optionalQuery)
	}

	// Combine with AND
	return strings.Join(parts, " AND ")
}

// ExtractQualifiers extracts and removes qualifier tokens from the token list
// Returns a map of qualifier key-value pairs
// Recognized qualifiers:
// - document_type: jira, confluence, github
// - case: match (for case-sensitive search)
func (p *QueryParser) ExtractQualifiers(tokens []Token) map[string]string {
	qualifiers := make(map[string]string)

	for _, token := range tokens {
		if token.Type == TokenTypeQualifier {
			key, value := p.SplitQualifier(token.Value)
			if key != "" {
				// Normalize known qualifiers
				switch key {
				case "document_type", "type", "source":
					qualifiers["document_type"] = strings.ToLower(value)
				case "case":
					qualifiers["case"] = strings.ToLower(value)
				default:
					// Store unknown qualifiers as-is (for future extension)
					qualifiers[key] = value
				}
			}
		}
	}

	return qualifiers
}
