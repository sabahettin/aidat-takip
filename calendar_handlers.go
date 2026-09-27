package main

import (
	"encoding/json"
	"html/template"
	"net/http"
	"strconv"
)

// calendarEvent matches the shape FullCalendar expects for a weekly
// recurring event (see web/static/vendor/fullcalendar.min.js).
type calendarEvent struct {
	Title      string `json:"title"`
	DaysOfWeek []int  `json:"daysOfWeek"`
	StartTime  string `json:"startTime"`
	EndTime    string `json:"endTime,omitempty"`
	URL        string `json:"url"`
}

// ---- Calendar (Takvim) ----

func (s *Server) handleCalendar(w http.ResponseWriter, r *http.Request) {
	groups, err := listGroups(s.db)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	var events []calendarEvent
	for _, g := range groups {
		schedules, err := listSchedulesForGroup(s.db, g.ID)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		title := g.Name
		if g.HasInstructor() {
			title = g.Name + " (" + g.InstructorName + ")"
		}
		for _, sched := range schedules {
			// This app's weekday is ISO 1=Pazartesi..7=Pazar; FullCalendar's
			// daysOfWeek is JS Date convention 0=Sunday..6=Saturday.
			fcDay := sched.Weekday % 7
			events = append(events, calendarEvent{
				Title:      title,
				DaysOfWeek: []int{fcDay},
				StartTime:  sched.StartTime,
				EndTime:    sched.EndTime,
				URL:        "/gruplar/" + strconv.FormatInt(g.ID, 10),
			})
		}
	}

	eventsJSON, err := json.Marshal(events)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	s.render(w, "calendar", map[string]any{
		"EventsJSON": template.JS(eventsJSON),
	})
}
