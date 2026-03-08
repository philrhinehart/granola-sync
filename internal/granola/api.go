package granola

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

const defaultBaseURL = "https://api.granola.ai"

// ErrUnauthorized is returned when the API rejects the auth token.
var ErrUnauthorized = errors.New("unauthorized")

// APIClient communicates with the Granola API.
type APIClient struct {
	client  *http.Client
	baseURL string
	token   string
}

// NewAPIClient creates a new Granola API client.
func NewAPIClient(baseURL, token string) *APIClient {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &APIClient{
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
		baseURL: baseURL,
		token:   token,
	}
}

// panelsRequest is the request body for get-document-panels.
type panelsRequest struct {
	DocumentID string `json:"document_id"`
}

// FetchDocumentPanels fetches panels for a document from the Granola API.
// The API returns a bare JSON array of panel objects.
func (c *APIClient) FetchDocumentPanels(ctx context.Context, docID string) ([]*DocumentPanel, error) {
	body, err := json.Marshal(panelsRequest{DocumentID: docID})
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/get-document-panels", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("making request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, ErrUnauthorized
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(respBody))
	}

	var panels []*DocumentPanel
	if err := json.NewDecoder(resp.Body).Decode(&panels); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return panels, nil
}
