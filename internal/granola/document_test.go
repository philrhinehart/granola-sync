package granola

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type DocumentSuite struct {
	suite.Suite
}

func TestDocumentSuite(t *testing.T) {
	suite.Run(t, new(DocumentSuite))
}

func (s *DocumentSuite) TestGetMeetingDate() {
	now := time.Now()
	calendarTime := now.Add(-24 * time.Hour)

	tests := []struct {
		name     string
		doc      *Document
		expected time.Time
	}{
		{
			name: "from_calendar_event",
			doc: &Document{
				CreatedAt: now,
				GoogleCalendarEvent: &GoogleCalendarEvent{
					Start: &EventTime{DateTime: calendarTime.Format(time.RFC3339)},
				},
			},
			expected: calendarTime.Local(),
		},
		{
			name: "fallback_to_created_at",
			doc: &Document{
				CreatedAt: now,
			},
			expected: now.Local(),
		},
		{
			name: "nil_start_time",
			doc: &Document{
				CreatedAt:           now,
				GoogleCalendarEvent: &GoogleCalendarEvent{},
			},
			expected: now.Local(),
		},
		{
			name: "invalid_datetime_format",
			doc: &Document{
				CreatedAt: now,
				GoogleCalendarEvent: &GoogleCalendarEvent{
					Start: &EventTime{DateTime: "invalid"},
				},
			},
			expected: now.Local(),
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			result := tt.doc.GetMeetingDate()
			s.WithinDuration(tt.expected, result, time.Second)
		})
	}
}

func (s *DocumentSuite) TestGetAttendeeNames() {
	tests := []struct {
		name     string
		doc      *Document
		expected []string
	}{
		{
			name: "from_people_attendees",
			doc: &Document{
				People: &People{
					Attendees: []AttendeeInfo{
						{Name: "Alice"},
						{Name: "Bob"},
					},
				},
			},
			expected: []string{"Alice", "Bob"},
		},
		{
			name: "from_people_details",
			doc: &Document{
				People: &People{
					Attendees: []AttendeeInfo{
						{Details: &PersonDetails{Person: &PersonData{Name: &PersonName{FullName: "Charlie"}}}},
					},
				},
			},
			expected: []string{"Charlie"},
		},
		{
			name: "fallback_to_calendar_displayname",
			doc: &Document{
				GoogleCalendarEvent: &GoogleCalendarEvent{
					Attendees: []Attendee{
						{DisplayName: "Dave", Email: "dave@example.com"},
					},
				},
			},
			expected: []string{"Dave"},
		},
		{
			name: "extract_from_email",
			doc: &Document{
				GoogleCalendarEvent: &GoogleCalendarEvent{
					Attendees: []Attendee{
						{Email: "john.doe@example.com"},
					},
				},
			},
			expected: []string{"John Doe"},
		},
		{
			name:     "no_attendees",
			doc:      &Document{},
			expected: nil,
		},
		{
			name: "dedup_names",
			doc: &Document{
				People: &People{
					Attendees: []AttendeeInfo{
						{Name: "Alice"},
						{Name: "Alice"},
						{Name: "Bob"},
					},
				},
			},
			expected: []string{"Alice", "Bob"},
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			result := tt.doc.GetAttendeeNames()
			s.Equal(tt.expected, result)
		})
	}
}

func (s *DocumentSuite) TestIsUserAttendee() {
	tests := []struct {
		name      string
		doc       *Document
		userEmail string
		want      bool
	}{
		{
			name: "calendar_attendee_match",
			doc: &Document{
				GoogleCalendarEvent: &GoogleCalendarEvent{
					Attendees: []Attendee{{Email: "test@example.com"}},
				},
			},
			userEmail: "test@example.com",
			want:      true,
		},
		{
			name: "calendar_attendee_no_match",
			doc: &Document{
				GoogleCalendarEvent: &GoogleCalendarEvent{
					Attendees: []Attendee{{Email: "other@example.com"}},
				},
			},
			userEmail: "test@example.com",
			want:      false,
		},
		{
			name: "calendar_self_flag",
			doc: &Document{
				GoogleCalendarEvent: &GoogleCalendarEvent{
					Attendees: []Attendee{{Email: "user@example.com", Self: true}},
				},
			},
			userEmail: "",
			want:      true,
		},
		{
			name: "calendar_no_self_flag_no_email",
			doc: &Document{
				GoogleCalendarEvent: &GoogleCalendarEvent{
					Attendees: []Attendee{{Email: "user@example.com", Self: false}},
				},
			},
			userEmail: "",
			want:      false,
		},
		{
			name: "no_calendar_is_creator",
			doc: &Document{
				People: &People{
					Creator: &PersonInfo{Email: "test@example.com"},
				},
			},
			userEmail: "test@example.com",
			want:      true,
		},
		{
			name: "no_calendar_not_creator",
			doc: &Document{
				People: &People{
					Creator: &PersonInfo{Email: "other@example.com"},
				},
			},
			userEmail: "test@example.com",
			want:      false,
		},
		{
			name:      "no_calendar_no_creator",
			doc:       &Document{},
			userEmail: "test@example.com",
			want:      true,
		},
		{
			name:      "no_calendar_no_email",
			doc:       &Document{},
			userEmail: "",
			want:      true,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.Equal(tt.want, tt.doc.IsUserAttendee(tt.userEmail))
		})
	}
}

func (s *DocumentSuite) TestIsDeleted() {
	now := time.Now()

	tests := []struct {
		name string
		doc  *Document
		want bool
	}{
		{
			name: "not_deleted",
			doc:  &Document{DeletedAt: nil},
			want: false,
		},
		{
			name: "deleted",
			doc:  &Document{DeletedAt: &now},
			want: true,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.Equal(tt.want, tt.doc.IsDeleted())
		})
	}
}

func (s *DocumentSuite) TestHasNotes() {
	emptyNotes := ""
	someNotes := "Meeting notes here"

	tests := []struct {
		name string
		doc  *Document
		want bool
	}{
		{
			name: "nil_notes",
			doc:  &Document{NotesMarkdown: nil},
			want: false,
		},
		{
			name: "empty_notes",
			doc:  &Document{NotesMarkdown: &emptyNotes},
			want: false,
		},
		{
			name: "has_notes",
			doc:  &Document{NotesMarkdown: &someNotes},
			want: true,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.Equal(tt.want, tt.doc.HasNotes())
		})
	}
}

func (s *DocumentSuite) TestGetMeetingTimeRange() {
	tests := []struct {
		name      string
		doc       *Document
		wantStart string
		wantEnd   string
		wantTz    string
	}{
		{
			name:      "no_calendar_event",
			doc:       &Document{},
			wantStart: "",
			wantEnd:   "",
			wantTz:    "",
		},
		{
			name: "with_times",
			doc: &Document{
				GoogleCalendarEvent: &GoogleCalendarEvent{
					Start: &EventTime{DateTime: "2024-01-15T10:00:00-08:00"},
					End:   &EventTime{DateTime: "2024-01-15T11:00:00-08:00"},
				},
			},
			wantStart: "10:00 AM",
			wantEnd:   "11:00 AM",
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			start, end, _ := tt.doc.GetMeetingTimeRange()
			if tt.wantStart != "" {
				s.NotEmpty(start)
			} else {
				s.Empty(start)
			}
			if tt.wantEnd != "" {
				s.NotEmpty(end)
			} else {
				s.Empty(end)
			}
		})
	}
}

func (s *DocumentSuite) TestExtractNameFromEmail() {
	tests := []struct {
		email    string
		expected string
	}{
		{"john.doe@example.com", "John Doe"},
		{"jane_smith@example.com", "Jane Smith"},
		{"bob-jones@example.com", "Bob Jones"},
		{"alice@example.com", "Alice"},
		{"", ""},
		{"@example.com", ""},
	}

	for _, tt := range tests {
		s.Run(tt.email, func() {
			result := extractNameFromEmail(tt.email)
			s.Equal(tt.expected, result)
		})
	}
}
