package state

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type StoreSuite struct {
	suite.Suite
	store *Store
}

func TestStoreSuite(t *testing.T) {
	suite.Run(t, new(StoreSuite))
}

func (s *StoreSuite) SetupTest() {
	var err error
	s.store, err = NewStore(":memory:")
	s.Require().NoError(err)
}

func (s *StoreSuite) TearDownTest() {
	if s.store != nil {
		_ = s.store.Close()
	}
}

func (s *StoreSuite) TestNewStore() {
	// Already tested in SetupTest, but let's verify tables exist
	store, err := NewStore(":memory:")
	s.NoError(err)
	s.NotNil(store)
	defer func() { _ = store.Close() }()

	// Verify we can query the table
	_, err = store.GetSyncedDocument("nonexistent")
	s.NoError(err)
}

func (s *StoreSuite) TestMarkSyncedAndGetSyncedDocument() {
	now := time.Now().Truncate(time.Second)
	updatedAt := now.Add(-time.Hour)

	doc := &SyncedDocument{
		ID:               "test-doc-1",
		Title:            "Test Meeting",
		SyncedAt:         now,
		GranolaUpdatedAt: &updatedAt,
		LogseqPagePath:   "/pages/test-meeting.md",
		ContentHash:      "abc123",
	}

	// Insert
	err := s.store.MarkSynced(doc)
	s.NoError(err)

	// Retrieve
	retrieved, err := s.store.GetSyncedDocument("test-doc-1")
	s.NoError(err)
	s.NotNil(retrieved)
	s.Equal(doc.ID, retrieved.ID)
	s.Equal(doc.Title, retrieved.Title)
	s.Equal(doc.LogseqPagePath, retrieved.LogseqPagePath)
	s.Equal(doc.ContentHash, retrieved.ContentHash)
	s.NotNil(retrieved.GranolaUpdatedAt)
}

func (s *StoreSuite) TestMarkSyncedUpsert() {
	now := time.Now().Truncate(time.Second)

	// Insert initial
	doc := &SyncedDocument{
		ID:          "test-doc-1",
		Title:       "Original Title",
		SyncedAt:    now,
		ContentHash: "hash1",
	}
	s.Require().NoError(s.store.MarkSynced(doc))

	// Update with same ID
	doc.Title = "Updated Title"
	doc.ContentHash = "hash2"
	s.Require().NoError(s.store.MarkSynced(doc))

	// Verify update
	retrieved, err := s.store.GetSyncedDocument("test-doc-1")
	s.NoError(err)
	s.Equal("Updated Title", retrieved.Title)
	s.Equal("hash2", retrieved.ContentHash)
}

func (s *StoreSuite) TestGetSyncedDocumentNotFound() {
	doc, err := s.store.GetSyncedDocument("nonexistent")
	s.NoError(err)
	s.Nil(doc)
}

func (s *StoreSuite) TestNeedsUpdate() {
	t1 := time.Now().Truncate(time.Second)
	t2 := t1.Add(time.Hour)

	// Helper to seed a document
	seedDoc := func(id string, updatedAt time.Time, hash string) {
		doc := &SyncedDocument{
			ID:               id,
			Title:            "Test",
			SyncedAt:         time.Now(),
			GranolaUpdatedAt: &updatedAt,
			ContentHash:      hash,
		}
		s.Require().NoError(s.store.MarkSynced(doc))
	}

	tests := []struct {
		name    string
		setup   func()
		id      string
		updated time.Time
		hash    string
		want    bool
	}{
		{
			name:    "new_document",
			setup:   nil,
			id:      "new-doc",
			updated: t1,
			hash:    "hash",
			want:    true,
		},
		{
			name:    "unchanged",
			setup:   func() { seedDoc("doc-1", t1, "abc") },
			id:      "doc-1",
			updated: t1,
			hash:    "abc",
			want:    false,
		},
		{
			name:    "hash_changed",
			setup:   func() { seedDoc("doc-2", t1, "abc") },
			id:      "doc-2",
			updated: t1,
			hash:    "xyz",
			want:    true,
		},
		{
			name:    "time_changed",
			setup:   func() { seedDoc("doc-3", t1, "abc") },
			id:      "doc-3",
			updated: t2,
			hash:    "abc",
			want:    true,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			// Fresh store for each test
			store, err := NewStore(":memory:")
			s.Require().NoError(err)
			defer func() { _ = store.Close() }()

			// Temporarily swap store
			origStore := s.store
			s.store = store

			if tt.setup != nil {
				tt.setup()
			}

			needs, err := s.store.NeedsUpdate(tt.id, tt.updated, tt.hash)
			s.NoError(err)
			s.Equal(tt.want, needs)

			s.store = origStore
		})
	}
}
