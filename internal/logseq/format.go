package logseq

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/philrhinehart/granola-sync/internal/granola"
)

// FormatMeetingPage formats a Granola document as a Logseq meeting page
func FormatMeetingPage(doc *granola.Document) string {
	var sb strings.Builder

	meetingDate := doc.GetMeetingDate()
	dateStr := meetingDate.Format("2006-01-02")
	startTime, endTime, tz := doc.GetMeetingTimeRange()
	attendees := doc.GetAttendeeNames()

	// Title
	sb.WriteString(fmt.Sprintf("- %s\n", doc.Title))

	// Properties
	sb.WriteString(fmt.Sprintf("  meeting-date:: [[%s]]\n", dateStr))
	if startTime != "" && endTime != "" {
		timeStr := fmt.Sprintf("%s - %s", startTime, endTime)
		if tz != "" {
			timeStr += fmt.Sprintf(" (%s)", shortTimezone(tz))
		}
		sb.WriteString(fmt.Sprintf("  meeting-time:: %s\n", timeStr))
	}
	sb.WriteString(fmt.Sprintf("  granola-id:: %s\n", doc.ID))

	// Build tags list
	var tags []string
	tags = append(tags, "Granola Notes")
	if tag := meetingTag(doc.Title); tag != "" {
		tags = append(tags, tag)
	}
	var tagLinks []string
	for _, t := range tags {
		tagLinks = append(tagLinks, fmt.Sprintf("[[%s]]", t))
	}
	sb.WriteString(fmt.Sprintf("  tags:: %s\n", strings.Join(tagLinks, ", ")))

	// Attendees
	if len(attendees) > 0 {
		sb.WriteString("\t- **Attendees**\n")
		for _, name := range attendees {
			sb.WriteString(fmt.Sprintf("\t\t- [[@%s]]\n", name))
		}
	}

	// Notes
	sb.WriteString("\t- **Notes**\n")
	if doc.NotesMarkdown != nil && *doc.NotesMarkdown != "" {
		// Notes from documentPanels are already in Logseq format, just need base indent
		notes := indentLogseqContent(*doc.NotesMarkdown, 2)
		sb.WriteString(notes)
	} else if doc.NotesPlain != nil && *doc.NotesPlain != "" {
		notes := convertPlainTextToLogseq(*doc.NotesPlain)
		sb.WriteString(notes)
	} else {
		sb.WriteString("\t\t- (No notes taken)\n")
	}

	return sb.String()
}

// FormatJournalEntry formats a journal reference for a meeting
func FormatJournalEntry(doc *granola.Document) string {
	meetingDate := doc.GetMeetingDate()
	dateStr := meetingDate.Format("2006-01-02")
	startTime, endTime, tz := doc.GetMeetingTimeRange()
	attendees := doc.GetAttendeeNames()

	pageName := fmt.Sprintf("meetings/%s %s", dateStr, sanitizeTitle(doc.Title))

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("- [[%s]]\n", pageName))

	// Add time and attendees on sub-bullet
	var details []string
	if startTime != "" && endTime != "" {
		timeStr := fmt.Sprintf("%s - %s", startTime, endTime)
		if tz != "" {
			timeStr += fmt.Sprintf(" (%s)", shortTimezone(tz))
		}
		details = append(details, timeStr)
	}
	if len(attendees) > 0 {
		var attendeeLinks []string
		for _, name := range attendees {
			attendeeLinks = append(attendeeLinks, fmt.Sprintf("[[@%s]]", name))
		}
		details = append(details, "with "+strings.Join(attendeeLinks, ", "))
	}
	if len(details) > 0 {
		sb.WriteString(fmt.Sprintf("\t- %s\n", strings.Join(details, " ")))
	}

	return sb.String()
}

// convertPlainTextToLogseq converts plain text to Logseq bullet format
func convertPlainTextToLogseq(text string) string {
	lines := strings.Split(text, "\n")
	var sb strings.Builder

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		sb.WriteString("\t\t- " + trimmed + "\n")
	}

	return sb.String()
}

// indentLogseqContent adds base indentation to pre-formatted Logseq content
func indentLogseqContent(content string, baseIndent int) string {
	lines := strings.Split(content, "\n")
	var sb strings.Builder
	indent := strings.Repeat("\t", baseIndent)

	for _, line := range lines {
		if line == "" {
			continue
		}
		sb.WriteString(indent + line + "\n")
	}

	return sb.String()
}

// todoSectionNames contains variations of section headers that contain action items
var todoSectionNames = []string{
	"Action Items",
	"Next Steps",
	"Immediate Tasks",
	"To Do",
	"To-Do",
	"TODO",
	"Tasks",
	"Follow-ups",
	"Follow Ups",
	"Followups",
}

// isTodoSectionHeader checks if a line contains a todo section header
func isTodoSectionHeader(line string) bool {
	lineLower := strings.ToLower(line)
	for _, name := range todoSectionNames {
		if strings.Contains(lineLower, strings.ToLower(name)) && strings.Contains(line, "**") {
			return true
		}
	}
	return false
}

// MarkUserTodos adds TODO markers to action items assigned to the user
func MarkUserTodos(content string, userName string) string {
	if userName == "" {
		return content
	}

	lines := strings.Split(content, "\n")
	var sb strings.Builder
	inActionItems := false

	for _, line := range lines {
		// Check if we're entering a todo section
		if isTodoSectionHeader(line) {
			inActionItems = true
			sb.WriteString(line + "\n")
			continue
		}

		// Check if we're leaving the section (new heading)
		if inActionItems && strings.Contains(line, "**") && !isTodoSectionHeader(line) {
			inActionItems = false
		}

		// Mark user's action items with TODO
		if inActionItems && strings.Contains(line, "- "+userName+":") {
			line = strings.Replace(line, "- "+userName+":", "- TODO "+userName+":", 1)
		}

		sb.WriteString(line + "\n")
	}

	return strings.TrimSuffix(sb.String(), "\n")
}

// sanitizeTitle removes characters that aren't safe for filenames
func sanitizeTitle(title string) string {
	// Replace slashes and other problematic chars
	unsafe := regexp.MustCompile(`[/\\:*?"<>|]`)
	result := unsafe.ReplaceAllString(title, "-")

	// Collapse multiple dashes
	result = regexp.MustCompile(`-+`).ReplaceAllString(result, "-")

	// Trim leading/trailing dashes and spaces
	result = strings.Trim(result, "- ")

	return result
}

// GetPageFilename returns the filename for a meeting page
func GetPageFilename(doc *granola.Document) string {
	meetingDate := doc.GetMeetingDate()
	dateStr := meetingDate.Format("2006-01-02")
	return fmt.Sprintf("meetings___%s %s.md", dateStr, sanitizeTitle(doc.Title))
}

// GetJournalFilename returns the filename for a journal entry
func GetJournalFilename(doc *granola.Document) string {
	meetingDate := doc.GetMeetingDate()
	return meetingDate.Format("2006_01_02") + ".md"
}

// shortTimezone converts a timezone name to a short abbreviation
func shortTimezone(tz string) string {
	// Common timezone mappings
	abbrevs := map[string]string{
		"America/Los_Angeles": "PST",
		"America/New_York":    "EST",
		"America/Chicago":     "CST",
		"America/Denver":      "MST",
		"Europe/London":       "GMT",
		"UTC":                 "UTC",
	}
	if abbrev, ok := abbrevs[tz]; ok {
		return abbrev
	}
	// Return the last part of the timezone (e.g., "Los_Angeles" from "America/Los_Angeles")
	parts := strings.Split(tz, "/")
	return parts[len(parts)-1]
}

// meetingTag extracts a tag from the meeting title
// Returns a cleaned version suitable for use as a Logseq tag
func meetingTag(title string) string {
	if title == "" {
		return ""
	}

	// Remove common patterns that make titles unique but aren't useful for tagging
	tag := title

	// Remove parenthetical day references like "(Tuesday)" first
	parenDayPattern := regexp.MustCompile(`(?i)\s*\(\s*(monday|tuesday|wednesday|thursday|friday|saturday|sunday)\s*\)`)
	tag = parenDayPattern.ReplaceAllString(tag, "")

	// Remove date patterns like "2024-01-15" or "01/15"
	datePattern := regexp.MustCompile(`\s*\d{4}[-/]\d{2}[-/]\d{2}`)
	tag = datePattern.ReplaceAllString(tag, "")
	datePattern2 := regexp.MustCompile(`\s*\d{1,2}[-/]\d{1,2}`)
	tag = datePattern2.ReplaceAllString(tag, "")

	// Remove standalone day names (with word boundaries)
	dayPattern := regexp.MustCompile(`(?i)\b(monday|tuesday|wednesday|thursday|friday|saturday|sunday)\b`)
	tag = dayPattern.ReplaceAllString(tag, "")

	// Remove empty parentheses
	tag = regexp.MustCompile(`\(\s*\)`).ReplaceAllString(tag, "")

	// Clean up whitespace and trailing punctuation
	tag = strings.TrimSpace(tag)
	tag = regexp.MustCompile(`\s+`).ReplaceAllString(tag, " ")
	tag = strings.TrimRight(tag, " -")

	return tag
}
