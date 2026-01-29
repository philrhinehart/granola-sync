package logseq

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type FormatSuite struct {
	suite.Suite
}

func TestFormatSuite(t *testing.T) {
	suite.Run(t, new(FormatSuite))
}

func (s *FormatSuite) TestMeetingTag() {
	tests := []struct {
		name  string
		title string
		want  string
	}{
		{"simple", "Standup", "Standup"},
		{"removes day suffix", "Project Review (Tuesday)", "Project Review"},
		{"removes another day suffix", "Project Review (Thursday)", "Project Review"},
		{"removes day suffix with dash", "Weekly Sync - Monday", "Weekly Sync"},
		{"removes date suffix", "Team Meeting 2024-01-15", "Team Meeting"},
		{"keeps exclamation", "Company All Hands!", "Company All Hands!"},
		{"keeps slash", "Alice / Carol", "Alice / Carol"},
		{"handles empty", "", ""},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			got := meetingTag(tt.title)
			s.Equal(tt.want, got)
		})
	}
}

func (s *FormatSuite) TestMarkUserTodos() {
	tests := []struct {
		name     string
		content  string
		userName string
		want     string
	}{
		{
			name: "marks user action item with TODO",
			content: `		- **Action Items**
		- Bob: Review the proposal
		- Alice: Update the documentation
		- Carol: Schedule follow-up meeting`,
			userName: "Alice",
			want: `		- **Action Items**
		- Bob: Review the proposal
		- TODO Alice: Update the documentation
		- Carol: Schedule follow-up meeting`,
		},
		{
			name: "does not mark other users",
			content: `		- **Action Items**
		- Bob: Review the proposal
		- Carol: Schedule follow-up meeting`,
			userName: "Alice",
			want: `		- **Action Items**
		- Bob: Review the proposal
		- Carol: Schedule follow-up meeting`,
		},
		{
			name: "handles empty userName",
			content: `		- **Action Items**
		- Alice: Do something`,
			userName: "",
			want: `		- **Action Items**
		- Alice: Do something`,
		},
		{
			name: "stops at next heading",
			content: `		- **Action Items**
		- Alice: First item
		- **Next Section**
		- Alice: Should not be marked`,
			userName: "Alice",
			want: `		- **Action Items**
		- TODO Alice: First item
		- **Next Section**
		- Alice: Should not be marked`,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			got := MarkUserTodos(tt.content, tt.userName)
			s.Equal(tt.want, got)
		})
	}
}
