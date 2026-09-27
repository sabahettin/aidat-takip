package main

import (
	"database/sql"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS members (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	full_name TEXT NOT NULL,
	phone TEXT NOT NULL DEFAULT '',
	email TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'active',
	note TEXT NOT NULL DEFAULT '',
	guardian_name TEXT NOT NULL DEFAULT '',
	guardian_phone TEXT NOT NULL DEFAULT '',
	deleted_at TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS membership_periods (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	member_id INTEGER NOT NULL REFERENCES members(id) ON DELETE CASCADE,
	start_date TEXT NOT NULL,
	end_date TEXT NOT NULL DEFAULT '',
	monthly_fee REAL NOT NULL DEFAULT 0,
	note TEXT NOT NULL DEFAULT ''
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

CREATE INDEX IF NOT EXISTS idx_periods_member ON membership_periods(member_id);
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

// migrateSchema brings a database created by an older version of the app up
// to date: it adds columns introduced since, and once (idempotently) turns
// each member's legacy join_date/end_date/monthly_fee fields into their
// first membership_periods row, since those columns were removed from new
// installs in favor of that table.
func migrateSchema(db *sql.DB) error {
	if err := addMissingColumns(db, "members", []struct{ column, ddl string }{
		{"guardian_name", "ALTER TABLE members ADD COLUMN guardian_name TEXT NOT NULL DEFAULT ''"},
		{"guardian_phone", "ALTER TABLE members ADD COLUMN guardian_phone TEXT NOT NULL DEFAULT ''"},
		{"deleted_at", "ALTER TABLE members ADD COLUMN deleted_at TEXT NOT NULL DEFAULT ''"},
	}); err != nil {
		return err
	}
	if err := addMissingColumns(db, "payments", []struct{ column, ddl string }{
		{"method", "ALTER TABLE payments ADD COLUMN method TEXT NOT NULL DEFAULT 'nakit'"},
	}); err != nil {
		return err
	}
	return seedPeriodsFromLegacyMemberFields(db)
}

// seedPeriodsFromLegacyMemberFields is a one-time, idempotent migration: if
// the members table still has the old join_date/monthly_fee columns (from
// before membership_periods existed), it creates a matching period for every
// member that doesn't have one yet, then leaves the legacy columns alone
// (harmless, unused going forward).
func seedPeriodsFromLegacyMemberFields(db *sql.DB) error {
	hasLegacy, err := hasColumn(db, "members", "join_date")
	if err != nil || !hasLegacy {
		return err
	}
	rows, err := db.Query(`
		SELECT m.id, m.join_date, m.end_date, m.monthly_fee
		FROM members m
		LEFT JOIN membership_periods p ON p.member_id = m.id
		WHERE p.id IS NULL AND m.join_date IS NOT NULL AND m.join_date != ''
	`)
	if err != nil {
		return err
	}
	type legacy struct {
		id                int64
		joinDate, endDate string
		monthlyFee        float64
	}
	var toSeed []legacy
	for rows.Next() {
		var l legacy
		if err := rows.Scan(&l.id, &l.joinDate, &l.endDate, &l.monthlyFee); err != nil {
			rows.Close()
			return err
		}
		toSeed = append(toSeed, l)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, l := range toSeed {
		if _, err := db.Exec(
			"INSERT INTO membership_periods (member_id, start_date, end_date, monthly_fee) VALUES (?, ?, ?, ?)",
			l.id, l.joinDate, l.endDate, l.monthlyFee,
		); err != nil {
			return err
		}
	}
	return nil
}

func hasColumn(db *sql.DB, table, column string) (bool, error) {
	rows, err := db.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dflt, &pk); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
}

func addMissingColumns(db *sql.DB, table string, alters []struct{ column, ddl string }) error {
	existing := map[string]bool{}
	for _, a := range alters {
		ok, err := hasColumn(db, table, a.column)
		if err != nil {
			return err
		}
		existing[a.column] = ok
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

const memberColumns = "id, full_name, phone, email, status, note, guardian_name, guardian_phone, deleted_at"

func scanMember(row interface{ Scan(dest ...any) error }, m *Member) error {
	return row.Scan(&m.ID, &m.FullName, &m.Phone, &m.Email, &m.Status, &m.Note, &m.GuardianName, &m.GuardianPhone, &m.DeletedAt)
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
		"INSERT INTO members (full_name, phone, email, status, note, guardian_name, guardian_phone) VALUES (?, ?, ?, ?, ?, ?, ?)",
		m.FullName, m.Phone, m.Email, m.Status, m.Note, m.GuardianName, m.GuardianPhone,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func updateMember(db *sql.DB, m Member) error {
	_, err := db.Exec(
		"UPDATE members SET full_name = ?, phone = ?, email = ?, note = ?, guardian_name = ?, guardian_phone = ? WHERE id = ?",
		m.FullName, m.Phone, m.Email, m.Note, m.GuardianName, m.GuardianPhone, m.ID,
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

// purgeMember permanently removes a member (and, via cascade, their periods and payments).
// Only meant to be called from the trash view on an already soft-deleted member.
func purgeMember(db *sql.DB, id int64) error {
	_, err := db.Exec("DELETE FROM members WHERE id = ?", id)
	return err
}

// ---- Membership periods ----

func listPeriodsForMember(db *sql.DB, memberID int64) ([]MembershipPeriod, error) {
	rows, err := db.Query(
		"SELECT id, member_id, start_date, end_date, monthly_fee, note FROM membership_periods WHERE member_id = ? ORDER BY start_date",
		memberID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var periods []MembershipPeriod
	for rows.Next() {
		var p MembershipPeriod
		if err := rows.Scan(&p.ID, &p.MemberID, &p.StartDate, &p.EndDate, &p.MonthlyFee, &p.Note); err != nil {
			return nil, err
		}
		periods = append(periods, p)
	}
	return periods, rows.Err()
}

func getPeriod(db *sql.DB, id int64) (MembershipPeriod, error) {
	var p MembershipPeriod
	err := db.QueryRow(
		"SELECT id, member_id, start_date, end_date, monthly_fee, note FROM membership_periods WHERE id = ?", id,
	).Scan(&p.ID, &p.MemberID, &p.StartDate, &p.EndDate, &p.MonthlyFee, &p.Note)
	return p, err
}

func createPeriod(db *sql.DB, p MembershipPeriod) (int64, error) {
	res, err := db.Exec(
		"INSERT INTO membership_periods (member_id, start_date, end_date, monthly_fee, note) VALUES (?, ?, ?, ?, ?)",
		p.MemberID, p.StartDate, p.EndDate, p.MonthlyFee, p.Note,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func updatePeriod(db *sql.DB, p MembershipPeriod) error {
	_, err := db.Exec(
		"UPDATE membership_periods SET start_date = ?, end_date = ?, monthly_fee = ?, note = ? WHERE id = ?",
		p.StartDate, p.EndDate, p.MonthlyFee, p.Note, p.ID,
	)
	return err
}

func deletePeriod(db *sql.DB, id int64) error {
	_, err := db.Exec("DELETE FROM membership_periods WHERE id = ?", id)
	return err
}

// applyFeeChange makes fee apply to a member from effectiveMonth ("YYYY-MM")
// onward, splitting the period that spans that month (see planFeeChange).
// All row changes happen in one transaction so a failure leaves the
// member's periods untouched.
func applyFeeChange(db *sql.DB, memberID int64, effectiveMonth string, fee float64) error {
	periods, err := listPeriodsForMember(db, memberID)
	if err != nil {
		return err
	}
	plan, err := planFeeChange(periods, effectiveMonth, fee)
	if err != nil {
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, p := range plan.Update {
		if _, err := tx.Exec(
			"UPDATE membership_periods SET start_date = ?, end_date = ?, monthly_fee = ?, note = ? WHERE id = ?",
			p.StartDate, p.EndDate, p.MonthlyFee, p.Note, p.ID,
		); err != nil {
			return err
		}
	}
	for _, p := range plan.Insert {
		if _, err := tx.Exec(
			"INSERT INTO membership_periods (member_id, start_date, end_date, monthly_fee, note) VALUES (?, ?, ?, ?, ?)",
			p.MemberID, p.StartDate, p.EndDate, p.MonthlyFee, p.Note,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ---- Payments ----

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

func getPayment(db *sql.DB, id int64) (Payment, error) {
	var p Payment
	err := db.QueryRow(
		"SELECT id, member_id, period, amount, paid_date, method, note FROM payments WHERE id = ?", id,
	).Scan(&p.ID, &p.MemberID, &p.Period, &p.Amount, &p.PaidDate, &p.Method, &p.Note)
	return p, err
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

// monthlyTotals returns the collected totals grouped by period.
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

// paymentTotalsByMethod sums recorded payments across the given periods,
// split by payment method (cash vs bank transfer).
func paymentTotalsByMethod(db *sql.DB, periods []string) (nakit, eft float64, err error) {
	if len(periods) == 0 {
		return 0, 0, nil
	}
	placeholders := strings.Repeat("?,", len(periods))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, len(periods))
	for i, p := range periods {
		args[i] = p
	}
	rows, err := db.Query(
		"SELECT method, COALESCE(SUM(amount), 0) FROM payments WHERE period IN ("+placeholders+") GROUP BY method",
		args...,
	)
	if err != nil {
		return 0, 0, err
	}
	defer rows.Close()
	for rows.Next() {
		var method string
		var sum float64
		if err := rows.Scan(&method, &sum); err != nil {
			return 0, 0, err
		}
		if method == "eft" {
			eft = sum
		} else {
			nakit = sum
		}
	}
	return nakit, eft, rows.Err()
}
