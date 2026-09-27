package main

import (
	"sort"
	"time"
)

type Member struct {
	ID            int64
	FullName      string
	Phone         string
	Email         string
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

// MembershipPeriod is one continuous stint of membership with its own fee.
// A member can have several, non-contiguous periods over time: closing one
// (setting EndDate) and later adding a new one models a member who left and
// came back, or a fee change, without touching earlier periods or their
// payment history.
type MembershipPeriod struct {
	ID         int64
	MemberID   int64
	StartDate  string // YYYY-MM-DD
	EndDate    string // YYYY-MM-DD, boş = hâlâ devam ediyor
	MonthlyFee float64
	Note       string
}

// IsOngoing reports whether this period has no end date, i.e. is still active.
func (p MembershipPeriod) IsOngoing() bool { return p.EndDate == "" }

// HasEnded reports whether the period's end date has already passed as of now.
func (p MembershipPeriod) HasEnded(now time.Time) bool {
	if p.EndDate == "" {
		return false
	}
	end, err := time.Parse("2006-01-02", p.EndDate)
	if err != nil {
		return false
	}
	return end.Before(now)
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
	Period    string // YYYY-MM
	Amount    float64
	Paid      bool
	PaymentID int64
	PaidDate  string
	Method    string
	Overdue   bool
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

// monthsInRange returns "YYYY-MM" strings from start's month up to and including cutoff's month.
func monthsInRange(startDate string, cutoff time.Time) []string {
	start, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		return nil
	}
	cur := time.Date(start.Year(), start.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(cutoff.Year(), cutoff.Month(), 1, 0, 0, 0, 0, time.UTC)
	var months []string
	for !cur.After(end) {
		months = append(months, cur.Format("2006-01"))
		cur = cur.AddDate(0, 1, 0)
	}
	return months
}

// buildDueSchedule merges every membership period's expected months with the
// member's recorded payments. Each period contributes only the months it
// actually covers, so a gap between two periods (left, then rejoined later)
// never shows up as owed, and a fee change is just a new period starting the
// month the new price applies. A closed period's last month counts as
// overdue once unpaid (the member has left); an ongoing period's current
// month is shown as "waiting" instead.
func buildDueSchedule(periods []MembershipPeriod, payments []Payment, now time.Time) []DuePeriod {
	paidByPeriod := make(map[string]Payment, len(payments))
	for _, p := range payments {
		paidByPeriod[p.Period] = p
	}

	byMonth := make(map[string]DuePeriod)
	for _, period := range periods {
		cutoff := now
		ended := false
		if period.EndDate != "" {
			if end, err := time.Parse("2006-01-02", period.EndDate); err == nil && end.Before(now) {
				cutoff = end
				ended = true
			}
		}
		cutoffMonth := cutoff.Format("2006-01")

		for _, month := range monthsInRange(period.StartDate, cutoff) {
			dp := DuePeriod{Period: month, Amount: period.MonthlyFee}
			if p, ok := paidByPeriod[month]; ok {
				dp.Paid = true
				dp.PaymentID = p.ID
				dp.PaidDate = p.PaidDate
				dp.Amount = p.Amount
				dp.Method = p.Method
			} else if month < cutoffMonth || (month == cutoffMonth && ended) {
				dp.Overdue = true
			}
			byMonth[month] = dp
		}
	}

	schedule := make([]DuePeriod, 0, len(byMonth))
	for _, dp := range byMonth {
		schedule = append(schedule, dp)
	}
	sort.Slice(schedule, func(i, j int) bool { return schedule[i].Period > schedule[j].Period })
	return schedule
}

// currentFee returns the monthly fee that applies right now: the ongoing
// period's fee if there is one, otherwise the most recently started
// period's fee (so forms have a sensible default even for a passive member).
func currentFee(periods []MembershipPeriod) float64 {
	p, ok := latestPeriod(periods)
	if !ok {
		return 0
	}
	return p.MonthlyFee
}

// latestPeriod returns the ongoing period if any, else the one with the most
// recent start date.
func latestPeriod(periods []MembershipPeriod) (MembershipPeriod, bool) {
	if len(periods) == 0 {
		return MembershipPeriod{}, false
	}
	best := periods[0]
	for _, p := range periods[1:] {
		if p.IsOngoing() && !best.IsOngoing() {
			best = p
			continue
		}
		if p.IsOngoing() == best.IsOngoing() && p.StartDate > best.StartDate {
			best = p
		}
	}
	return best, true
}

// firstJoinDate returns the earliest period's start date, for display as
// "İlk Katılım".
func firstJoinDate(periods []MembershipPeriod) string {
	if len(periods) == 0 {
		return ""
	}
	first := periods[0].StartDate
	for _, p := range periods[1:] {
		if p.StartDate < first {
			first = p.StartDate
		}
	}
	return first
}
