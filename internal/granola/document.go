package granola

import "time"

// Document represents a Granola meeting document
type Document struct {
	ID                  string               `json:"id"`
	Title               string               `json:"title"`
	CreatedAt           time.Time            `json:"created_at"`
	UpdatedAt           time.Time            `json:"updated_at"`
	DeletedAt           *time.Time           `json:"deleted_at"`
	Type                string               `json:"type"`
	Notes               interface{}          `json:"notes"`
	NotesPlain          *string              `json:"notes_plain"`
	NotesMarkdown       *string              `json:"notes_markdown"`
	Overview            *string              `json:"overview"`
	GoogleCalendarEvent *GoogleCalendarEvent `json:"google_calendar_event"`
	People              *People              `json:"people"`
}

type GoogleCalendarEvent struct {
	ID        string     `json:"id"`
	Summary   string     `json:"summary"`
	Start     *EventTime `json:"start"`
	End       *EventTime `json:"end"`
	Attendees []Attendee `json:"attendees"`
}

type EventTime struct {
	DateTime string `json:"dateTime"`
	TimeZone string `json:"timeZone"`
}

type Attendee struct {
	Email          string `json:"email"`
	DisplayName    string `json:"displayName"`
	ResponseStatus string `json:"responseStatus"`
	Self           bool   `json:"self"`
	Organizer      bool   `json:"organizer"`
}

type People struct {
	Title     string         `json:"title"`
	Creator   *PersonInfo    `json:"creator"`
	Attendees []AttendeeInfo `json:"attendees"`
}

type PersonInfo struct {
	Name    string         `json:"name"`
	Email   string         `json:"email"`
	Details *PersonDetails `json:"details"`
}

type AttendeeInfo struct {
	Name    string         `json:"name"`
	Email   string         `json:"email"`
	Details *PersonDetails `json:"details"`
}

type PersonDetails struct {
	Person *PersonData `json:"person"`
}

type PersonData struct {
	Name   *PersonName `json:"name"`
	Avatar string      `json:"avatar"`
}

type PersonName struct {
	FullName   string `json:"fullName"`
	GivenName  string `json:"givenName"`
	FamilyName string `json:"familyName"`
}

// GetMeetingDate returns the meeting date from the calendar event or created_at, localized to system timezone
func (d *Document) GetMeetingDate() time.Time {
	if d.GoogleCalendarEvent != nil && d.GoogleCalendarEvent.Start != nil {
		if t, err := time.Parse(time.RFC3339, d.GoogleCalendarEvent.Start.DateTime); err == nil {
			return t.Local()
		}
	}
	return d.CreatedAt.Local()
}

// GetMeetingTimeRange returns formatted start and end times in 12-hour format, localized to system timezone
func (d *Document) GetMeetingTimeRange() (start, end, tz string) {
	if d.GoogleCalendarEvent == nil {
		return "", "", ""
	}
	if d.GoogleCalendarEvent.Start != nil {
		if t, err := time.Parse(time.RFC3339, d.GoogleCalendarEvent.Start.DateTime); err == nil {
			localTime := t.Local()
			start = localTime.Format("3:04 PM")
			tz = localTime.Format("MST") // Get local timezone abbreviation
		}
	}
	if d.GoogleCalendarEvent.End != nil {
		if t, err := time.Parse(time.RFC3339, d.GoogleCalendarEvent.End.DateTime); err == nil {
			end = t.Local().Format("3:04 PM")
		}
	}
	return start, end, tz
}

// GetAttendeeNames returns a list of attendee names
func (d *Document) GetAttendeeNames() []string {
	var names []string
	seen := make(map[string]bool)

	// Get from People.Attendees first (has better names)
	if d.People != nil {
		for _, a := range d.People.Attendees {
			name := a.Name
			if name == "" && a.Details != nil && a.Details.Person != nil && a.Details.Person.Name != nil {
				name = a.Details.Person.Name.FullName
			}
			if name != "" && !seen[name] {
				seen[name] = true
				names = append(names, name)
			}
		}
	}

	// Fall back to GoogleCalendarEvent attendees if no People attendees
	if len(names) == 0 && d.GoogleCalendarEvent != nil {
		for _, a := range d.GoogleCalendarEvent.Attendees {
			name := a.DisplayName
			if name == "" {
				// Extract name from email
				name = extractNameFromEmail(a.Email)
			}
			if name != "" && !seen[name] {
				seen[name] = true
				names = append(names, name)
			}
		}
	}

	return names
}

func extractNameFromEmail(email string) string {
	if email == "" {
		return ""
	}
	// Extract part before @ and convert to title case
	parts := []byte(email)
	atIdx := -1
	for i, b := range parts {
		if b == '@' {
			atIdx = i
			break
		}
	}
	if atIdx <= 0 {
		return ""
	}
	localPart := string(parts[:atIdx])
	// Replace dots and underscores with spaces
	result := make([]byte, 0, len(localPart))
	capitalize := true
	for i := 0; i < len(localPart); i++ {
		c := localPart[i]
		if c == '.' || c == '_' || c == '-' {
			result = append(result, ' ')
			capitalize = true
		} else if capitalize && c >= 'a' && c <= 'z' {
			result = append(result, c-32) // uppercase
			capitalize = false
		} else {
			result = append(result, c)
			capitalize = false
		}
	}
	return string(result)
}

// IsDeleted returns true if the document has been deleted
func (d *Document) IsDeleted() bool {
	return d.DeletedAt != nil
}

// IsUserAttendee returns true if the specified user email is an attendee of this meeting
func (d *Document) IsUserAttendee(userEmail string) bool {
	// For meetings without a calendar event, check the creator
	if d.GoogleCalendarEvent == nil {
		if userEmail == "" {
			return true // No email configured, include all notes
		}
		// Check if user is the creator
		if d.People != nil && d.People.Creator != nil {
			return d.People.Creator.Email == userEmail
		}
		return true // No creator info, include by default
	}

	if userEmail == "" {
		// No email configured, fall back to checking for any self flag
		for _, a := range d.GoogleCalendarEvent.Attendees {
			if a.Self {
				return true
			}
		}
		return false
	}
	// Check if the specified email is an attendee
	for _, a := range d.GoogleCalendarEvent.Attendees {
		if a.Email == userEmail {
			return true
		}
	}
	return false
}

// HasNotes returns true if the document has notes
func (d *Document) HasNotes() bool {
	return d.NotesMarkdown != nil && *d.NotesMarkdown != ""
}
