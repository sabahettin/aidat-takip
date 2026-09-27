package main

import (
	"errors"
	"net/http"
	"strconv"
	"time"
)

// ---- Groups ----

type groupRow struct {
	Group
	MemberCount int
}

func (s *Server) handleGroupsList(w http.ResponseWriter, r *http.Request) {
	groups, err := listGroups(s.db)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	rows := make([]groupRow, 0, len(groups))
	for _, g := range groups {
		members, err := listGroupMembers(s.db, g.ID)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		rows = append(rows, groupRow{Group: g, MemberCount: len(members)})
	}
	s.render(w, "groups_list", map[string]any{"Groups": rows})
}

func (s *Server) handleGroupNewForm(w http.ResponseWriter, r *http.Request) {
	s.render(w, "group_form", map[string]any{
		"IsNew":   true,
		"Group":   Group{},
		"FormURL": "/gruplar",
	})
}

func parseGroupForm(r *http.Request) (Group, error) {
	if err := r.ParseForm(); err != nil {
		return Group{}, err
	}
	name := r.FormValue("name")
	if name == "" {
		return Group{}, errors.New("grup adı zorunludur")
	}
	return Group{Name: name, Note: r.FormValue("note")}, nil
}

func (s *Server) handleGroupCreate(w http.ResponseWriter, r *http.Request) {
	g, err := parseGroupForm(r)
	if err != nil {
		s.render(w, "group_form", map[string]any{
			"IsNew": true, "Group": g, "FormURL": "/gruplar", "Error": err.Error(),
		})
		return
	}
	id, err := createGroup(s.db, g)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	redirectWithToast(w, r, "/gruplar/"+strconv.FormatInt(id, 10), "Grup oluşturuldu.", "success")
}

func (s *Server) handleGroupEditForm(w http.ResponseWriter, r *http.Request) {
	id, err := idFromPath(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	g, err := getGroup(s.db, id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	s.render(w, "group_form", map[string]any{
		"IsNew":   false,
		"Group":   g,
		"FormURL": "/gruplar/" + strconv.FormatInt(id, 10) + "/duzenle",
	})
}

func (s *Server) handleGroupUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := idFromPath(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	g, err := parseGroupForm(r)
	if err != nil {
		g.ID = id
		s.render(w, "group_form", map[string]any{
			"IsNew": false, "Group": g, "FormURL": "/gruplar/" + strconv.FormatInt(id, 10) + "/duzenle", "Error": err.Error(),
		})
		return
	}
	g.ID = id
	if err := updateGroup(s.db, g); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	redirectWithToast(w, r, "/gruplar/"+strconv.FormatInt(id, 10), "Grup güncellendi.", "success")
}

func (s *Server) handleGroupDelete(w http.ResponseWriter, r *http.Request) {
	id, err := idFromPath(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := deleteGroup(s.db, id); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	redirectWithToast(w, r, "/gruplar", "Grup silindi.", "success")
}

func (s *Server) handleGroupDetail(w http.ResponseWriter, r *http.Request) {
	id, err := idFromPath(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	g, err := getGroup(s.db, id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	schedules, err := listSchedulesForGroup(s.db, id)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	roster, err := listGroupMembers(s.db, id)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	available, err := listAvailableMembersForGroup(s.db, id)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	sessions, err := listAttendanceSessions(s.db, id)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	s.render(w, "group_detail", map[string]any{
		"Group":            g,
		"Schedules":        schedules,
		"Roster":           roster,
		"AvailableMembers": available,
		"Sessions":         sessions,
		"Today":            time.Now().Format("2006-01-02"),
		"Weekdays":         sortedWeekdayList(),
	})
}

// ---- Schedule ----

func (s *Server) handleScheduleCreate(w http.ResponseWriter, r *http.Request) {
	id, err := idFromPath(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	groupURL := "/gruplar/" + strconv.FormatInt(id, 10)
	if err := r.ParseForm(); err != nil {
		redirectWithToast(w, r, groupURL, err.Error(), "error")
		return
	}
	weekday, err := strconv.Atoi(r.FormValue("weekday"))
	if err != nil || weekday < 1 || weekday > 7 {
		redirectWithToast(w, r, groupURL, "Geçersiz gün seçimi.", "error")
		return
	}
	startTime := r.FormValue("start_time")
	if startTime == "" {
		redirectWithToast(w, r, groupURL, "Başlangıç saati zorunludur.", "error")
		return
	}
	sched := GroupSchedule{
		GroupID:   id,
		Weekday:   weekday,
		StartTime: startTime,
		EndTime:   r.FormValue("end_time"),
	}
	if _, err := createSchedule(s.db, sched); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	redirectWithToast(w, r, groupURL, "Ders saati eklendi.", "success")
}

func (s *Server) handleScheduleDelete(w http.ResponseWriter, r *http.Request) {
	id, err := idFromPath(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	sid, err := pidFromPath(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := deleteSchedule(s.db, sid); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	redirectWithToast(w, r, "/gruplar/"+strconv.FormatInt(id, 10), "Ders saati silindi.", "success")
}

// ---- Roster ----

func (s *Server) handleGroupMemberAdd(w http.ResponseWriter, r *http.Request) {
	id, err := idFromPath(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	groupURL := "/gruplar/" + strconv.FormatInt(id, 10)
	if err := r.ParseForm(); err != nil {
		redirectWithToast(w, r, groupURL, err.Error(), "error")
		return
	}
	memberID, err := strconv.ParseInt(r.FormValue("member_id"), 10, 64)
	if err != nil {
		redirectWithToast(w, r, groupURL, "Üye seçilmedi.", "error")
		return
	}
	if err := addGroupMember(s.db, id, memberID); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	redirectWithToast(w, r, groupURL, "Üye gruba eklendi.", "success")
}

func (s *Server) handleGroupMemberRemove(w http.ResponseWriter, r *http.Request) {
	id, err := idFromPath(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	mid, err := midFromPath(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := removeGroupMember(s.db, id, mid); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	redirectWithToast(w, r, "/gruplar/"+strconv.FormatInt(id, 10), "Üye gruptan çıkarıldı.", "success")
}

// ---- Attendance ----

type attendanceRow struct {
	Member  Member
	Present bool
}

func (s *Server) handleAttendanceForm(w http.ResponseWriter, r *http.Request) {
	id, err := idFromPath(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	g, err := getGroup(s.db, id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	date := r.URL.Query().Get("tarih")
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	roster, err := listGroupMembers(s.db, id)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	existing, err := attendanceForSession(s.db, id, date)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	// Default to present for a session with no marks recorded yet, so
	// taking attendance is "uncheck the absent ones" rather than starting
	// from all-empty.
	hasRecords := len(existing) > 0
	rows := make([]attendanceRow, 0, len(roster))
	for _, m := range roster {
		present := !hasRecords
		if rec, ok := existing[m.ID]; ok {
			present = rec.Present
		}
		rows = append(rows, attendanceRow{Member: m, Present: present})
	}

	s.render(w, "attendance_form", map[string]any{
		"Group": g,
		"Date":  date,
		"Rows":  rows,
	})
}

func (s *Server) handleAttendanceSave(w http.ResponseWriter, r *http.Request) {
	id, err := idFromPath(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	groupURL := "/gruplar/" + strconv.FormatInt(id, 10)
	if err := r.ParseForm(); err != nil {
		redirectWithToast(w, r, groupURL, err.Error(), "error")
		return
	}
	date := r.FormValue("session_date")
	if date == "" {
		redirectWithToast(w, r, groupURL, "Tarih seçilmedi.", "error")
		return
	}
	roster, err := listGroupMembers(s.db, id)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	rosterIDs := make([]int64, len(roster))
	for i, m := range roster {
		rosterIDs[i] = m.ID
	}
	present := make(map[int64]bool)
	for _, v := range r.Form["present"] {
		mid, err := strconv.ParseInt(v, 10, 64)
		if err == nil {
			present[mid] = true
		}
	}
	if err := saveAttendance(s.db, id, date, rosterIDs, present); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	redirectWithToast(w, r, groupURL, "Yoklama kaydedildi ("+trDate(date)+").", "success")
}
