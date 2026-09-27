package main

import (
	"database/sql"
	"errors"
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
	mux.HandleFunc("POST /uyeler/{id}/odeme", s.handlePaymentCreate)
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

// redirectWithToast redirects to path, appending query params the front-end
// reads on load to pop a toastr notification (see web/static/app.js).
func redirectWithToast(w http.ResponseWriter, r *http.Request, path, message, toastType string) {
	u := path + "?toast=" + url.QueryEscape(message) + "&toast_type=" + toastType
	http.Redirect(w, r, u, http.StatusSeeOther)
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
		payments, err := listPaymentsForMember(s.db, m.ID)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		schedule := buildDueSchedule(m, payments, now)
		var due []DuePeriod
		var memberTotal float64
		for _, dp := range schedule {
			if dp.Overdue {
				due = append(due, dp)
				memberTotal += dp.Amount
			}
		}
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
		Period string
		Total  float64
	}
	var rows []row
	var grandTotal float64
	for _, p := range periods {
		t := totals[p]
		grandTotal += t
		rows = append(rows, row{Period: p, Total: t})
	}

	s.render(w, "reports", map[string]any{
		"Rows":       rows,
		"GrandTotal": grandTotal,
	})
}

// ---- Members ----

func (s *Server) handleMembersList(w http.ResponseWriter, r *http.Request) {
	members, err := listMembers(s.db, true)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	s.render(w, "members_list", map[string]any{"Members": members})
}

func (s *Server) handleMemberNewForm(w http.ResponseWriter, r *http.Request) {
	s.render(w, "member_form", map[string]any{
		"IsNew":   true,
		"Member":  Member{JoinDate: time.Now().Format("2006-01-02"), Status: "active"},
		"FormURL": "/uyeler",
	})
}

func parseMemberForm(r *http.Request) (Member, error) {
	if err := r.ParseForm(); err != nil {
		return Member{}, err
	}
	fee, err := strconv.ParseFloat(r.FormValue("monthly_fee"), 64)
	if err != nil {
		return Member{}, errors.New("aidat tutarı geçersiz")
	}
	name := r.FormValue("full_name")
	if name == "" {
		return Member{}, errors.New("ad soyad zorunludur")
	}
	joinDate := r.FormValue("join_date")
	if joinDate == "" {
		joinDate = time.Now().Format("2006-01-02")
	}
	endDate := r.FormValue("end_date")
	if endDate != "" && endDate < joinDate {
		return Member{}, errors.New("bitiş tarihi başlangıç tarihinden önce olamaz")
	}
	return Member{
		FullName:      name,
		Phone:         r.FormValue("phone"),
		Email:         r.FormValue("email"),
		JoinDate:      joinDate,
		EndDate:       endDate,
		MonthlyFee:    fee,
		Status:        "active",
		Note:          r.FormValue("note"),
		GuardianName:  r.FormValue("guardian_name"),
		GuardianPhone: r.FormValue("guardian_phone"),
	}, nil
}

func (s *Server) handleMemberCreate(w http.ResponseWriter, r *http.Request) {
	m, err := parseMemberForm(r)
	if err != nil {
		s.render(w, "member_form", map[string]any{
			"IsNew": true, "Member": m, "FormURL": "/uyeler", "Error": err.Error(),
		})
		return
	}
	id, err := createMember(s.db, m)
	if err != nil {
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
	payments, err := listPaymentsForMember(s.db, id)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	schedule := buildDueSchedule(m, payments, time.Now())

	var overdue []DuePeriod
	var overdueTotal float64
	for _, dp := range schedule {
		if dp.Overdue {
			overdue = append(overdue, dp)
			overdueTotal += dp.Amount
		}
	}
	var waLink string
	if len(overdue) > 0 {
		waLink = whatsAppLink(m.ReminderPhone(), buildReminderMessage(m, overdue, overdueTotal))
	}

	s.render(w, "member_detail", map[string]any{
		"Member":        m,
		"Schedule":      schedule,
		"CurrentPeriod": time.Now().Format("2006-01"),
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

// ---- Payments ----

func (s *Server) handlePaymentCreate(w http.ResponseWriter, r *http.Request) {
	id, err := idFromPath(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	amount, err := strconv.ParseFloat(r.FormValue("amount"), 64)
	if err != nil {
		http.Error(w, "geçersiz tutar", 400)
		return
	}
	period := r.FormValue("period")
	paidDate := r.FormValue("paid_date")
	if paidDate == "" {
		paidDate = time.Now().Format("2006-01-02")
	}
	method := r.FormValue("method")
	if method != "eft" {
		method = "nakit"
	}
	p := Payment{
		MemberID: id,
		Period:   period,
		Amount:   amount,
		PaidDate: paidDate,
		Method:   method,
		Note:     r.FormValue("note"),
	}
	if err := recordPayment(s.db, p); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	redirectWithToast(w, r, "/uyeler/"+strconv.FormatInt(id, 10), "Ödeme kaydedildi.", "success")
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
