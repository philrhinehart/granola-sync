package logseq

import (
	"testing"
)

func TestMeetingTag(t *testing.T) {
	tests := []struct {
		title string
		want  string
	}{
		{"Standup", "Standup"},
		{"Nova War Room (Tuesday)", "Nova War Room"},
		{"Nova War Room (Thursday)", "Nova War Room"},
		{"Weekly Sync - Monday", "Weekly Sync"},
		{"Team Meeting 2024-01-15", "Team Meeting"},
		{"AngelList All Hands!", "AngelList All Hands!"},
		{"Phil / Ashok", "Phil / Ashok"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			got := meetingTag(tt.title)
			if got != tt.want {
				t.Errorf("meetingTag(%q) = %q, want %q", tt.title, got, tt.want)
			}
		})
	}
}

func TestMarkUserTodos(t *testing.T) {
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
		t.Run(tt.name, func(t *testing.T) {
			got := MarkUserTodos(tt.content, tt.userName)
			if got != tt.want {
				t.Errorf("MarkUserTodos() =\n%q\nwant:\n%q", got, tt.want)
			}
		})
	}
}
