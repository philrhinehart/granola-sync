package logseq

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/philrhinehart/granola-sync/internal/granola"
)

// Writer handles writing Logseq pages and journal entries
type Writer struct {
	basePath string
	userName string
}

// NewWriter creates a new Logseq writer
func NewWriter(basePath, userName string) *Writer {
	return &Writer{basePath: basePath, userName: userName}
}

// WriteMeetingPage creates or updates a meeting page
func (w *Writer) WriteMeetingPage(doc *granola.Document) (string, error) {
	filename := GetPageFilename(doc)
	pagePath := filepath.Join(w.basePath, "pages", filename)

	content := FormatMeetingPage(doc)
	content = MarkUserTodos(content, w.userName)

	if err := os.WriteFile(pagePath, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("writing meeting page: %w", err)
	}

	return pagePath, nil
}

// AppendJournalEntry adds a meeting reference to the journal
// Returns true if an entry was added, false if it already existed
func (w *Writer) AppendJournalEntry(doc *granola.Document) (bool, error) {
	filename := GetJournalFilename(doc)
	journalPath := filepath.Join(w.basePath, "journals", filename)

	// Read existing content
	existingContent, err := os.ReadFile(journalPath)
	if err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("reading journal: %w", err)
	}

	// Check if entry already exists
	pageName := GetPageName(doc)
	if strings.Contains(string(existingContent), pageName) {
		return false, nil // Entry already exists
	}

	// Format new entry
	entry := FormatJournalEntry(doc)

	// Append to file
	var newContent string
	if len(existingContent) == 0 {
		newContent = entry
	} else {
		// Add newline if needed
		if !strings.HasSuffix(string(existingContent), "\n") {
			newContent = string(existingContent) + "\n" + entry
		} else {
			newContent = string(existingContent) + entry
		}
	}

	if err := os.WriteFile(journalPath, []byte(newContent), 0o644); err != nil {
		return false, fmt.Errorf("writing journal: %w", err)
	}

	return true, nil
}

// DryRunMeetingPage returns what would be written for a meeting page
func (w *Writer) DryRunMeetingPage(doc *granola.Document) (path, content string) {
	filename := GetPageFilename(doc)
	pagePath := filepath.Join(w.basePath, "pages", filename)
	content = FormatMeetingPage(doc)
	content = MarkUserTodos(content, w.userName)
	return pagePath, content
}

// DryRunJournalEntry returns what would be appended to a journal
func (w *Writer) DryRunJournalEntry(doc *granola.Document) (path, content string, wouldAdd bool) {
	filename := GetJournalFilename(doc)
	journalPath := filepath.Join(w.basePath, "journals", filename)

	// Check if entry already exists
	existingContent, err := os.ReadFile(journalPath)
	if err == nil {
		if strings.Contains(string(existingContent), GetPageName(doc)) {
			return journalPath, "", false
		}
	}

	entry := FormatJournalEntry(doc)
	return journalPath, entry, true
}
