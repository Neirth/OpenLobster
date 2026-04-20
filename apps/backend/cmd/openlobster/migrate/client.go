// # License
// See LICENSE in the root of the repository.
package migrate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

// gqlClient is a minimal GraphQL HTTP client.
type gqlClient struct {
	endpoint string
	apiKey   string
	dryRun   bool
	http     *http.Client
}

func newGQLClient(endpoint, apiKey string, dryRun bool) *gqlClient {
	return &gqlClient{
		endpoint: endpoint,
		apiKey:   apiKey,
		dryRun:   dryRun,
		http:     &http.Client{},
	}
}

// do executes a GraphQL mutation/query and unmarshals the response into out.
func (c *gqlClient) do(query string, variables map[string]any, out any) error {
	body, err := json.Marshal(map[string]any{"query": query, "variables": variables})
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	var gqlResp struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&gqlResp); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	if len(gqlResp.Errors) > 0 {
		return fmt.Errorf("GraphQL error: %s", gqlResp.Errors[0].Message)
	}
	if out != nil && gqlResp.Data != nil {
		return json.Unmarshal(gqlResp.Data, out)
	}
	return nil
}
