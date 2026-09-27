package main

import (
	"path/filepath"
	"testing"
	"time"
)

func TestPlanFeeChangeSplitsOngoingPeriod(t *testing.T) {
	periods := []MembershipPeriod{
		{ID: 1, MemberID: 7, StartDate: "2026-01-15", MonthlyFee: 3000},
	}
	plan, err := planFeeChange(periods, "2026-07", 5000)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Update) != 1 || plan.Update[0].EndDate != "2026-06-30" || plan.Update[0].MonthlyFee != 3000 {
		t.Fatalf("old period should end 2026-06-30 at 3000, got %+v", plan.Update)
	}
	if len(plan.Insert) != 1 {
		t.Fatalf("want 1 new period, got %d", len(plan.Insert))
	}
	n := plan.Insert[0]
	if n.StartDate != "2026-07-01" || n.EndDate != "" || n.MonthlyFee != 5000 || n.MemberID != 7 {
		t.Fatalf("unexpected new period %+v", n)
	}
}

func TestPlanFeeChangeKeepsOriginalEndDate(t *testing.T) {
	periods := []MembershipPeriod{
		{ID: 1, StartDate: "2026-01-01", EndDate: "2026-12-31", MonthlyFee: 3000},
	}
	plan, err := planFeeChange(periods, "2026-07", 5000)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Insert[0].EndDate != "2026-12-31" {
		t.Fatalf("new period should keep the original end date, got %q", plan.Insert[0].EndDate)
	}
}

func TestPlanFeeChangeFromStartMonthUpdatesInPlace(t *testing.T) {
	periods := []MembershipPeriod{
		{ID: 1, StartDate: "2026-07-01", MonthlyFee: 3000},
	}
	plan, err := planFeeChange(periods, "2026-07", 5000)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Insert) != 0 || len(plan.Update) != 1 || plan.Update[0].MonthlyFee != 5000 || plan.Update[0].StartDate != "2026-07-01" {
		t.Fatalf("expected an in-place fee update, got %+v", plan)
	}
}

func TestPlanFeeChangeLeavesEarlierPeriodsAndUpdatesLaterOnes(t *testing.T) {
	periods := []MembershipPeriod{
		{ID: 1, StartDate: "2024-01-01", EndDate: "2024-12-31", MonthlyFee: 2000},
		{ID: 2, StartDate: "2026-01-01", EndDate: "2026-05-31", MonthlyFee: 3000},
		{ID: 3, StartDate: "2026-09-01", MonthlyFee: 3000},
	}
	plan, err := planFeeChange(periods, "2026-03", 4000)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range plan.Update {
		if p.ID == 1 {
			t.Fatal("period that ended before the effective month must not change")
		}
	}
	if len(plan.Update) != 2 || len(plan.Insert) != 1 {
		t.Fatalf("want 2 updates + 1 insert, got %+v", plan)
	}
}

func TestPlanFeeChangeRejectsMonthWithNoPeriod(t *testing.T) {
	periods := []MembershipPeriod{
		{ID: 1, StartDate: "2024-01-01", EndDate: "2024-12-31", MonthlyFee: 2000},
	}
	if _, err := planFeeChange(periods, "2026-01", 5000); err != errNoPeriodForFeeChange {
		t.Fatalf("want errNoPeriodForFeeChange, got %v", err)
	}
}

func TestApplyFeeChangeRecalculatesOnlyUnpaidMonthsFromEffectiveMonth(t *testing.T) {
	db, err := openDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	memberID, err := createMember(db, Member{FullName: "Test", Status: "active"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := createPeriod(db, MembershipPeriod{MemberID: memberID, StartDate: "2026-01-01", MonthlyFee: 3000}); err != nil {
		t.Fatal(err)
	}
	// July is already paid at the old price; that must stay as recorded.
	if err := recordPayment(db, Payment{MemberID: memberID, Period: "2026-07", Amount: 3000, PaidDate: "2026-07-03", Method: "nakit"}); err != nil {
		t.Fatal(err)
	}

	if err := applyFeeChange(db, memberID, "2026-06", 5000); err != nil {
		t.Fatal(err)
	}

	periods, err := listPeriodsForMember(db, memberID)
	if err != nil {
		t.Fatal(err)
	}
	payments, err := listPaymentsForMember(db, memberID)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	byMonth := map[string]DuePeriod{}
	for _, dp := range buildDueSchedule(periods, payments, now) {
		byMonth[dp.Period] = dp
	}

	want := map[string]float64{
		"2026-01": 3000, "2026-05": 3000, // before the change
		"2026-06": 5000, "2026-08": 5000, "2026-09": 5000, // from the change onward
		"2026-07": 3000, // already paid, keeps the paid amount
	}
	for month, amount := range want {
		if got := byMonth[month].Amount; got != amount {
			t.Errorf("%s: want %.0f, got %.0f", month, amount, got)
		}
	}
	if len(byMonth) != 9 {
		t.Errorf("want 9 months Jan–Sep with no gaps or duplicates, got %d", len(byMonth))
	}
	if got := currentFee(periods, now); got != 5000 {
		t.Errorf("current fee: want 5000, got %.0f", got)
	}
}

func TestCurrentFeeIgnoresFutureChange(t *testing.T) {
	periods := []MembershipPeriod{
		{StartDate: "2026-01-01", EndDate: "2026-09-30", MonthlyFee: 3000},
		{StartDate: "2026-10-01", MonthlyFee: 5000},
	}
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	if got := currentFee(periods, now); got != 3000 {
		t.Fatalf("fee scheduled for next month must not be current yet, got %.0f", got)
	}
	up, ok := upcomingPeriod(periods, now)
	if !ok || up.MonthlyFee != 5000 || up.StartDate != "2026-10-01" {
		t.Fatalf("expected upcoming 5000 from 2026-10-01, got %+v %v", up, ok)
	}
}
