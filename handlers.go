package main

import (
	"database/sql"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type Server struct {
	db       *sql.DB
	tmpl     *template.Template
	staticFS http.Handler
}

func newServer(db *sql.DB, tmpl *template.Template, staticFS http.Handler) *Server {
	return &Server{db: db, tmpl: tmpl, staticFS: staticFS}
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/static/", s.staticFS)
	mux.HandleFunc("GET /{$}", s.handleDashboard)
	mux.HandleFunc("GET /raporlar", s.handleReports)
	mux.HandleFunc("GET /uyeler", s.handleMembersList)
	mux.HandleFunc("GET /uyeler/yeni", s.handleMemberNewForm)
	mux.HandleFunc("POST /uyeler", s.handleMemberCreate)
	mux.HandleFunc("GET /uyeler/silinmis", s.handleTrashList)
	mux.HandleFunc("GET /uyeler/{id}", s.handleMemberDetail)
	mux.HandleFunc("GET /uyeler/{id}/duzenle", s.handleMemberEditForm)
	mux.HandleFunc("POST /uyeler/{id}/duzenle", s.handleMemberUpdate)
	mux.HandleFunc("POST /uyeler/{id}/durum", s.handleMemberToggleStatus)
	mux.HandleFunc("POST /uyeler/{id}/sil", s.handleMemberDelete)
	mux.HandleFunc("POST /uyeler/{id}/geri-yukle", s.handleMemberRestore)
	mux.HandleFunc("POST /uyeler/{id}/kalici-sil", s.handleMemberPurge)
	mux.HandleFunc("POST /uyeler/{id}/donem", s.handlePeriodCreate)
	mux.HandleFunc("POST /uyeler/{id}/aidat-guncelle", s.handleFeeChange)
	mux.HandleFunc("GET /uyeler/{id}/donem/{pid}/duzenle", s.handlePeriodEditForm)
	mux.HandleFunc("POST /uyeler/{id}/donem/{pid}/duzenle", s.handlePeriodUpdate)
	mux.HandleFunc("POST /uyeler/{id}/donem/{pid}/sil", s.handlePeriodDelete)
	mux.HandleFunc("POST /uyeler/{id}/odeme", s.handlePaymentCreate)
	mux.HandleFunc("GET /odeme/{id}/duzenle", s.handlePaymentEditForm)
	mux.HandleFunc("POST /odeme/{id}/duzenle", s.handlePaymentUpdate)
	mux.HandleFunc("POST /odeme/{id}/sil", s.handlePaymentDelete)
	return mux
}

func (s *Server) render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("template hatası (%s): %v", name, err)
		http.Error(w, "Bir hata oluştu", http.StatusInternalServerError)
	}
}

func idFromPath(r *http.Request) (int64, error) {
	return strconv.ParseInt(r.PathValue("id"), 10, 64)
}

func pidFromPath(r *http.Request) (int64, error) {
	return strconv.ParseInt(r.PathValue("pid"), 10, 64)
}

// redirectWithToast redirects to path, appending query params the front-end
// reads on load to pop a toastr notification (see web/static/app.js).
func redirectWithToast(w http.ResponseWriter, r *http.Request, path, message, toastType string) {
	u := path + "?toast=" + url.QueryEscape(message) + "&toast_type=" + toastType
	http.Redirect(w, r, u, http.StatusSeeOther)
}

func memberOverdue(periods []MembershipPeriod, payments []Payment, now time.Time) (schedule []DuePeriod, overdue []DuePeriod, overdueTotal float64) {
	schedule = buildDueSchedule(periods, payments, now)
	for _, dp := range schedule {
		if dp.Overdue {
			overdue = append(overdue, dp)
			overdueTotal += dp.Amount
		}
	}
	return
}

// ---- Dashboard ----

type overdueRow struct {
	Member  Member
	Periods []DuePeriod
	Total   float64
	WALink  string
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	members, err := listMembers(s.db, true)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	currentPeriod := now.Format("2006-01")
	collectedThisMonth, err := paymentsTotalForPeriod(s.db, currentPeriod)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	var activeCount, passiveCount int
	var overdueRows []overdueRow
	var overdueTotal float64
	for _, m := range members {
		if m.IsActive() {
			activeCount++
		} else {
			passiveCount++
			continue
		}
		periods, err := listPeriodsForMember(s.db, m.ID)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		payments, err := listPaymentsForMember(s.db, m.ID)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		_, due, memberTotal := memberOverdue(periods, payments, now)
		if len(due) > 0 {
			link := whatsAppLink(m.ReminderPhone(), buildReminderMessage(m, due, memberTotal))
			overdueRows = append(overdueRows, overdueRow{Member: m, Periods: due, Total: memberTotal, WALink: link})
			overdueTotal += memberTotal
		}
	}

	s.render(w, "dashboard", map[string]any{
		"ActiveCount":        activeCount,
		"PassiveCount":       passiveCount,
		"TotalCount":         len(members),
		"CollectedThisMonth": collectedThisMonth,
		"CurrentPeriod":      currentPeriod,
		"OverdueRows":        overdueRows,
		"OverdueTotal":       overdueTotal,
		"OverdueMemberCount": len(overdueRows),
	})
}

// ---- Reports ----

func (s *Server) handleReports(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	currentPeriod := now.Format("2006-01")
	var periods []string
	cur := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 12; i++ {
		periods = append([]string{cur.Format("2006-01")}, periods...)
		cur = cur.AddDate(0, -1, 0)
	}

	totals, err := monthlyTotals(s.db, periods)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	type row struct {
		Period  string
		Total   float64
		Percent float64
		Current bool
	}
	var rows []row
	var grandTotal, maxTotal float64
	for _, p := range periods {
		t := totals[p]
		grandTotal += t
		if t > maxTotal {
			maxTotal = t
		}
		rows = append(rows, row{Period: p, Total: t})
	}
	bestPeriod, bestTotal := "", 0.0
	for i := range rows {
		if maxTotal > 0 {
			rows[i].Percent = rows[i].Total / maxTotal * 100
		}
		if rows[i].Period == currentPeriod {
			rows[i].Current = true
		}
		if rows[i].Total > bestTotal {
			bestTotal, bestPeriod = rows[i].Total, rows[i].Period
		}
	}
	// Chart reads best top-to-bottom as most-recent-first.
	chartRows := make([]row, len(rows))
	for i, r := range rows {
		chartRows[len(rows)-1-i] = r
	}

	members, err := listMembers(s.db, true)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	var activeCount int
	for _, m := range members {
		if m.IsActive() {
			activeCount++
		}
	}

	s.render(w, "reports", map[string]any{
		"Rows":              rows,
		"ChartRows":         chartRows,
		"GrandTotal":        grandTotal,
		"AverageMonthly":    grandTotal / float64(len(rows)),
		"BestPeriod":        bestPeriod,
		"BestTotal":         bestTotal,
		"CurrentMonthTotal": totals[currentPeriod],
		"CurrentPeriod":     currentPeriod,
		"ActiveMemberCount": activeCount,
		"TotalMemberCount":  len(members),
	})
}

// ---- Members ----

type memberRow struct {
	Member
	JoinDate string
	Fee      float64
	Ongoing  bool
}

func (s *Server) handleMembersList(w http.ResponseWriter, r *http.Request) {
	members, err := listMembers(s.db, true)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	rows := make([]memberRow, 0, len(members))
	for _, m := range members {
		periods, err := listPeriodsForMember(s.db, m.ID)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		latest, hasLatest := latestPeriod(periods)
		rows = append(rows, memberRow{
			Member:   m,
			JoinDate: firstJoinDate(periods),
			Fee:      currentFee(periods, time.Now()),
			Ongoing:  hasLatest && latest.IsOngoing(),
		})
	}
	s.render(w, "members_list", map[string]any{"Members": rows})
}

func (s *Server) handleMemberNewForm(w http.ResponseWriter, r *http.Request) {
	s.render(w, "member_form", map[string]any{
		"IsNew":     true,
		"Member":    Member{Status: "active"},
		"FormURL":   "/uyeler",
		"StartDate": time.Now().Format("2006-01-02"),
	})
}

func parseMemberForm(r *http.Request) (Member, error) {
	if err := r.ParseForm(); err != nil {
		return Member{}, err
	}
	name := r.FormValue("full_name")
	if name == "" {
		return Member{}, errors.New("ad soyad zorunludur")
	}
	return Member{
		FullName:      name,
		Phone:         r.FormValue("phone"),
		Email:         r.FormValue("email"),
		Status:        "active",
		Note:          r.FormValue("note"),
		GuardianName:  r.FormValue("guardian_name"),
		GuardianPhone: r.FormValue("guardian_phone"),
	}, nil
}

// parsePeriodForm reads start_date/end_date/monthly_fee/note, shared by the
// initial period created alongside a new member and the standalone period form.
func parsePeriodForm(r *http.Request) (MembershipPeriod, error) {
	if err := r.ParseForm(); err != nil {
		return MembershipPeriod{}, err
	}
	fee, err := strconv.ParseFloat(r.FormValue("monthly_fee"), 64)
	if err != nil {
		return MembershipPeriod{}, errors.New("aidat tutarı geçersiz")
	}
	startDate := r.FormValue("start_date")
	if startDate == "" {
		startDate = time.Now().Format("2006-01-02")
	}
	endDate := r.FormValue("end_date")
	if endDate != "" && endDate < startDate {
		return MembershipPeriod{}, errors.New("bitiş tarihi başlangıç tarihinden önce olamaz")
	}
	return MembershipPeriod{
		StartDate:  startDate,
		EndDate:    endDate,
		MonthlyFee: fee,
		Note:       r.FormValue("period_note"),
	}, nil
}

func (s *Server) handleMemberCreate(w http.ResponseWriter, r *http.Request) {
	m, err := parseMemberForm(r)
	if err != nil {
		s.render(w, "member_form", map[string]any{
			"IsNew": true, "Member": m, "FormURL": "/uyeler", "Error": err.Error(),
			"StartDate": time.Now().Format("2006-01-02"),
		})
		return
	}
	period, err := parsePeriodForm(r)
	if err != nil {
		s.render(w, "member_form", map[string]any{
			"IsNew": true, "Member": m, "FormURL": "/uyeler", "Error": err.Error(),
			"StartDate": r.FormValue("start_date"),
		})
		return
	}
	id, err := createMember(s.db, m)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	period.MemberID = id
	if _, err := createPeriod(s.db, period); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	redirectWithToast(w, r, "/uyeler/"+strconv.FormatInt(id, 10), "Üye eklendi.", "success")
}

func (s *Server) handleMemberDetail(w http.ResponseWriter, r *http.Request) {
	id, err := idFromPath(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	m, err := getMember(s.db, id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	periods, err := listPeriodsForMember(s.db, id)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	payments, err := listPaymentsForMember(s.db, id)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	now := time.Now()
	schedule, overdue, overdueTotal := memberOverdue(periods, payments, now)
	upcoming, hasUpcoming := upcomingPeriod(periods, now)

	var waLink string
	if len(overdue) > 0 {
		waLink = whatsAppLink(m.ReminderPhone(), buildReminderMessage(m, overdue, overdueTotal))
	}

	// Sort periods most-recent-start first for display.
	displayPeriods := make([]MembershipPeriod, len(periods))
	copy(displayPeriods, periods)
	for i, j := 0, len(displayPeriods)-1; i < j; i, j = i+1, j-1 {
		displayPeriods[i], displayPeriods[j] = displayPeriods[j], displayPeriods[i]
	}

	s.render(w, "member_detail", map[string]any{
		"Member":        m,
		"Periods":       displayPeriods,
		"Schedule":      schedule,
		"CurrentPeriod": now.Format("2006-01"),
		"CurrentFee":    currentFee(periods, now),
		"HasUpcoming":   hasUpcoming,
		"Upcoming":      upcoming,
		"FirstJoinDate": firstJoinDate(periods),
		"OverdueTotal":  overdueTotal,
		"WALink":        waLink,
	})
}

func (s *Server) handleMemberEditForm(w http.ResponseWriter, r *http.Request) {
	id, err := idFromPath(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	m, err := getMember(s.db, id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	s.render(w, "member_form", map[string]any{
		"IsNew":   false,
		"Member":  m,
		"FormURL": "/uyeler/" + strconv.FormatInt(id, 10) + "/duzenle",
	})
}

func (s *Server) handleMemberUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := idFromPath(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	m, err := parseMemberForm(r)
	if err != nil {
		m.ID = id
		s.render(w, "member_form", map[string]any{
			"IsNew": false, "Member": m, "FormURL": "/uyeler/" + strconv.FormatInt(id, 10) + "/duzenle", "Error": err.Error(),
		})
		return
	}
	m.ID = id
	if err := updateMember(s.db, m); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	redirectWithToast(w, r, "/uyeler/"+strconv.FormatInt(id, 10), "Değişiklikler kaydedildi.", "success")
}

func (s *Server) handleMemberToggleStatus(w http.ResponseWriter, r *http.Request) {
	id, err := idFromPath(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	m, err := getMember(s.db, id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	newStatus := "passive"
	label := "Üye pasif yapıldı."
	if m.Status == "passive" {
		newStatus = "active"
		label = "Üye aktif yapıldı."
	}
	if err := setMemberStatus(s.db, id, newStatus); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	redirectWithToast(w, r, "/uyeler/"+strconv.FormatInt(id, 10), label, "success")
}

func (s *Server) handleMemberDelete(w http.ResponseWriter, r *http.Request) {
	id, err := idFromPath(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := softDeleteMember(s.db, id, time.Now().Format(time.RFC3339)); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	redirectWithToast(w, r, "/uyeler", "Üye çöp kutusuna taşındı.", "success")
}

func (s *Server) handleMemberRestore(w http.ResponseWriter, r *http.Request) {
	id, err := idFromPath(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := restoreMember(s.db, id); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	redirectWithToast(w, r, "/uyeler/"+strconv.FormatInt(id, 10), "Üye geri yüklendi.", "success")
}

func (s *Server) handleMemberPurge(w http.ResponseWriter, r *http.Request) {
	id, err := idFromPath(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := purgeMember(s.db, id); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	redirectWithToast(w, r, "/uyeler/silinmis", "Üye kalıcı olarak silindi.", "success")
}

// ---- Trash ----

func (s *Server) handleTrashList(w http.ResponseWriter, r *http.Request) {
	members, err := listDeletedMembers(s.db)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	s.render(w, "members_trash", map[string]any{"Members": members})
}

// ---- Membership periods ----

func (s *Server) handlePeriodCreate(w http.ResponseWriter, r *http.Request) {
	id, err := idFromPath(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	period, err := parsePeriodForm(r)
	memberURL := "/uyeler/" + strconv.FormatInt(id, 10)
	if err != nil {
		redirectWithToast(w, r, memberURL, err.Error(), "error")
		return
	}
	period.MemberID = id
	if _, err := createPeriod(s.db, period); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	redirectWithToast(w, r, memberURL, "Yeni dönem eklendi.", "success")
}

func (s *Server) handleFeeChange(w http.ResponseWriter, r *http.Request) {
	id, err := idFromPath(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	memberURL := "/uyeler/" + strconv.FormatInt(id, 10)
	if err := r.ParseForm(); err != nil {
		redirectWithToast(w, r, memberURL, err.Error(), "error")
		return
	}
	effectiveMonth := r.FormValue("effective_month")
	fee, err := strconv.ParseFloat(r.FormValue("monthly_fee"), 64)
	if err != nil || fee < 0 {
		redirectWithToast(w, r, memberURL, "Aidat tutarı geçersiz.", "error")
		return
	}
	if err := applyFeeChange(s.db, id, effectiveMonth, fee); err != nil {
		redirectWithToast(w, r, memberURL, err.Error(), "error")
		return
	}
	msg := fmt.Sprintf("Aidat %s itibarıyla %.2f ₺ olarak güncellendi.", periodLabelText(effectiveMonth), fee)
	redirectWithToast(w, r, memberURL, msg, "success")
}

func (s *Server) handlePeriodEditForm(w http.ResponseWriter, r *http.Request) {
	id, err := idFromPath(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	pid, err := pidFromPath(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	p, err := getPeriod(s.db, pid)
	if err != nil || p.MemberID != id {
		http.NotFound(w, r)
		return
	}
	m, err := getMember(s.db, id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	s.render(w, "period_form", map[string]any{
		"Member":  m,
		"Period":  p,
		"FormURL": "/uyeler/" + strconv.FormatInt(id, 10) + "/donem/" + strconv.FormatInt(pid, 10) + "/duzenle",
	})
}

func (s *Server) handlePeriodUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := idFromPath(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	pid, err := pidFromPath(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	existing, err := getPeriod(s.db, pid)
	if err != nil || existing.MemberID != id {
		http.NotFound(w, r)
		return
	}
	period, err := parsePeriodForm(r)
	if err != nil {
		m, _ := getMember(s.db, id)
		period.ID = pid
		period.MemberID = id
		s.render(w, "period_form", map[string]any{
			"Member": m, "Period": period,
			"FormURL": "/uyeler/" + strconv.FormatInt(id, 10) + "/donem/" + strconv.FormatInt(pid, 10) + "/duzenle",
			"Error":   err.Error(),
		})
		return
	}
	period.ID = pid
	period.MemberID = id
	if err := updatePeriod(s.db, period); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	redirectWithToast(w, r, "/uyeler/"+strconv.FormatInt(id, 10), "Dönem güncellendi.", "success")
}

func (s *Server) handlePeriodDelete(w http.ResponseWriter, r *http.Request) {
	id, err := idFromPath(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	pid, err := pidFromPath(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := deletePeriod(s.db, pid); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	redirectWithToast(w, r, "/uyeler/"+strconv.FormatInt(id, 10), "Dönem silindi.", "success")
}

// ---- Payments ----

func parsePaymentForm(r *http.Request) (Payment, error) {
	if err := r.ParseForm(); err != nil {
		return Payment{}, err
	}
	amount, err := strconv.ParseFloat(r.FormValue("amount"), 64)
	if err != nil {
		return Payment{}, errors.New("geçersiz tutar")
	}
	period := r.FormValue("period")
	if period == "" {
		return Payment{}, errors.New("dönem seçilmelidir")
	}
	paidDate := r.FormValue("paid_date")
	if paidDate == "" {
		paidDate = time.Now().Format("2006-01-02")
	}
	method := r.FormValue("method")
	if method != "eft" {
		method = "nakit"
	}
	return Payment{
		Period:   period,
		Amount:   amount,
		PaidDate: paidDate,
		Method:   method,
		Note:     r.FormValue("note"),
	}, nil
}

func (s *Server) handlePaymentCreate(w http.ResponseWriter, r *http.Request) {
	id, err := idFromPath(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	memberURL := "/uyeler/" + strconv.FormatInt(id, 10)
	p, err := parsePaymentForm(r)
	if err != nil {
		redirectWithToast(w, r, memberURL, err.Error(), "error")
		return
	}
	p.MemberID = id
	if err := recordPayment(s.db, p); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	redirectWithToast(w, r, memberURL, "Ödeme kaydedildi.", "success")
}

func (s *Server) handlePaymentEditForm(w http.ResponseWriter, r *http.Request) {
	id, err := idFromPath(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	p, err := getPayment(s.db, id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	m, err := getMember(s.db, p.MemberID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	s.render(w, "payment_form", map[string]any{
		"Member":  m,
		"Payment": p,
		"FormURL": "/odeme/" + strconv.FormatInt(id, 10) + "/duzenle",
	})
}

func (s *Server) handlePaymentUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := idFromPath(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	existing, err := getPayment(s.db, id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	p, err := parsePaymentForm(r)
	if err != nil {
		m, _ := getMember(s.db, existing.MemberID)
		p.ID = id
		p.MemberID = existing.MemberID
		s.render(w, "payment_form", map[string]any{
			"Member": m, "Payment": p,
			"FormURL": "/odeme/" + strconv.FormatInt(id, 10) + "/duzenle",
			"Error":   err.Error(),
		})
		return
	}
	memberURL := "/uyeler/" + strconv.FormatInt(existing.MemberID, 10)
	if p.Period != existing.Period {
		// Period changed: delete the old row first so the unique(member_id, period)
		// constraint doesn't collide, then insert fresh under the new period.
		if err := deletePayment(s.db, id); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
	}
	p.MemberID = existing.MemberID
	if err := recordPayment(s.db, p); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	redirectWithToast(w, r, memberURL, "Ödeme güncellendi.", "success")
}

func (s *Server) handlePaymentDelete(w http.ResponseWriter, r *http.Request) {
	id, err := idFromPath(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	memberID := r.FormValue("member_id")
	if err := deletePayment(s.db, id); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	redirectWithToast(w, r, "/uyeler/"+memberID, "Ödeme kaydı silindi.", "success")
}
