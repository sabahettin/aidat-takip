package main

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS members (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	full_name TEXT NOT NULL,
	phone TEXT NOT NULL DEFAULT '',
	email TEXT NOT NULL DEFAULT '',
	join_date TEXT NOT NULL,
	end_date TEXT NOT NULL DEFAULT '',
	monthly_fee REAL NOT NULL DEFAULT 0,
	status TEXT NOT NULL DEFAULT 'active',
	note TEXT NOT NULL DEFAULT '',
	guardian_name TEXT NOT NULL DEFAULT '',
	guardian_phone TEXT NOT NULL DEFAULT '',
	deleted_at TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS payments (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	member_id INTEGER NOT NULL REFERENCES members(id) ON DELETE CASCADE,
	period TEXT NOT NULL,
	amount REAL NOT NULL,
	paid_date TEXT NOT NULL,
	method TEXT NOT NULL DEFAULT 'nakit',
	note TEXT NOT NULL DEFAULT '',
	UNIQUE(member_id, period)
);

CREATE INDEX IF NOT EXISTS idx_payments_member ON payments(member_id);
CREATE INDEX IF NOT EXISTS idx_payments_period ON payments(period);
`

func openDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // modernc sqlite: keep it simple, avoid concurrent-write locking issues
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("şema oluşturulamadı: %w", err)
	}
	if err := migrateSchema(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("veritabanı güncellenemedi: %w", err)
	}
	return db, nil
}

// migrateSchema adds columns introduced after the initial release to
// existing databases created by older versions of the app.
func migrateSchema(db *sql.DB) error {
	if err := addMissingColumns(db, "members", []struct{ column, ddl string }{
		{"guardian_name", "ALTER TABLE members ADD COLUMN guardian_name TEXT NOT NULL DEFAULT ''"},
		{"guardian_phone", "ALTER TABLE members ADD COLUMN guardian_phone TEXT NOT NULL DEFAULT ''"},
		{"end_date", "ALTER TABLE members ADD COLUMN end_date TEXT NOT NULL DEFAULT ''"},
		{"deleted_at", "ALTER TABLE members ADD COLUMN deleted_at TEXT NOT NULL DEFAULT ''"},
	}); err != nil {
		return err
	}
	return addMissingColumns(db, "payments", []struct{ column, ddl string }{
		{"method", "ALTER TABLE payments ADD COLUMN method TEXT NOT NULL DEFAULT 'nakit'"},
	})
}

func addMissingColumns(db *sql.DB, table string, alters []struct{ column, ddl string }) error {
	rows, err := db.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		return err
	}
	existing := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dflt, &pk); err != nil {
			rows.Close()
			return err
		}
		existing[name] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	for _, a := range alters {
		if existing[a.column] {
			continue
		}
		if _, err := db.Exec(a.ddl); err != nil {
			return err
		}
	}
	return nil
}

const memberColumns = "id, full_name, phone, email, join_date, end_date, monthly_fee, status, note, guardian_name, guardian_phone, deleted_at"

func scanMember(row interface{ Scan(dest ...any) error }, m *Member) error {
	return row.Scan(&m.ID, &m.FullName, &m.Phone, &m.Email, &m.JoinDate, &m.EndDate, &m.MonthlyFee, &m.Status, &m.Note, &m.GuardianName, &m.GuardianPhone, &m.DeletedAt)
}

func listMembers(db *sql.DB, includePassive bool) ([]Member, error) {
	q := "SELECT " + memberColumns + " FROM members WHERE deleted_at = ''"
	if !includePassive {
		q += " AND status = 'active'"
	}
	q += " ORDER BY full_name COLLATE NOCASE"
	rows, err := db.Query(q)
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

// listDeletedMembers returns soft-deleted members, most recently deleted first.
func listDeletedMembers(db *sql.DB) ([]Member, error) {
	rows, err := db.Query("SELECT " + memberColumns + " FROM members WHERE deleted_at != '' ORDER BY deleted_at DESC")
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

func getMember(db *sql.DB, id int64) (Member, error) {
	var m Member
	row := db.QueryRow("SELECT "+memberColumns+" FROM members WHERE id = ?", id)
	err := scanMember(row, &m)
	return m, err
}

func createMember(db *sql.DB, m Member) (int64, error) {
	res, err := db.Exec(
		"INSERT INTO members (full_name, phone, email, join_date, end_date, monthly_fee, status, note, guardian_name, guardian_phone) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		m.FullName, m.Phone, m.Email, m.JoinDate, m.EndDate, m.MonthlyFee, m.Status, m.Note, m.GuardianName, m.GuardianPhone,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func updateMember(db *sql.DB, m Member) error {
	_, err := db.Exec(
		"UPDATE members SET full_name = ?, phone = ?, email = ?, join_date = ?, end_date = ?, monthly_fee = ?, note = ?, guardian_name = ?, guardian_phone = ? WHERE id = ?",
		m.FullName, m.Phone, m.Email, m.JoinDate, m.EndDate, m.MonthlyFee, m.Note, m.GuardianName, m.GuardianPhone, m.ID,
	)
	return err
}

func setMemberStatus(db *sql.DB, id int64, status string) error {
	_, err := db.Exec("UPDATE members SET status = ? WHERE id = ?", status, id)
	return err
}

// softDeleteMember moves a member to the trash without losing their payment history.
func softDeleteMember(db *sql.DB, id int64, deletedAt string) error {
	_, err := db.Exec("UPDATE members SET deleted_at = ? WHERE id = ?", deletedAt, id)
	return err
}

// restoreMember brings a soft-deleted member back out of the trash.
func restoreMember(db *sql.DB, id int64) error {
	_, err := db.Exec("UPDATE members SET deleted_at = '' WHERE id = ?", id)
	return err
}

// purgeMember permanently removes a member (and, via cascade, their payments).
// Only meant to be called from the trash view on an already soft-deleted member.
func purgeMember(db *sql.DB, id int64) error {
	_, err := db.Exec("DELETE FROM members WHERE id = ?", id)
	return err
}

func listPaymentsForMember(db *sql.DB, memberID int64) ([]Payment, error) {
	rows, err := db.Query(
		"SELECT id, member_id, period, amount, paid_date, method, note FROM payments WHERE member_id = ? ORDER BY period",
		memberID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var payments []Payment
	for rows.Next() {
		var p Payment
		if err := rows.Scan(&p.ID, &p.MemberID, &p.Period, &p.Amount, &p.PaidDate, &p.Method, &p.Note); err != nil {
			return nil, err
		}
		payments = append(payments, p)
	}
	return payments, rows.Err()
}

func recordPayment(db *sql.DB, p Payment) error {
	_, err := db.Exec(
		`INSERT INTO payments (member_id, period, amount, paid_date, method, note) VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(member_id, period) DO UPDATE SET amount = excluded.amount, paid_date = excluded.paid_date, method = excluded.method, note = excluded.note`,
		p.MemberID, p.Period, p.Amount, p.PaidDate, p.Method, p.Note,
	)
	return err
}

func deletePayment(db *sql.DB, id int64) error {
	_, err := db.Exec("DELETE FROM payments WHERE id = ?", id)
	return err
}

// paymentsTotalForPeriod returns the sum of all payments recorded for a given YYYY-MM period.
func paymentsTotalForPeriod(db *sql.DB, period string) (float64, error) {
	var total sql.NullFloat64
	err := db.QueryRow("SELECT SUM(amount) FROM payments WHERE period = ?", period).Scan(&total)
	if err != nil {
		return 0, err
	}
	return total.Float64, nil
}

// monthlyTotals returns the last n periods (oldest first) with their collected totals.
func monthlyTotals(db *sql.DB, periods []string) (map[string]float64, error) {
	totals := make(map[string]float64, len(periods))
	rows, err := db.Query("SELECT period, SUM(amount) FROM payments GROUP BY period")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var period string
		var sum float64
		if err := rows.Scan(&period, &sum); err != nil {
			return nil, err
		}
		totals[period] = sum
	}
	return totals, rows.Err()
}
