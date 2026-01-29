package state

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// Store manages the sync state in SQLite
type Store struct {
	db *sql.DB
}

// SyncedDocument represents a synced document record
type SyncedDocument struct {
	ID               string
	Title            string
	SyncedAt         time.Time
	GranolaUpdatedAt *time.Time
	LogseqPagePath   string
	ContentHash      string
}

// NewStore creates a new state store
func NewStore(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	store := &Store{db: db}
	if err := store.migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrating database: %w", err)
	}

	return store, nil
}

// Close closes the database connection
func (s *Store) Close() error {
	return s.db.Close()
}

// GetSyncedDocument retrieves a synced document by ID
func (s *Store) GetSyncedDocument(id string) (*SyncedDocument, error) {
	var doc SyncedDocument
	var granolaUpdatedAt sql.NullTime

	err := s.db.QueryRow(`
		SELECT id, title, synced_at, granola_updated_at, logseq_page_path, content_hash
		FROM synced_documents WHERE id = ?
	`, id).Scan(&doc.ID, &doc.Title, &doc.SyncedAt, &granolaUpdatedAt, &doc.LogseqPagePath, &doc.ContentHash)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if granolaUpdatedAt.Valid {
		doc.GranolaUpdatedAt = &granolaUpdatedAt.Time
	}

	return &doc, nil
}

// MarkSynced records that a document has been synced
func (s *Store) MarkSynced(doc *SyncedDocument) error {
	_, err := s.db.Exec(`
		INSERT INTO synced_documents (id, title, synced_at, granola_updated_at, logseq_page_path, content_hash)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			title = excluded.title,
			synced_at = excluded.synced_at,
			granola_updated_at = excluded.granola_updated_at,
			logseq_page_path = excluded.logseq_page_path,
			content_hash = excluded.content_hash
	`, doc.ID, doc.Title, doc.SyncedAt, doc.GranolaUpdatedAt, doc.LogseqPagePath, doc.ContentHash)
	return err
}

// NeedsUpdate checks if a document needs to be re-synced
func (s *Store) NeedsUpdate(id string, currentUpdatedAt time.Time, contentHash string) (bool, error) {
	doc, err := s.GetSyncedDocument(id)
	if err != nil {
		return false, err
	}

	// New document
	if doc == nil {
		return true, nil
	}

	// Check if content changed via hash
	if doc.ContentHash != contentHash {
		return true, nil
	}

	// Check if Granola's updated_at changed
	if doc.GranolaUpdatedAt == nil || !doc.GranolaUpdatedAt.Equal(currentUpdatedAt) {
		return true, nil
	}

	return false, nil
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS synced_documents (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			synced_at TIMESTAMP NOT NULL,
			granola_updated_at TIMESTAMP,
			logseq_page_path TEXT,
			content_hash TEXT
		)
	`)
	return err
}
