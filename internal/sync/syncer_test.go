package sync

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/philrhinehart/granola-sync/internal/config"
	"github.com/philrhinehart/granola-sync/internal/granola"
	"github.com/philrhinehart/granola-sync/internal/state"
)

type SyncerSuite struct {
	suite.Suite
	tempDir string
	store   *state.Store
	cfg     *config.Config
}

func TestSyncerSuite(t *testing.T) {
	suite.Run(t, new(SyncerSuite))
}

func (s *SyncerSuite) SetupTest() {
	var err error
	s.tempDir, err = os.MkdirTemp("", "syncer-test-*")
	s.Require().NoError(err)

	s.store, err = state.NewStore(":memory:")
	s.Require().NoError(err)

	s.cfg = &config.Config{
		GranolaCachePath: filepath.Join(s.tempDir, "cache.json"),
		LogseqBasePath:   filepath.Join(s.tempDir, "logseq"),
		StateDBPath:      ":memory:",
		DebounceSeconds:  5,
		MinAgeSeconds:    60,
		UserEmail:        "test@example.com",
	}

	// Create logseq directories
	s.Require().NoError(os.MkdirAll(filepath.Join(s.cfg.LogseqBasePath, "pages"), 0o755))
	s.Require().NoError(os.MkdirAll(filepath.Join(s.cfg.LogseqBasePath, "journals"), 0o755))
}

func (s *SyncerSuite) TearDownTest() {
	if s.store != nil {
		_ = s.store.Close()
	}
	_ = os.RemoveAll(s.tempDir)
}

func (s *SyncerSuite) TestHashContent() {
	notes1 := "Notes content"
	notes2 := "Different notes"
	plain1 := "Plain text"

	tests := []struct {
		name    string
		doc1    *granola.Document
		doc2    *granola.Document
		sameDoc bool
	}{
		{
			name:    "same_content_same_hash",
			doc1:    &granola.Document{Title: "Meeting", NotesMarkdown: &notes1},
			doc2:    &granola.Document{Title: "Meeting", NotesMarkdown: &notes1},
			sameDoc: true,
		},
		{
			name:    "different_title_different_hash",
			doc1:    &granola.Document{Title: "Meeting 1"},
			doc2:    &granola.Document{Title: "Meeting 2"},
			sameDoc: false,
		},
		{
			name:    "different_notes_different_hash",
			doc1:    &granola.Document{Title: "Meeting", NotesMarkdown: &notes1},
			doc2:    &granola.Document{Title: "Meeting", NotesMarkdown: &notes2},
			sameDoc: false,
		},
		{
			name:    "with_plain_notes",
			doc1:    &granola.Document{Title: "Meeting", NotesPlain: &plain1},
			doc2:    &granola.Document{Title: "Meeting", NotesPlain: &plain1},
			sameDoc: true,
		},
		{
			name:    "nil_notes_same",
			doc1:    &granola.Document{Title: "Meeting"},
			doc2:    &granola.Document{Title: "Meeting"},
			sameDoc: true,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			hash1 := hashContent(tt.doc1)
			hash2 := hashContent(tt.doc2)

			s.NotEmpty(hash1)
			s.NotEmpty(hash2)

			if tt.sameDoc {
				s.Equal(hash1, hash2)
			} else {
				s.NotEqual(hash1, hash2)
			}
		})
	}
}

func (s *SyncerSuite) TestSortDocumentsByDate() {
	now := time.Now()

	docs := map[string]*granola.Document{
		"doc-3": {ID: "doc-3", CreatedAt: now.Add(2 * time.Hour)},
		"doc-1": {ID: "doc-1", CreatedAt: now},
		"doc-2": {ID: "doc-2", CreatedAt: now.Add(time.Hour)},
	}

	sorted := sortDocumentsByDate(docs)

	s.Len(sorted, 3)
	s.Equal("doc-1", sorted[0].ID)
	s.Equal("doc-2", sorted[1].ID)
	s.Equal("doc-3", sorted[2].ID)
}

func (s *SyncerSuite) TestSortDocumentsByDateWithCalendarEvent() {
	now := time.Now()

	// doc-2 has an earlier calendar event even though CreatedAt is later
	docs := map[string]*granola.Document{
		"doc-1": {ID: "doc-1", CreatedAt: now},
		"doc-2": {
			ID:        "doc-2",
			CreatedAt: now.Add(time.Hour),
			GoogleCalendarEvent: &granola.GoogleCalendarEvent{
				Start: &granola.EventTime{DateTime: now.Add(-time.Hour).Format(time.RFC3339)},
			},
		},
	}

	sorted := sortDocumentsByDate(docs)

	s.Len(sorted, 2)
	s.Equal("doc-2", sorted[0].ID) // Calendar event is earlier
	s.Equal("doc-1", sorted[1].ID)
}

func (s *SyncerSuite) TestTruncate() {
	tests := []struct {
		input    string
		max      int
		expected string
	}{
		{"short", 10, "short"},
		{"exactly10!", 10, "exactly10!"},
		{"this is longer than ten", 10, "this is lo..."},
		{"", 10, ""},
	}

	for _, tt := range tests {
		s.Run(tt.input, func() {
			result := truncate(tt.input, tt.max)
			s.Equal(tt.expected, result)
		})
	}
}

func (s *SyncerSuite) TestSyncWithEmptyCache() {
	// Create empty cache file
	cacheContent := `{"cache": "{\"state\":{\"documents\":{},\"documentPanels\":{}}}", "version": 3}`
	err := os.WriteFile(s.cfg.GranolaCachePath, []byte(cacheContent), 0o644)
	s.Require().NoError(err)

	syncer := NewSyncer(s.cfg, s.store)
	result, err := syncer.Sync(nil, false)

	s.NoError(err)
	s.NotNil(result)
	s.Equal(0, result.NewMeetings)
	s.Equal(0, result.UpdatedMeetings)
}

func (s *SyncerSuite) TestSyncFilteringDeleted() {
	now := time.Now()
	deletedAt := now.Add(-time.Hour)
	oldUpdatedAt := now.Add(-2 * time.Hour) // Older than minAge

	cacheContent := `{
		"cache": "{\"state\":{\"documents\":{\"deleted-doc\":{\"id\":\"deleted-doc\",\"title\":\"Deleted Meeting\",\"created_at\":\"` + now.Add(-3*time.Hour).Format(time.RFC3339) + `\",\"updated_at\":\"` + oldUpdatedAt.Format(time.RFC3339) + `\",\"deleted_at\":\"` + deletedAt.Format(time.RFC3339) + `\",\"type\":\"meeting\"}},\"documentPanels\":{}}}",
		"version": 3
	}`
	err := os.WriteFile(s.cfg.GranolaCachePath, []byte(cacheContent), 0o644)
	s.Require().NoError(err)

	syncer := NewSyncer(s.cfg, s.store)
	result, err := syncer.Sync(nil, false)

	s.NoError(err)
	s.Equal(0, result.NewMeetings) // Deleted doc should be skipped
}

func (s *SyncerSuite) TestSyncFilteringNonAttendee() {
	oldTime := time.Now().Add(-2 * time.Hour).Format(time.RFC3339)

	// Document where user is not an attendee
	cacheContent := `{
		"cache": "{\"state\":{\"documents\":{\"other-meeting\":{\"id\":\"other-meeting\",\"title\":\"Other Meeting\",\"created_at\":\"` + oldTime + `\",\"updated_at\":\"` + oldTime + `\",\"type\":\"meeting\",\"google_calendar_event\":{\"id\":\"cal-1\",\"attendees\":[{\"email\":\"other@example.com\"}]}}},\"documentPanels\":{}}}",
		"version": 3
	}`
	err := os.WriteFile(s.cfg.GranolaCachePath, []byte(cacheContent), 0o644)
	s.Require().NoError(err)

	syncer := NewSyncer(s.cfg, s.store)
	result, err := syncer.Sync(nil, false)

	s.NoError(err)
	s.Equal(0, result.NewMeetings) // Non-attendee doc should be skipped
}

func (s *SyncerSuite) TestSyncFilteringTooRecent() {
	// Document updated just now (too recent)
	recentTime := time.Now().Format(time.RFC3339)

	cacheContent := `{
		"cache": "{\"state\":{\"documents\":{\"recent-doc\":{\"id\":\"recent-doc\",\"title\":\"Recent Meeting\",\"created_at\":\"` + recentTime + `\",\"updated_at\":\"` + recentTime + `\",\"type\":\"meeting\"}},\"documentPanels\":{}}}",
		"version": 3
	}`
	err := os.WriteFile(s.cfg.GranolaCachePath, []byte(cacheContent), 0o644)
	s.Require().NoError(err)

	syncer := NewSyncer(s.cfg, s.store)
	result, err := syncer.Sync(nil, false)

	s.NoError(err)
	s.Equal(0, result.NewMeetings) // Too recent doc should be skipped
}

func (s *SyncerSuite) TestSyncProcessesValidDoc() {
	oldTime := time.Now().Add(-2 * time.Hour).Format(time.RFC3339)

	// Valid document: not deleted, user is attendee (no calendar = include by default), not too recent
	cacheContent := `{
		"cache": "{\"state\":{\"documents\":{\"valid-doc\":{\"id\":\"valid-doc\",\"title\":\"Valid Meeting\",\"created_at\":\"` + oldTime + `\",\"updated_at\":\"` + oldTime + `\",\"type\":\"meeting\"}},\"documentPanels\":{}}}",
		"version": 3
	}`
	err := os.WriteFile(s.cfg.GranolaCachePath, []byte(cacheContent), 0o644)
	s.Require().NoError(err)

	syncer := NewSyncer(s.cfg, s.store)
	result, err := syncer.Sync(nil, false)

	s.NoError(err)
	s.Equal(1, result.NewMeetings)

	// Verify file was created
	files, _ := filepath.Glob(filepath.Join(s.cfg.LogseqBasePath, "pages", "*.md"))
	s.Len(files, 1)
}

func (s *SyncerSuite) TestSyncDryRun() {
	oldTime := time.Now().Add(-2 * time.Hour).Format(time.RFC3339)

	cacheContent := `{
		"cache": "{\"state\":{\"documents\":{\"dry-run-doc\":{\"id\":\"dry-run-doc\",\"title\":\"Dry Run Meeting\",\"created_at\":\"` + oldTime + `\",\"updated_at\":\"` + oldTime + `\",\"type\":\"meeting\"}},\"documentPanels\":{}}}",
		"version": 3
	}`
	err := os.WriteFile(s.cfg.GranolaCachePath, []byte(cacheContent), 0o644)
	s.Require().NoError(err)

	syncer := NewSyncer(s.cfg, s.store)
	result, err := syncer.Sync(nil, true) // dry run = true

	s.NoError(err)
	s.Equal(1, result.NewMeetings)

	// Verify no file was created in dry run
	files, _ := filepath.Glob(filepath.Join(s.cfg.LogseqBasePath, "pages", "*.md"))
	s.Len(files, 0)
}

func (s *SyncerSuite) TestSyncWithSinceFilter() {
	oldTime := time.Now().Add(-48 * time.Hour)
	recentTime := time.Now().Add(-2 * time.Hour)
	sinceTime := time.Now().Add(-24 * time.Hour)

	// Two documents: one before since, one after
	cacheContent := `{
		"cache": "{\"state\":{\"documents\":{\"old-doc\":{\"id\":\"old-doc\",\"title\":\"Old Meeting\",\"created_at\":\"` + oldTime.Format(time.RFC3339) + `\",\"updated_at\":\"` + oldTime.Format(time.RFC3339) + `\",\"type\":\"meeting\"},\"recent-doc\":{\"id\":\"recent-doc\",\"title\":\"Recent Meeting\",\"created_at\":\"` + recentTime.Format(time.RFC3339) + `\",\"updated_at\":\"` + recentTime.Format(time.RFC3339) + `\",\"type\":\"meeting\"}},\"documentPanels\":{}}}",
		"version": 3
	}`
	err := os.WriteFile(s.cfg.GranolaCachePath, []byte(cacheContent), 0o644)
	s.Require().NoError(err)

	syncer := NewSyncer(s.cfg, s.store)
	result, err := syncer.Sync(&sinceTime, false)

	s.NoError(err)
	s.Equal(1, result.NewMeetings) // Only the recent one should be processed
}

func (s *SyncerSuite) TestSyncSkipsAlreadySynced() {
	// Use a fixed time string to avoid nanosecond precision issues
	oldTimeStr := "2024-01-15T10:00:00Z"
	oldTime, _ := time.Parse(time.RFC3339, oldTimeStr)

	cacheContent := `{
		"cache": "{\"state\":{\"documents\":{\"synced-doc\":{\"id\":\"synced-doc\",\"title\":\"Already Synced\",\"created_at\":\"` + oldTimeStr + `\",\"updated_at\":\"` + oldTimeStr + `\",\"type\":\"meeting\"}},\"documentPanels\":{}}}",
		"version": 3
	}`
	err := os.WriteFile(s.cfg.GranolaCachePath, []byte(cacheContent), 0o644)
	s.Require().NoError(err)

	// Pre-sync the document with matching hash and timestamp
	syncedDoc := &state.SyncedDocument{
		ID:               "synced-doc",
		Title:            "Already Synced",
		SyncedAt:         time.Now(),
		GranolaUpdatedAt: &oldTime,
		LogseqPagePath:   "/pages/already-synced.md",
		ContentHash:      hashContent(&granola.Document{Title: "Already Synced"}),
	}
	s.Require().NoError(s.store.MarkSynced(syncedDoc))

	syncer := NewSyncer(s.cfg, s.store)
	result, err := syncer.Sync(nil, false)

	s.NoError(err)
	s.Equal(0, result.NewMeetings)
	s.Equal(0, result.UpdatedMeetings)
}
