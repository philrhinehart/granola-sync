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
		Documents      map[string]*Document                    `json:"documents"`
		DocumentPanels map[string]map[string]*DocumentPanel    `json:"documentPanels"`
	} `json:"state"`
}

// DocumentPanel represents a panel containing notes/summary for a document
type DocumentPanel struct {
	ID         string      `json:"id"`
	DocumentID string      `json:"document_id"`
	Title      string      `json:"title"`
	Content    interface{} `json:"content"`
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
	for docID, doc := range inner.State.Documents {
		if panels, ok := inner.State.DocumentPanels[docID]; ok {
			for _, panel := range panels {
				if panel.Title == "Summary" && panel.Content != nil {
					md := extractMarkdownFromContent(panel.Content)
					if md != "" {
						doc.NotesMarkdown = &md
					}
					break
				}
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
	var result string
	indent := strings.Repeat("\t", depth)

	switch nodeType {
	case "heading":
		text := extractTextFromNode(nodeMap)
		if text != "" {
			// Headings become bold bullets
			result = indent + "- **" + text + "**\n"
		}

	case "paragraph":
		text := extractTextFromNode(nodeMap)
		if text != "" {
			result = indent + "- " + text + "\n"
		}

	case "bulletList", "orderedList":
		if content, ok := nodeMap["content"].([]interface{}); ok {
			for _, child := range content {
				result += extractNodeToLogseq(child, depth)
			}
		}

	case "listItem":
		if content, ok := nodeMap["content"].([]interface{}); ok {
			// First child is usually the paragraph with the item text
			// Subsequent children are nested lists
			for i, child := range content {
				childMap, _ := child.(map[string]interface{})
				childType, _ := childMap["type"].(string)

				if childType == "paragraph" {
					text := extractTextFromNode(childMap)
					if text != "" {
						result += indent + "- " + text + "\n"
					}
				} else if childType == "bulletList" || childType == "orderedList" {
					// Nested list - increase depth
					// If this is the only content (no paragraph before), add empty bullet
					if i == 0 {
						result += indent + "- \n"
					}
					result += extractNodeToLogseq(child, depth+1)
				}
			}
		}

	case "text":
		if text, ok := nodeMap["text"].(string); ok {
			result = text
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
