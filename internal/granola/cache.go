package granola

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// CacheFile represents the outer structure of the Granola cache
type CacheFile struct {
	Cache   string `json:"cache"`
	Version int    `json:"version"`
}

// CacheState represents the inner JSON structure
type CacheState struct {
	State struct {
		Documents      map[string]*Document                 `json:"documents"`
		DocumentPanels map[string]map[string]*DocumentPanel `json:"documentPanels"`
	} `json:"state"`
}

// DocumentPanel represents a panel containing notes/summary for a document
type DocumentPanel struct {
	ID               string      `json:"id"`
	DocumentID       string      `json:"document_id"`
	Title            string      `json:"title"`
	Content          interface{} `json:"content"`
	ContentUpdatedAt string      `json:"content_updated_at"`
}

// ParseCache parses the double-encoded Granola cache file
func ParseCache(path string) (map[string]*Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading cache file: %w", err)
	}

	return ParseCacheData(data)
}

// ParseCacheData parses the cache data bytes
func ParseCacheData(data []byte) (map[string]*Document, error) {
	// First decode: get the outer wrapper
	var outer CacheFile
	if err := json.Unmarshal(data, &outer); err != nil {
		return nil, fmt.Errorf("parsing outer JSON: %w", err)
	}

	// Second decode: parse the stringified inner JSON
	var inner CacheState
	if err := json.Unmarshal([]byte(outer.Cache), &inner); err != nil {
		return nil, fmt.Errorf("parsing inner JSON: %w", err)
	}

	// Extract notes from documentPanels and populate documents
	// Use the most recently updated Summary panel that has actual content
	for docID, doc := range inner.State.Documents {
		if panels, ok := inner.State.DocumentPanels[docID]; ok {
			var bestPanel *DocumentPanel
			var bestContent string
			var bestTimestamp string

			for _, panel := range panels {
				if panel.Title == "Summary" && panel.Content != nil {
					md := extractMarkdownFromContent(panel.Content)
					if md != "" {
						// Use this panel if it's newer than our current best
						if bestPanel == nil || panel.ContentUpdatedAt > bestTimestamp {
							bestPanel = panel
							bestContent = md
							bestTimestamp = panel.ContentUpdatedAt
						}
					}
				}
			}

			if bestContent != "" {
				doc.NotesMarkdown = &bestContent
			}
		}
	}

	return inner.State.Documents, nil
}

// extractMarkdownFromContent converts the rich text content structure to Logseq-formatted bullets
func extractMarkdownFromContent(content interface{}) string {
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
