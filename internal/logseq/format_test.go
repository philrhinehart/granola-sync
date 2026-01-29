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
		{"removes day suffix", "Nova War Room (Tuesday)", "Nova War Room"},
		{"removes another day suffix", "Nova War Room (Thursday)", "Nova War Room"},
		{"removes day suffix with dash", "Weekly Sync - Monday", "Weekly Sync"},
		{"removes date suffix", "Team Meeting 2024-01-15", "Team Meeting"},
		{"keeps exclamation", "AngelList All Hands!", "AngelList All Hands!"},
		{"keeps slash", "Phil / Ashok", "Phil / Ashok"},
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
		- Tibi: Complete console PR
		- Phil: Continue investigating Nova 404 errors
		- Ashok: Test something`,
			userName: "Phil",
			want: `		- **Action Items**
		- Tibi: Complete console PR
		- TODO Phil: Continue investigating Nova 404 errors
		- Ashok: Test something`,
		},
		{
			name: "does not mark other users",
			content: `		- **Action Items**
		- Tibi: Complete console PR
		- Ashok: Test something`,
			userName: "Phil",
			want: `		- **Action Items**
		- Tibi: Complete console PR
		- Ashok: Test something`,
		},
		{
			name: "handles empty userName",
			content: `		- **Action Items**
		- Phil: Do something`,
			userName: "",
			want: `		- **Action Items**
		- Phil: Do something`,
		},
		{
			name: "stops at next heading",
			content: `		- **Action Items**
		- Phil: First item
		- **Next Section**
		- Phil: Should not be marked`,
			userName: "Phil",
			want: `		- **Action Items**
		- TODO Phil: First item
		- **Next Section**
		- Phil: Should not be marked`,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			got := MarkUserTodos(tt.content, tt.userName)
			s.Equal(tt.want, got)
		})
	}
}
