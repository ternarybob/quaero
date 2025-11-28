package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// GraphQL endpoint
const githubGraphQLEndpoint = "https://api.github.com/graphql"

// BulkFileResult represents the result of fetching a single file via GraphQL
type BulkFileResult struct {
	Path     string
	Content  string
	Size     int
	IsBinary bool
	Error    error
}

// graphqlClient wraps HTTP client for GraphQL requests
type graphqlClient struct {
	httpClient *http.Client
	token      string
}

// newGraphQLClient creates a new GraphQL client with the given token
func newGraphQLClient(httpClient *http.Client, token string) *graphqlClient {
	return &graphqlClient{
		httpClient: httpClient,
		token:      token,
	}
}

// graphqlRequest represents a GraphQL request
type graphqlRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables,omitempty"`
}

// graphqlResponse represents a GraphQL response
type graphqlResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []struct {
		Message string `json:"message"`
		Path    []any  `json:"path"`
	} `json:"errors"`
}

// blobResponse represents a file blob from GraphQL
type blobResponse struct {
	Text     *string `json:"text"`
	ByteSize int     `json:"byteSize"`
	IsBinary bool    `json:"isBinary"`
}

// repositoryResponse represents the repository data from GraphQL
type repositoryResponse struct {
	Repository map[string]json.RawMessage `json:"repository"`
}

// BulkGetFileContent fetches multiple files in a single GraphQL request
// Maximum recommended batch size is 50-100 files per request
func (c *Connector) BulkGetFileContent(ctx context.Context, owner, repo, branch string, paths []string) ([]BulkFileResult, error) {
	if len(paths) == 0 {
		return nil, nil
	}

	if len(paths) > 100 {
		return nil, fmt.Errorf("batch size exceeds maximum of 100 files")
	}

	// Get token from the underlying HTTP client
	token := c.getToken()
	if token == "" {
		return nil, fmt.Errorf("no authentication token available")
	}

	gqlClient := newGraphQLClient(c.client.Client(), token)

	// Build the dynamic GraphQL query
	query := buildBulkFileQuery(owner, repo, branch, paths)

	// Execute the query
	result, err := gqlClient.execute(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("graphql query failed: %w", err)
	}

	// Parse the response
	return parseGraphQLResponse(result, paths)
}

// getToken returns the stored token for GraphQL requests
func (c *Connector) getToken() string {
	return c.token
}

// buildBulkFileQuery constructs a GraphQL query for multiple files
func buildBulkFileQuery(owner, repo, branch string, paths []string) string {
	var builder strings.Builder

	builder.WriteString("query BulkFileContent {\n")
	builder.WriteString(fmt.Sprintf("  repository(owner: %q, name: %q) {\n", owner, repo))

	for i, path := range paths {
		// Create a unique alias for each file
		alias := fmt.Sprintf("f%d", i)
		expression := fmt.Sprintf("%s:%s", branch, path)

		builder.WriteString(fmt.Sprintf("    %s: object(expression: %q) {\n", alias, expression))
		builder.WriteString("      ... on Blob {\n")
		builder.WriteString("        text\n")
		builder.WriteString("        byteSize\n")
		builder.WriteString("        isBinary\n")
		builder.WriteString("      }\n")
		builder.WriteString("    }\n")
	}

	builder.WriteString("  }\n")
	builder.WriteString("}\n")

	return builder.String()
}

// execute sends a GraphQL query and returns the raw response
func (c *graphqlClient) execute(ctx context.Context, query string) (json.RawMessage, error) {
	reqBody := graphqlRequest{
		Query: query,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", githubGraphQLEndpoint, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("graphql request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var gqlResp graphqlResponse
	if err := json.Unmarshal(body, &gqlResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(gqlResp.Errors) > 0 {
		var errMsgs []string
		for _, e := range gqlResp.Errors {
			errMsgs = append(errMsgs, e.Message)
		}
		return nil, fmt.Errorf("graphql errors: %s", strings.Join(errMsgs, "; "))
	}

	return gqlResp.Data, nil
}

// parseGraphQLResponse parses the GraphQL response and maps results to file paths
func parseGraphQLResponse(data json.RawMessage, paths []string) ([]BulkFileResult, error) {
	var repoResp repositoryResponse
	if err := json.Unmarshal(data, &repoResp); err != nil {
		return nil, fmt.Errorf("failed to parse repository data: %w", err)
	}

	results := make([]BulkFileResult, len(paths))

	for i, path := range paths {
		alias := fmt.Sprintf("f%d", i)
		results[i] = BulkFileResult{
			Path: path,
		}

		rawBlob, exists := repoResp.Repository[alias]
		if !exists || string(rawBlob) == "null" {
			results[i].Error = fmt.Errorf("file not found: %s", path)
			continue
		}

		var blob blobResponse
		if err := json.Unmarshal(rawBlob, &blob); err != nil {
			results[i].Error = fmt.Errorf("failed to parse blob for %s: %w", path, err)
			continue
		}

		results[i].Size = blob.ByteSize
		results[i].IsBinary = blob.IsBinary

		if blob.Text != nil {
			results[i].Content = *blob.Text
		} else if blob.IsBinary {
			results[i].Content = "[Binary file - content not stored]"
		}
	}

	return results, nil
}
