package sync

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/philrhinehart/granola-sync/internal/config"
	"github.com/philrhinehart/granola-sync/internal/granola"
	"github.com/philrhinehart/granola-sync/internal/logseq"
	"github.com/philrhinehart/granola-sync/internal/state"
)

// Syncer orchestrates syncing between Granola and Logseq
type Syncer struct {
	cfg    *config.Config
	store  *state.Store
	writer *logseq.Writer
}

// SyncResult contains the result of a sync operation
type SyncResult struct {
	NewMeetings     int
	UpdatedMeetings int
	NewJournals     int
	Errors          []error
}

// NewSyncer creates a new syncer
func NewSyncer(cfg *config.Config, store *state.Store) *Syncer {
	return &Syncer{
		cfg:    cfg,
		store:  store,
		writer: logseq.NewWriter(cfg.LogseqBasePath, cfg.UserName),
	}
}

// Sync performs a full sync of all documents
func (s *Syncer) Sync(since *time.Time, dryRun bool) (*SyncResult, error) {
	docs, err := granola.ParseCache(s.cfg.GranolaCachePath)
	if err != nil {
		return nil, fmt.Errorf("parsing cache: %w", err)
	}

	result := &SyncResult{}
	minAge := time.Duration(s.cfg.MinAgeSeconds) * time.Second

	// Sort documents by meeting date for consistent ordering
	sortedDocs := sortDocumentsByDate(docs)

	for _, doc := range sortedDocs {
		if err := s.processDocument(doc, since, minAge, dryRun, result); err != nil {
			slog.Error("failed to process document", "id", doc.ID, "title", doc.Title, "error", err)
			result.Errors = append(result.Errors, fmt.Errorf("doc %s: %w", doc.ID, err))
		}
	}

	return result, nil
}

func (s *Syncer) processDocument(doc *granola.Document, since *time.Time, minAge time.Duration, dryRun bool, result *SyncResult) error {
	// Skip deleted documents
	if doc.IsDeleted() {
		slog.Debug("skipping deleted document", "id", doc.ID, "title", doc.Title)
		return nil
	}

	// Skip meetings the user wasn't invited to
	if !doc.IsUserAttendee(s.cfg.UserEmail) {
		slog.Debug("skipping meeting user wasn't invited to", "id", doc.ID, "title", doc.Title)
		return nil
	}

	// Skip documents that are too new (might still be in progress)
	if !dryRun && time.Since(doc.UpdatedAt) < minAge {
		slog.Debug("skipping recent document", "id", doc.ID, "title", doc.Title, "age", time.Since(doc.UpdatedAt))
		return nil
	}

	// Apply since filter
	meetingDate := doc.GetMeetingDate()
	if since != nil && meetingDate.Before(*since) {
		slog.Debug("skipping document before since date", "id", doc.ID, "title", doc.Title, "date", meetingDate)
		return nil
	}

	// Calculate content hash for change detection
	contentHash := hashContent(doc)

	// Check if this document needs syncing
	needsUpdate, err := s.store.NeedsUpdate(doc.ID, doc.UpdatedAt, contentHash)
	if err != nil {
		return fmt.Errorf("checking update status: %w", err)
	}

	if !needsUpdate {
		slog.Debug("document already synced", "id", doc.ID, "title", doc.Title)
		return nil
	}

	// Check if this is new or updated
	existing, err := s.store.GetSyncedDocument(doc.ID)
	if err != nil {
		return fmt.Errorf("getting existing document: %w", err)
	}

	isNew := existing == nil

	if dryRun {
		return s.dryRunDocument(doc, isNew, result)
	}

	return s.syncDocument(doc, contentHash, isNew, result)
}

func (s *Syncer) dryRunDocument(doc *granola.Document, isNew bool, result *SyncResult) error {
	pagePath, pageContent := s.writer.DryRunMeetingPage(doc)
	journalPath, journalContent, wouldAddJournal := s.writer.DryRunJournalEntry(doc)

	action := "UPDATE"
	if isNew {
		action = "NEW"
		result.NewMeetings++
	} else {
		result.UpdatedMeetings++
	}

	fmt.Printf("\n[%s] %s\n", action, doc.Title)
	fmt.Printf("  Meeting date: %s\n", doc.GetMeetingDate().Format("2006-01-02 15:04"))
	fmt.Printf("  Page: %s\n", pagePath)
	fmt.Printf("  Content preview:\n%s\n", truncate(pageContent, 500))

	if wouldAddJournal {
		result.NewJournals++
		fmt.Printf("  Journal: %s\n", journalPath)
		fmt.Printf("  Entry: %s", journalContent)
	} else {
		fmt.Printf("  Journal: (entry already exists)\n")
	}

	return nil
}

func (s *Syncer) syncDocument(doc *granola.Document, contentHash string, isNew bool, result *SyncResult) error {
	// Write meeting page
	pagePath, err := s.writer.WriteMeetingPage(doc)
	if err != nil {
		return fmt.Errorf("writing meeting page: %w", err)
	}

	if isNew {
		result.NewMeetings++
		slog.Info("created meeting page", "title", doc.Title, "path", pagePath)
	} else {
		result.UpdatedMeetings++
		slog.Info("updated meeting page", "title", doc.Title, "path", pagePath)
	}

	// Add journal entry if this is new
	if isNew {
		added, err := s.writer.AppendJournalEntry(doc)
		if err != nil {
			return fmt.Errorf("appending journal entry: %w", err)
		}
		if added {
			result.NewJournals++
			slog.Info("added journal entry", "title", doc.Title)
		}
	}

	// Mark as synced
	syncedDoc := &state.SyncedDocument{
		ID:               doc.ID,
		Title:            doc.Title,
		SyncedAt:         time.Now(),
		GranolaUpdatedAt: &doc.UpdatedAt,
		LogseqPagePath:   pagePath,
		ContentHash:      contentHash,
	}

	if err := s.store.MarkSynced(syncedDoc); err != nil {
		return fmt.Errorf("marking synced: %w", err)
	}

	return nil
}

func sortDocumentsByDate(docs map[string]*granola.Document) []*granola.Document {
	sorted := make([]*granola.Document, 0, len(docs))
	for _, doc := range docs {
		sorted = append(sorted, doc)
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].GetMeetingDate().Before(sorted[j].GetMeetingDate())
	})
	return sorted
}

func hashContent(doc *granola.Document) string {
	h := sha256.New()
	h.Write([]byte(doc.Title))
	if doc.NotesMarkdown != nil {
		h.Write([]byte(*doc.NotesMarkdown))
	}
	if doc.NotesPlain != nil {
		h.Write([]byte(*doc.NotesPlain))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
