package main

import "sort"

// Group is a class/team (Sınıf/Grup) that members can be assigned to, with
// its own weekly schedule and per-session attendance records.
type Group struct {
	ID             int64
	Name           string
	Note           string
	InstructorID   int64  // 0 = atanmamış
	InstructorName string // join'lenmiş, salt gösterim için
}

// HasInstructor reports whether the group has an instructor assigned.
func (g Group) HasInstructor() bool { return g.InstructorID != 0 }

// GroupSchedule is one weekly recurring time slot for a group, e.g.
// "Pazartesi 18:00-19:00".
type GroupSchedule struct {
	ID        int64
	GroupID   int64
	Weekday   int // 1=Pazartesi ... 7=Pazar (ISO 8601)
	StartTime string
	EndTime   string
}

var weekdayNames = map[int]string{
	1: "Pazartesi", 2: "Salı", 3: "Çarşamba", 4: "Perşembe",
	5: "Cuma", 6: "Cumartesi", 7: "Pazar",
}

// WeekdayLabel renders the schedule's weekday in Turkish.
func (s GroupSchedule) WeekdayLabel() string { return weekdayNames[s.Weekday] }

// TimeLabel renders the slot's time range, e.g. "18:00–19:00" or just
// "18:00" when no end time was given.
func (s GroupSchedule) TimeLabel() string {
	if s.EndTime == "" {
		return s.StartTime
	}
	return s.StartTime + "–" + s.EndTime
}

// sortedWeekdayList returns 1..7 in calendar order, for building weekday
// <select> options in forms.
func sortedWeekdayList() []int {
	days := make([]int, 0, 7)
	for d := 1; d <= 7; d++ {
		days = append(days, d)
	}
	return days
}

// sortSchedules orders schedule slots by weekday then start time, so a
// group's weekly program reads Monday-through-Sunday.
func sortSchedules(schedules []GroupSchedule) {
	sort.Slice(schedules, func(i, j int) bool {
		if schedules[i].Weekday != schedules[j].Weekday {
			return schedules[i].Weekday < schedules[j].Weekday
		}
		return schedules[i].StartTime < schedules[j].StartTime
	})
}

// AttendanceRecord is one member's presence mark for one group session date.
type AttendanceRecord struct {
	ID          int64
	GroupID     int64
	MemberID    int64
	SessionDate string // YYYY-MM-DD
	Present     bool
	Note        string
}

// AttendanceSession summarizes one past session date for a group's history list.
type AttendanceSession struct {
	Date         string
	PresentCount int
	TotalCount   int
}
