package sync

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/philrhinehart/granola-sync/internal/config"
	"github.com/philrhinehart/granola-sync/internal/state"
)

// testDoc represents a test document for building cache fixtures
type testDoc struct {
	ID        string
	Title     string
	Email     string
	Notes     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// makeDocument creates a test document with sensible defaults
func makeDocument(id, title, email, notes string) testDoc {
	// Use a fixed time in the past to ensure meetings aren't too recent
	baseTime := time.Date(2025, 1, 28, 10, 0, 0, 0, time.UTC)
	return testDoc{
		ID:        id,
		Title:     title,
		Email:     email,
		Notes:     notes,
		CreatedAt: baseTime,
		UpdatedAt: baseTime.Add(time.Hour),
	}
}

// makeCache builds the double-encoded JSON cache format
func makeCache(docs []testDoc) string {
	// Build documents map
	documents := make(map[string]interface{})
	documentPanels := make(map[string]map[string]interface{})

	for _, doc := range docs {
		// Build the document
		docMap := map[string]interface{}{
			"id":         doc.ID,
			"title":      doc.Title,
			"created_at": doc.CreatedAt.Format(time.RFC3339),
			"updated_at": doc.UpdatedAt.Format(time.RFC3339),
			"type":       "meeting",
		}

		// Add calendar event with attendees so user email matching works
		if doc.Email != "" {
			docMap["google_calendar_event"] = map[string]interface{}{
				"id":      "event-" + doc.ID,
				"summary": doc.Title,
				"start": map[string]interface{}{
					"dateTime": doc.CreatedAt.Format(time.RFC3339),
					"timeZone": "America/Los_Angeles",
				},
				"end": map[string]interface{}{
					"dateTime": doc.CreatedAt.Add(time.Hour).Format(time.RFC3339),
					"timeZone": "America/Los_Angeles",
				},
				"attendees": []map[string]interface{}{
					{
						"email":          doc.Email,
						"displayName":    "Test User",
						"responseStatus": "accepted",
						"self":           true,
					},
				},
			}
		}

		documents[doc.ID] = docMap

		// Build panels with notes if provided
		if doc.Notes != "" {
			documentPanels[doc.ID] = map[string]interface{}{
				"panel-" + doc.ID: map[string]interface{}{
					"id":          "panel-" + doc.ID,
					"document_id": doc.ID,
					"title":       "Summary",
					"content": map[string]interface{}{
						"content": []interface{}{
							map[string]interface{}{
								"type": "paragraph",
								"content": []interface{}{
									map[string]interface{}{"text": doc.Notes},
								},
							},
						},
					},
				},
			}
		}
	}

	// Build inner state
	innerState := map[string]interface{}{
		"state": map[string]interface{}{
			"documents":      documents,
			"documentPanels": documentPanels,
		},
	}

	// Encode inner state to string
	innerJSON, _ := json.Marshal(innerState)

	// Build outer cache
	outer := map[string]interface{}{
		"cache":   string(innerJSON),
		"version": 3,
	}

	outerJSON, _ := json.Marshal(outer)
	return string(outerJSON)
}

// writeCache writes the cache content to the specified path
func writeCache(t *testing.T, path, content string) {
	t.Helper()
	err := os.WriteFile(path, []byte(content), 0o644)
	require.NoError(t, err)
}

func TestSyncE2E(t *testing.T) {
	// Setup temp dirs
	tmpDir := t.TempDir()
	logseqDir := filepath.Join(tmpDir, "logseq")
	require.NoError(t, os.MkdirAll(filepath.Join(logseqDir, "pages"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(logseqDir, "journals"), 0o755))

	cachePath := filepath.Join(tmpDir, "cache.json")
	stateDBPath := filepath.Join(tmpDir, "state.db")

	// Create config
	cfg := &config.Config{
		GranolaCachePath: cachePath,
		LogseqBasePath:   logseqDir,
		StateDBPath:      stateDBPath,
		UserEmail:        "test@example.com",
		UserName:         "Test User",
		MinAgeSeconds:    0, // Don't skip recent meetings
	}

	// Test 1: Initial sync with one meeting
	t.Run("initial_sync_creates_page_and_journal", func(t *testing.T) {
		writeCache(t, cachePath, makeCache([]testDoc{
			makeDocument("doc1", "Team Standup", "test@example.com", "Action item 1"),
		}))

		store, err := state.NewStore(stateDBPath)
		require.NoError(t, err)
		defer func() { _ = store.Close() }()

		syncer := NewSyncer(cfg, store)
		result, err := syncer.Sync(nil, false)
		require.NoError(t, err)

		assert.Equal(t, 1, result.NewMeetings)
		assert.Equal(t, 0, result.UpdatedMeetings)

		// Verify meeting page was created
		pagePattern := filepath.Join(logseqDir, "pages", "meetings___2025-01-28 Team Standup.md")
		matches, _ := filepath.Glob(pagePattern)
		assert.Len(t, matches, 1, "Expected meeting page to be created")

		// Verify journal entry was created
		journalPath := filepath.Join(logseqDir, "journals", "2025_01_28.md")
		_, err = os.Stat(journalPath)
		assert.NoError(t, err, "Expected journal file to be created")
	})

	// Test 2: Re-sync with same content - no changes
	t.Run("resync_same_content_no_changes", func(t *testing.T) {
		store, err := state.NewStore(stateDBPath)
		require.NoError(t, err)
		defer func() { _ = store.Close() }()

		syncer := NewSyncer(cfg, store)
		result, err := syncer.Sync(nil, false)
		require.NoError(t, err)

		assert.Equal(t, 0, result.NewMeetings)
		assert.Equal(t, 0, result.UpdatedMeetings)
	})

	// Test 3: Update notes - should detect change
	t.Run("update_notes_detected", func(t *testing.T) {
		writeCache(t, cachePath, makeCache([]testDoc{
			makeDocument("doc1", "Team Standup", "test@example.com", "Updated notes content"),
		}))

		store, err := state.NewStore(stateDBPath)
		require.NoError(t, err)
		defer func() { _ = store.Close() }()

		syncer := NewSyncer(cfg, store)
		result, err := syncer.Sync(nil, false)
		require.NoError(t, err)

		assert.Equal(t, 0, result.NewMeetings)
		assert.Equal(t, 1, result.UpdatedMeetings)

		// Verify page content was updated
		pageContent, err := os.ReadFile(filepath.Join(logseqDir, "pages", "meetings___2025-01-28 Team Standup.md"))
		require.NoError(t, err)
		assert.Contains(t, string(pageContent), "Updated notes content")
	})

	// Test 4: Add second meeting
	t.Run("add_second_meeting", func(t *testing.T) {
		writeCache(t, cachePath, makeCache([]testDoc{
			makeDocument("doc1", "Team Standup", "test@example.com", "Updated notes content"),
			makeDocument("doc2", "1-1 Meeting", "test@example.com", "Discussion points"),
		}))

		store, err := state.NewStore(stateDBPath)
		require.NoError(t, err)
		defer func() { _ = store.Close() }()

		syncer := NewSyncer(cfg, store)
		result, err := syncer.Sync(nil, false)
		require.NoError(t, err)

		assert.Equal(t, 1, result.NewMeetings)
		assert.Equal(t, 0, result.UpdatedMeetings)

		// Verify second meeting page was created
		page2Path := filepath.Join(logseqDir, "pages", "meetings___2025-01-28 1-1 Meeting.md")
		_, err = os.Stat(page2Path)
		assert.NoError(t, err, "Expected second meeting page to be created")

		// Verify journal has entry for second meeting (appended, not duplicated)
		journalContent, err := os.ReadFile(filepath.Join(logseqDir, "journals", "2025_01_28.md"))
		require.NoError(t, err)
		assert.Contains(t, string(journalContent), "Team Standup")
		assert.Contains(t, string(journalContent), "1-1 Meeting")
	})

	// Test 5: Skip meetings not attended by user
	t.Run("skip_meetings_not_attended", func(t *testing.T) {
		writeCache(t, cachePath, makeCache([]testDoc{
			makeDocument("doc1", "Team Standup", "test@example.com", "Updated notes content"),
			makeDocument("doc2", "1-1 Meeting", "test@example.com", "Discussion points"),
			makeDocument("doc3", "Other Team Meeting", "other@example.com", "Not my meeting"),
		}))

		store, err := state.NewStore(stateDBPath)
		require.NoError(t, err)
		defer func() { _ = store.Close() }()

		syncer := NewSyncer(cfg, store)
		result, err := syncer.Sync(nil, false)
		require.NoError(t, err)

		// doc3 should be skipped because user email doesn't match
		assert.Equal(t, 0, result.NewMeetings)
		assert.Equal(t, 0, result.UpdatedMeetings)

		// Verify page was NOT created for doc3
		page3Path := filepath.Join(logseqDir, "pages", "meetings___2025-01-28 Other Team Meeting.md")
		_, err = os.Stat(page3Path)
		assert.True(t, os.IsNotExist(err), "Expected page for non-attended meeting to NOT be created")
	})
}

func TestSyncE2E_DeletedDocument(t *testing.T) {
	tmpDir := t.TempDir()
	logseqDir := filepath.Join(tmpDir, "logseq")
	require.NoError(t, os.MkdirAll(filepath.Join(logseqDir, "pages"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(logseqDir, "journals"), 0o755))

	cachePath := filepath.Join(tmpDir, "cache.json")
	stateDBPath := filepath.Join(tmpDir, "state.db")

	cfg := &config.Config{
		GranolaCachePath: cachePath,
		LogseqBasePath:   logseqDir,
		StateDBPath:      stateDBPath,
		UserEmail:        "test@example.com",
		UserName:         "Test User",
		MinAgeSeconds:    0,
	}

	// Create a cache with a deleted document
	baseTime := time.Date(2025, 1, 28, 10, 0, 0, 0, time.UTC)
	deletedTime := baseTime.Add(2 * time.Hour)

	// Build cache with deleted document manually
	innerState := map[string]interface{}{
		"state": map[string]interface{}{
			"documents": map[string]interface{}{
				"deleted-doc": map[string]interface{}{
					"id":         "deleted-doc",
					"title":      "Deleted Meeting",
					"created_at": baseTime.Format(time.RFC3339),
					"updated_at": baseTime.Add(time.Hour).Format(time.RFC3339),
					"deleted_at": deletedTime.Format(time.RFC3339),
					"type":       "meeting",
					"google_calendar_event": map[string]interface{}{
						"id": "event-deleted",
						"attendees": []map[string]interface{}{
							{"email": "test@example.com", "self": true},
						},
					},
				},
			},
			"documentPanels": map[string]interface{}{},
		},
	}

	innerJSON, _ := json.Marshal(innerState)
	outer := map[string]interface{}{
		"cache":   string(innerJSON),
		"version": 3,
	}
	outerJSON, _ := json.Marshal(outer)

	writeCache(t, cachePath, string(outerJSON))

	store, err := state.NewStore(stateDBPath)
	require.NoError(t, err)
	defer func() { _ = store.Close() }()

	syncer := NewSyncer(cfg, store)
	result, err := syncer.Sync(nil, false)
	require.NoError(t, err)

	// Deleted document should be skipped
	assert.Equal(t, 0, result.NewMeetings)
	assert.Equal(t, 0, result.UpdatedMeetings)

	// Verify no page was created
	pagePath := filepath.Join(logseqDir, "pages", "meetings___2025-01-28 Deleted Meeting.md")
	_, err = os.Stat(pagePath)
	assert.True(t, os.IsNotExist(err), "Expected page for deleted meeting to NOT be created")
}

func TestSyncE2E_DryRun(t *testing.T) {
	tmpDir := t.TempDir()
	logseqDir := filepath.Join(tmpDir, "logseq")
	require.NoError(t, os.MkdirAll(filepath.Join(logseqDir, "pages"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(logseqDir, "journals"), 0o755))

	cachePath := filepath.Join(tmpDir, "cache.json")
	stateDBPath := filepath.Join(tmpDir, "state.db")

	cfg := &config.Config{
		GranolaCachePath: cachePath,
		LogseqBasePath:   logseqDir,
		StateDBPath:      stateDBPath,
		UserEmail:        "test@example.com",
		UserName:         "Test User",
		MinAgeSeconds:    0,
	}

	writeCache(t, cachePath, makeCache([]testDoc{
		makeDocument("doc1", "Dry Run Meeting", "test@example.com", "Test notes"),
	}))

	store, err := state.NewStore(stateDBPath)
	require.NoError(t, err)
	defer func() { _ = store.Close() }()

	syncer := NewSyncer(cfg, store)
	result, err := syncer.Sync(nil, true) // dryRun = true
	require.NoError(t, err)

	assert.Equal(t, 1, result.NewMeetings)

	// Verify NO files were actually created
	pagePath := filepath.Join(logseqDir, "pages", "meetings___2025-01-28 Dry Run Meeting.md")
	_, err = os.Stat(pagePath)
	assert.True(t, os.IsNotExist(err), "Expected NO page to be created during dry run")

	journalPath := filepath.Join(logseqDir, "journals", "2025_01_28.md")
	_, err = os.Stat(journalPath)
	assert.True(t, os.IsNotExist(err), "Expected NO journal to be created during dry run")
}
