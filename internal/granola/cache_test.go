package granola

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"
)

type CacheSuite struct {
	suite.Suite
	testdataDir string
}

func TestCacheSuite(t *testing.T) {
	suite.Run(t, new(CacheSuite))
}

func (s *CacheSuite) SetupSuite() {
	// Get the testdata directory relative to this test file
	s.testdataDir = filepath.Join("testdata")
}

func (s *CacheSuite) TestParseCacheData() {
	tests := []struct {
		name        string
		file        string
		wantErr     bool
		errContains string
		validate    func(map[string]*Document)
	}{
		{
			name:    "valid_cache",
			file:    "valid_cache.json",
			wantErr: false,
			validate: func(docs map[string]*Document) {
				s.Len(docs, 1)
				doc, ok := docs["doc-1"]
				s.True(ok)
				s.Equal("Test Meeting", doc.Title)
				s.Equal("doc-1", doc.ID)
			},
		},
		{
			name:    "with_panels",
			file:    "with_panels.json",
			wantErr: false,
			validate: func(docs map[string]*Document) {
				s.Len(docs, 1)
				doc := docs["doc-1"]
				s.NotNil(doc.NotesMarkdown)
				s.Contains(*doc.NotesMarkdown, "Meeting summary")
			},
		},
		{
			name:    "empty_documents",
			file:    "empty_documents.json",
			wantErr: false,
			validate: func(docs map[string]*Document) {
				s.Len(docs, 0)
			},
		},
		{
			name:    "valid_cache_v4",
			file:    "valid_cache_v4.json",
			wantErr: false,
			validate: func(docs map[string]*Document) {
				s.Len(docs, 1)
				doc, ok := docs["doc-1"]
				s.True(ok)
				s.Equal("Test Meeting V4", doc.Title)
			},
		},
		{
			name:    "with_notes_v4",
			file:    "with_notes_v4.json",
			wantErr: false,
			validate: func(docs map[string]*Document) {
				s.Len(docs, 1)
				doc := docs["doc-1"]
				s.NotNil(doc.NotesMarkdown)
				s.Contains(*doc.NotesMarkdown, "Action Items")
				s.Contains(*doc.NotesMarkdown, "Follow up on the proposal")
			},
		},
		{
			name:    "empty_documents_v4",
			file:    "empty_documents_v4.json",
			wantErr: false,
			validate: func(docs map[string]*Document) {
				s.Len(docs, 0)
			},
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			data, err := os.ReadFile(filepath.Join(s.testdataDir, tt.file))
			s.Require().NoError(err)

			docs, err := ParseCacheData(data)
			if tt.wantErr {
				s.Error(err)
				if tt.errContains != "" {
					s.Contains(err.Error(), tt.errContains)
				}
			} else {
				s.NoError(err)
				if tt.validate != nil {
					tt.validate(docs)
				}
			}
		})
	}
}

func (s *CacheSuite) TestParseCacheDataErrors() {
	tests := []struct {
		name        string
		data        string
		errContains string
	}{
		{
			name:        "malformed_outer_json",
			data:        `{invalid json`,
			errContains: "parsing outer JSON",
		},
		{
			name:        "malformed_inner_json",
			data:        `{"cache": "{invalid inner", "version": 1}`,
			errContains: "parsing inner JSON",
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			_, err := ParseCacheData([]byte(tt.data))
			s.Error(err)
			s.Contains(err.Error(), tt.errContains)
		})
	}
}

func (s *CacheSuite) TestParseCache() {
	// Test file not found
	_, err := ParseCache("/nonexistent/path/cache.json")
	s.Error(err)
	s.Contains(err.Error(), "reading cache file")
}

func (s *CacheSuite) TestExtractMarkdownFromContent() {
	tests := []struct {
		name     string
		content  interface{}
		expected string
	}{
		{
			name:     "nil_content",
			content:  nil,
			expected: "",
		},
		{
			name:     "not_a_map",
			content:  "string",
			expected: "",
		},
		{
			name:     "empty_map",
			content:  map[string]interface{}{},
			expected: "",
		},
		{
			name: "heading_node",
			content: map[string]interface{}{
				"content": []interface{}{
					map[string]interface{}{
						"type": "heading",
						"content": []interface{}{
							map[string]interface{}{"text": "Section Title"},
						},
					},
				},
			},
			expected: "- **Section Title**\n",
		},
		{
			name: "paragraph_node",
			content: map[string]interface{}{
				"content": []interface{}{
					map[string]interface{}{
						"type": "paragraph",
						"content": []interface{}{
							map[string]interface{}{"text": "Some text"},
						},
					},
				},
			},
			expected: "- Some text\n",
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			result := ExtractMarkdownFromContent(tt.content)
			s.Equal(tt.expected, result)
		})
	}
}
