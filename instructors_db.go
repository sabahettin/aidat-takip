package main

import "database/sql"

func listInstructors(db *sql.DB) ([]Instructor, error) {
	rows, err := db.Query("SELECT id, full_name, phone, email, note FROM instructors ORDER BY full_name COLLATE NOCASE")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var instructors []Instructor
	for rows.Next() {
		var i Instructor
		if err := rows.Scan(&i.ID, &i.FullName, &i.Phone, &i.Email, &i.Note); err != nil {
			return nil, err
		}
		instructors = append(instructors, i)
	}
	return instructors, rows.Err()
}

func getInstructor(db *sql.DB, id int64) (Instructor, error) {
	var i Instructor
	err := db.QueryRow("SELECT id, full_name, phone, email, note FROM instructors WHERE id = ?", id).
		Scan(&i.ID, &i.FullName, &i.Phone, &i.Email, &i.Note)
	return i, err
}

func createInstructor(db *sql.DB, i Instructor) (int64, error) {
	res, err := db.Exec(
		"INSERT INTO instructors (full_name, phone, email, note) VALUES (?, ?, ?, ?)",
		i.FullName, i.Phone, i.Email, i.Note,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func updateInstructor(db *sql.DB, i Instructor) error {
	_, err := db.Exec(
		"UPDATE instructors SET full_name = ?, phone = ?, email = ?, note = ? WHERE id = ?",
		i.FullName, i.Phone, i.Email, i.Note, i.ID,
	)
	return err
}

// deleteInstructor removes an instructor and unassigns them from any groups
// (groups.instructor_id has no DB-level foreign key, so this is done
// explicitly rather than relying on ON DELETE SET NULL).
func deleteInstructor(db *sql.DB, id int64) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec("UPDATE groups SET instructor_id = NULL WHERE instructor_id = ?", id); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM instructors WHERE id = ?", id); err != nil {
		return err
	}
	return tx.Commit()
}

// countGroupsForInstructor is used on the instructor edit page to show how
// many groups currently have this instructor assigned.
func listGroupsForInstructor(db *sql.DB, instructorID int64) ([]Group, error) {
	rows, err := db.Query("SELECT id, name, note FROM groups WHERE instructor_id = ? ORDER BY name COLLATE NOCASE", instructorID)
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
