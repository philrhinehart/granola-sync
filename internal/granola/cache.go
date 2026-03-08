package granola

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// CacheFileRaw is used for initial unmarshalling to detect the cache format.
// In v3, Cache is a JSON string (double-encoded). In v4, Cache is a JSON object.
type CacheFileRaw struct {
	Cache   json.RawMessage `json:"cache"`
	Version int             `json:"version"`
}

// CacheState represents the inner JSON structure
type CacheState struct {
	State struct {
		Documents      map[string]*Document                 `json:"documents"`
		DocumentPanels map[string]map[string]*DocumentPanel `json:"documentPanels"`
	} `json:"state"`
}

// DocumentPanel represents a panel containing notes/summary for a document (v3 format)
type DocumentPanel struct {
	ID               string      `json:"id"`
	DocumentID       string      `json:"document_id"`
	Title            string      `json:"title"`
	Content          interface{} `json:"content"`
	ContentUpdatedAt string      `json:"content_updated_at"`
}

// FindCacheFile finds the newest cache-v*.json file in the given directory.
// Returns the full path to the cache file, or an error if none found.
func FindCacheFile(dir string) (string, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "cache-v*.json"))
	if err != nil {
		return "", fmt.Errorf("searching for cache files: %w", err)
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("no cache-v*.json files found in %s", dir)
	}
	// Sort lexically — cache-v4 > cache-v3 — and pick the last (highest version)
	sort.Strings(matches)
	return matches[len(matches)-1], nil
}

// ParseCache parses the Granola cache file
func ParseCache(path string) (map[string]*Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading cache file: %w", err)
	}

	return ParseCacheData(data)
}

// ParseCacheData parses the cache data bytes.
// Supports both v3 (double-encoded string) and v4 (direct object) cache formats.
func ParseCacheData(data []byte) (map[string]*Document, error) {
	var raw CacheFileRaw
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing outer JSON: %w", err)
	}

	// Determine if cache is a string (v3) or object (v4)
	var innerData []byte
	if len(raw.Cache) > 0 && raw.Cache[0] == '"' {
		// v3: double-encoded JSON string
		var s string
		if err := json.Unmarshal(raw.Cache, &s); err != nil {
			return nil, fmt.Errorf("parsing inner JSON string: %w", err)
		}
		innerData = []byte(s)
	} else {
		// v4: direct JSON object
		innerData = raw.Cache
	}

	var inner CacheState
	if err := json.Unmarshal(innerData, &inner); err != nil {
		return nil, fmt.Errorf("parsing inner JSON: %w", err)
	}

	// Extract notes from documentPanels (v3) or inline notes content (v4)
	for docID, doc := range inner.State.Documents {
		populateNotes(doc, inner.State.DocumentPanels[docID])
	}

	return inner.State.Documents, nil
}

// populateNotes sets NotesMarkdown on a document from panels (v3) or inline notes (v4).
func populateNotes(doc *Document, panels map[string]*DocumentPanel) {
	if doc.NotesMarkdown != nil && *doc.NotesMarkdown != "" {
		return
	}

	if md := bestSummaryFromPanels(panels); md != "" {
		doc.NotesMarkdown = &md
		return
	}

	if doc.Notes != nil {
		if md := ExtractMarkdownFromContent(doc.Notes); md != "" {
			doc.NotesMarkdown = &md
		}
	}
}

// BestSummaryFromPanels picks the most recently updated "Summary" panel and returns its markdown.
func BestSummaryFromPanels(panels []*DocumentPanel) string {
	var bestContent string
	var bestTime time.Time

	for _, panel := range panels {
		if panel.Title != "Summary" || panel.Content == nil {
			continue
		}
		md := ExtractMarkdownFromContent(panel.Content)
		if md == "" {
			continue
		}
		ts, err := time.Parse(time.RFC3339, panel.ContentUpdatedAt)
		if err != nil {
			if bestContent == "" {
				bestContent = md
			}
			continue
		}
		if bestTime.IsZero() || ts.After(bestTime) {
			bestContent = md
			bestTime = ts
		}
	}

	return bestContent
}

// bestSummaryFromPanels is the map-keyed variant used by the v3 cache parser.
func bestSummaryFromPanels(panels map[string]*DocumentPanel) string {
	slice := make([]*DocumentPanel, 0, len(panels))
	for _, p := range panels {
		slice = append(slice, p)
	}
	return BestSummaryFromPanels(slice)
}

// ExtractMarkdownFromContent converts the rich text content structure to Logseq-formatted bullets
func ExtractMarkdownFromContent(content interface{}) string {
	contentMap, ok := content.(map[string]interface{})
	if !ok {
		return ""
	}

	contentArr, ok := contentMap["content"].([]interface{})
	if !ok {
		return ""
	}

	var result string
	for _, item := range contentArr {
		result += extractNodeToLogseq(item, 0)
	}
	return result
}

// extractNodeToLogseq recursively extracts content as Logseq-formatted bullets
// depth 0 = top level under **Notes** (will get 2 tabs added by format.go)
func extractNodeToLogseq(node interface{}, depth int) string {
	nodeMap, ok := node.(map[string]interface{})
	if !ok {
		return ""
	}

	nodeType, _ := nodeMap["type"].(string)
	indent := strings.Repeat("\t", depth)

	switch nodeType {
	case "heading":
		return extractHeadingNode(nodeMap, indent)
	case "paragraph":
		return extractParagraphNode(nodeMap, indent)
	case "bulletList", "orderedList":
		return extractListNode(nodeMap, depth)
	case "listItem":
		return extractListItemNode(nodeMap, indent, depth)
	case "text":
		if text, ok := nodeMap["text"].(string); ok {
			return text
		}
	}

	return ""
}

func extractHeadingNode(nodeMap map[string]interface{}, indent string) string {
	text := extractTextFromNode(nodeMap)
	if text != "" {
		return indent + "- **" + text + "**\n"
	}
	return ""
}

func extractParagraphNode(nodeMap map[string]interface{}, indent string) string {
	text := extractTextFromNode(nodeMap)
	if text != "" {
		return indent + "- " + text + "\n"
	}
	return ""
}

func extractListNode(nodeMap map[string]interface{}, depth int) string {
	content, ok := nodeMap["content"].([]interface{})
	if !ok {
		return ""
	}
	var result string
	for _, child := range content {
		result += extractNodeToLogseq(child, depth)
	}
	return result
}

func extractListItemNode(nodeMap map[string]interface{}, indent string, depth int) string {
	content, ok := nodeMap["content"].([]interface{})
	if !ok {
		return ""
	}

	var result string
	for i, child := range content {
		childMap, _ := child.(map[string]interface{})
		childType, _ := childMap["type"].(string)

		switch childType {
		case "paragraph":
			text := extractTextFromNode(childMap)
			if text != "" {
				result += indent + "- " + text + "\n"
			}
		case "bulletList", "orderedList":
			if i == 0 {
				result += indent + "- \n"
			}
			result += extractNodeToLogseq(child, depth+1)
		}
	}
	return result
}

// extractTextFromNode extracts all text content from a node's children
func extractTextFromNode(nodeMap map[string]interface{}) string {
	content, ok := nodeMap["content"].([]interface{})
	if !ok {
		return ""
	}

	var texts []string
	for _, child := range content {
		childMap, ok := child.(map[string]interface{})
		if !ok {
			continue
		}
		if text, ok := childMap["text"].(string); ok {
			texts = append(texts, text)
		}
	}
	return strings.Join(texts, "")
}
