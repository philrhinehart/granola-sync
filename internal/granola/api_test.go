package granola

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/suite"
)

type APISuite struct {
	suite.Suite
}

func TestAPISuite(t *testing.T) {
	suite.Run(t, new(APISuite))
}

func (s *APISuite) TestFetchDocumentPanels() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.Equal("POST", r.Method)
		s.Equal("/v1/get-document-panels", r.URL.Path)
		s.Equal("Bearer test-token", r.Header.Get("Authorization"))
		s.Equal("application/json", r.Header.Get("Content-Type"))

		var req panelsRequest
		s.NoError(json.NewDecoder(r.Body).Decode(&req))
		s.Equal("doc-123", req.DocumentID)

		panels := []*DocumentPanel{
			{
				ID:         "panel-1",
				DocumentID: "doc-123",
				Title:      "Summary",
				Content: map[string]interface{}{
					"content": []interface{}{
						map[string]interface{}{
							"type": "heading",
							"content": []interface{}{
								map[string]interface{}{"text": "Meeting Notes"},
							},
						},
					},
				},
				ContentUpdatedAt: "2024-01-15T10:00:00Z",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		s.NoError(json.NewEncoder(w).Encode(panels))
	}))
	defer server.Close()

	client := NewAPIClient(server.URL, "test-token")
	panels, err := client.FetchDocumentPanels(context.Background(), "doc-123")

	s.NoError(err)
	s.Len(panels, 1)
	s.Equal("Summary", panels[0].Title)
	s.Equal("doc-123", panels[0].DocumentID)
}

func (s *APISuite) TestFetchDocumentPanelsUnauthorized() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error": "unauthorized"}`))
	}))
	defer server.Close()

	client := NewAPIClient(server.URL, "bad-token")
	_, err := client.FetchDocumentPanels(context.Background(), "doc-123")

	s.Error(err)
	s.True(errors.Is(err, ErrUnauthorized))
}

func (s *APISuite) TestFetchDocumentPanelsServerError() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error": "internal"}`))
	}))
	defer server.Close()

	client := NewAPIClient(server.URL, "test-token")
	_, err := client.FetchDocumentPanels(context.Background(), "doc-123")

	s.Error(err)
	s.Contains(err.Error(), "500")
	s.False(errors.Is(err, ErrUnauthorized))
}

func (s *APISuite) TestFetchDocumentPanelsEmptyResponse() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		s.NoError(json.NewEncoder(w).Encode([]*DocumentPanel{}))
	}))
	defer server.Close()

	client := NewAPIClient(server.URL, "test-token")
	panels, err := client.FetchDocumentPanels(context.Background(), "doc-123")

	s.NoError(err)
	s.Len(panels, 0)
}

func (s *APISuite) TestFetchDocumentPanelsCancelled() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		s.NoError(json.NewEncoder(w).Encode([]*DocumentPanel{}))
	}))
	defer server.Close()

	client := NewAPIClient(server.URL, "test-token")
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := client.FetchDocumentPanels(ctx, "doc-123")
	s.Error(err)
}

func (s *APISuite) TestDefaultBaseURL() {
	client := NewAPIClient("", "token")
	s.Equal(defaultBaseURL, client.baseURL)
}
