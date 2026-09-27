package main

import "time"

type Member struct {
	ID            int64
	FullName      string
	Phone         string
	Email         string
	JoinDate      string // YYYY-MM-DD
	EndDate       string // YYYY-MM-DD, opsiyonel: dönemi belirli bir kayıt ise ayrılış/bitiş tarihi
	MonthlyFee    float64
	Status        string // "active" | "passive"
	Note          string
	GuardianName  string // opsiyonel: üye reşit değilse veli adı
	GuardianPhone string // opsiyonel: veli telefonu
	DeletedAt     string // boş değilse üye çöp kutusundadır (soft delete)
}

func (m Member) IsActive() bool { return m.Status == "active" }

// IsDeleted reports whether the member has been soft-deleted.
func (m Member) IsDeleted() bool { return m.DeletedAt != "" }

// HasGuardian reports whether a guardian phone was provided.
func (m Member) HasGuardian() bool { return m.GuardianPhone != "" }

// ReminderPhone returns the number reminders should be sent to: the
// guardian's if one is on file, otherwise the member's own phone.
func (m Member) ReminderPhone() string {
	if m.GuardianPhone != "" {
		return m.GuardianPhone
	}
	return m.Phone
}

// ReminderContactName returns who the reminder message should address.
func (m Member) ReminderContactName() string {
	if m.GuardianName != "" {
		return m.GuardianName
	}
	return m.FullName
}

type Payment struct {
	ID       int64
	MemberID int64
	Period   string // YYYY-MM
	Amount   float64
	PaidDate string // YYYY-MM-DD
	Method   string // "eft" | "nakit"
	Note     string
}

// MethodLabel renders the payment method in Turkish for display.
func (p Payment) MethodLabel() string {
	if p.Method == "eft" {
		return "EFT/Havale"
	}
	return "Nakit"
}

// DuePeriod represents one month of expected fee for a member and whether it was paid.
type DuePeriod struct {
	Period   string // YYYY-MM
	Amount   float64
	Paid     bool
	PaidDate string
	Method   string
	Overdue  bool
}

// periodLabelText renders a "YYYY-MM" period as a human Turkish label, e.g. "Eylül 2026".
func periodLabelText(p string) string {
	months := map[string]string{
		"01": "Ocak", "02": "Şubat", "03": "Mart", "04": "Nisan",
		"05": "Mayıs", "06": "Haziran", "07": "Temmuz", "08": "Ağustos",
		"09": "Eylül", "10": "Ekim", "11": "Kasım", "12": "Aralık",
	}
	if len(p) != 7 {
		return p
	}
	return months[p[5:7]] + " " + p[:4]
}

// periodsInRange returns "YYYY-MM" strings from joinDate's month up to and including cutoff's month.
func periodsInRange(joinDate string, cutoff time.Time) []string {
	start, err := time.Parse("2006-01-02", joinDate)
	if err != nil {
		return nil
	}
	cur := time.Date(start.Year(), start.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(cutoff.Year(), cutoff.Month(), 1, 0, 0, 0, 0, time.UTC)
	var periods []string
	for !cur.After(end) {
		periods = append(periods, cur.Format("2006-01"))
		cur = cur.AddDate(0, 1, 0)
	}
	return periods
}

// buildDueSchedule merges the expected periods for a member with their recorded payments.
// When the member has an end date in the past, dues are only generated up to that
// date and the final period counts as overdue once unpaid (the member has left);
// otherwise the current (ongoing) month is shown as "waiting", not overdue.
func buildDueSchedule(m Member, payments []Payment, now time.Time) []DuePeriod {
	paidByPeriod := make(map[string]Payment, len(payments))
	for _, p := range payments {
		paidByPeriod[p.Period] = p
	}

	cutoff := now
	memberEnded := false
	if m.EndDate != "" {
		if end, err := time.Parse("2006-01-02", m.EndDate); err == nil && end.Before(now) {
			cutoff = end
			memberEnded = true
		}
	}
	cutoffPeriod := cutoff.Format("2006-01")

	var schedule []DuePeriod
	for _, period := range periodsInRange(m.JoinDate, cutoff) {
		dp := DuePeriod{Period: period, Amount: m.MonthlyFee}
		if p, ok := paidByPeriod[period]; ok {
			dp.Paid = true
			dp.PaidDate = p.PaidDate
			dp.Amount = p.Amount
			dp.Method = p.Method
		} else if period < cutoffPeriod || (period == cutoffPeriod && memberEnded) {
			dp.Overdue = true
		}
		schedule = append(schedule, dp)
	}
	// Show most recent period first.
	for i, j := 0, len(schedule)-1; i < j; i, j = i+1, j-1 {
		schedule[i], schedule[j] = schedule[j], schedule[i]
	}
	return schedule
}
