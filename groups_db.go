package main

import "database/sql"

// ---- Groups ----

func listGroups(db *sql.DB) ([]Group, error) {
	rows, err := db.Query("SELECT id, name, note FROM groups ORDER BY name COLLATE NOCASE")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []Group
	for rows.Next() {
		var g Group
		if err := rows.Scan(&g.ID, &g.Name, &g.Note); err != nil {
			return nil, err
		}
		groups = append(groups, g)
	}
	return groups, rows.Err()
}

func getGroup(db *sql.DB, id int64) (Group, error) {
	var g Group
	err := db.QueryRow("SELECT id, name, note FROM groups WHERE id = ?", id).Scan(&g.ID, &g.Name, &g.Note)
	return g, err
}

func createGroup(db *sql.DB, g Group) (int64, error) {
	res, err := db.Exec("INSERT INTO groups (name, note) VALUES (?, ?)", g.Name, g.Note)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func updateGroup(db *sql.DB, g Group) error {
	_, err := db.Exec("UPDATE groups SET name = ?, note = ? WHERE id = ?", g.Name, g.Note, g.ID)
	return err
}

func deleteGroup(db *sql.DB, id int64) error {
	_, err := db.Exec("DELETE FROM groups WHERE id = ?", id)
	return err
}

// ---- Group membership (roster) ----

// listGroupMembers returns the members assigned to a group, active ones first.
func listGroupMembers(db *sql.DB, groupID int64) ([]Member, error) {
	rows, err := db.Query(
		"SELECT "+memberColumnsQualified+" FROM members "+
			"JOIN group_members gm ON gm.member_id = members.id "+
			"WHERE gm.group_id = ? AND members.deleted_at = '' "+
			"ORDER BY members.full_name COLLATE NOCASE",
		groupID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []Member
	for rows.Next() {
		var m Member
		if err := scanMember(rows, &m); err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

// listAvailableMembersForGroup returns active members not already on the
// group's roster, for the "add member" dropdown.
func listAvailableMembersForGroup(db *sql.DB, groupID int64) ([]Member, error) {
	rows, err := db.Query(
		"SELECT "+memberColumns+" FROM members "+
			"WHERE deleted_at = '' AND status = 'active' AND id NOT IN "+
			"(SELECT member_id FROM group_members WHERE group_id = ?) "+
			"ORDER BY full_name COLLATE NOCASE",
		groupID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []Member
	for rows.Next() {
		var m Member
		if err := scanMember(rows, &m); err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

func addGroupMember(db *sql.DB, groupID, memberID int64) error {
	_, err := db.Exec(
		"INSERT OR IGNORE INTO group_members (group_id, member_id) VALUES (?, ?)",
		groupID, memberID,
	)
	return err
}

func removeGroupMember(db *sql.DB, groupID, memberID int64) error {
	_, err := db.Exec("DELETE FROM group_members WHERE group_id = ? AND member_id = ?", groupID, memberID)
	return err
}

// ---- Group schedule ----

func listSchedulesForGroup(db *sql.DB, groupID int64) ([]GroupSchedule, error) {
	rows, err := db.Query(
		"SELECT id, group_id, weekday, start_time, end_time FROM group_schedules WHERE group_id = ?",
		groupID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var schedules []GroupSchedule
	for rows.Next() {
		var s GroupSchedule
		if err := rows.Scan(&s.ID, &s.GroupID, &s.Weekday, &s.StartTime, &s.EndTime); err != nil {
			return nil, err
		}
		schedules = append(schedules, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sortSchedules(schedules)
	return schedules, nil
}

func createSchedule(db *sql.DB, s GroupSchedule) (int64, error) {
	res, err := db.Exec(
		"INSERT INTO group_schedules (group_id, weekday, start_time, end_time) VALUES (?, ?, ?, ?)",
		s.GroupID, s.Weekday, s.StartTime, s.EndTime,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func deleteSchedule(db *sql.DB, id int64) error {
	_, err := db.Exec("DELETE FROM group_schedules WHERE id = ?", id)
	return err
}

// ---- Attendance ----

// attendanceForSession returns a map of member ID to attendance record for
// one group session date (only members with an existing mark are present in
// the map).
func attendanceForSession(db *sql.DB, groupID int64, date string) (map[int64]AttendanceRecord, error) {
	rows, err := db.Query(
		"SELECT id, group_id, member_id, session_date, present, note FROM attendance WHERE group_id = ? AND session_date = ?",
		groupID, date,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int64]AttendanceRecord)
	for rows.Next() {
		var a AttendanceRecord
		if err := rows.Scan(&a.ID, &a.GroupID, &a.MemberID, &a.SessionDate, &a.Present, &a.Note); err != nil {
			return nil, err
		}
		result[a.MemberID] = a
	}
	return result, rows.Err()
}

// saveAttendance upserts one session's attendance for a group in a single
// transaction: present=true for the given member IDs, present=false for
// every other roster member (so unchecking a box in the form correctly
// records an absence rather than leaving no record at all).
func saveAttendance(db *sql.DB, groupID int64, date string, rosterMemberIDs []int64, presentMemberIDs map[int64]bool) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, memberID := range rosterMemberIDs {
		present := presentMemberIDs[memberID]
		if _, err := tx.Exec(
			`INSERT INTO attendance (group_id, member_id, session_date, present) VALUES (?, ?, ?, ?)
			 ON CONFLICT(group_id, member_id, session_date) DO UPDATE SET present = excluded.present`,
			groupID, memberID, date, present,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// listAttendanceSessions returns past session dates for a group (most recent
// first) with a present/total headcount for each, for the history list.
func listAttendanceSessions(db *sql.DB, groupID int64) ([]AttendanceSession, error) {
	rows, err := db.Query(
		`SELECT session_date, SUM(present), COUNT(*) FROM attendance
		 WHERE group_id = ? GROUP BY session_date ORDER BY session_date DESC`,
		groupID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []AttendanceSession
	for rows.Next() {
		var s AttendanceSession
		if err := rows.Scan(&s.Date, &s.PresentCount, &s.TotalCount); err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}

// listGroupsForMember returns the groups a member currently belongs to, for
// display on the member detail page.
func listGroupsForMember(db *sql.DB, memberID int64) ([]Group, error) {
	rows, err := db.Query(
		"SELECT g.id, g.name, g.note FROM groups g "+
			"JOIN group_members gm ON gm.group_id = g.id "+
			"WHERE gm.member_id = ? ORDER BY g.name COLLATE NOCASE",
		memberID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []Group
	for rows.Next() {
		var g Group
		if err := rows.Scan(&g.ID, &g.Name, &g.Note); err != nil {
			return nil, err
		}
		groups = append(groups, g)
	}
	return groups, rows.Err()
}
