package main

import (
	"html/template"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func newTestServer(t *testing.T) (*Server, http.Handler) {
	t.Helper()
	tmpl, err := template.New("").Funcs(funcMap).ParseFS(templatesFS, "web/templates/*.html")
	if err != nil {
		t.Fatal(err)
	}
	db, err := openDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	staticSub, err := fs.Sub(staticFS, "web/static")
	if err != nil {
		t.Fatal(err)
	}
	srv := newServer(db, tmpl, http.StripPrefix("/static/", http.FileServerFS(staticSub)))
	return srv, srv.routes()
}

func TestFeeChangeEndpointAndDetailPage(t *testing.T) {
	srv, h := newTestServer(t)
	id, err := createMember(srv.db, Member{FullName: "Deneme Üye", Status: "active"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := createPeriod(srv.db, MembershipPeriod{MemberID: id, StartDate: "2026-01-01", MonthlyFee: 3000}); err != nil {
		t.Fatal(err)
	}
	base := "/uyeler/" + strconv.FormatInt(id, 10)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", base, nil))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "data-fee-update") {
		t.Fatalf("detail page should render the Aidat Güncelle form, status %d", rec.Code)
	}

	form := url.Values{"effective_month": {"2026-06"}, "monthly_fee": {"5000"}}
	req := httptest.NewRequest("POST", base+"/aidat-guncelle", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther || !strings.Contains(rec.Header().Get("Location"), "toast_type=success") {
		t.Fatalf("fee change should redirect with a success toast, got %d %q", rec.Code, rec.Header().Get("Location"))
	}

	periods, _ := listPeriodsForMember(srv.db, id)
	if len(periods) != 2 || periods[0].EndDate != "2026-05-31" || periods[1].StartDate != "2026-06-01" || periods[1].MonthlyFee != 5000 {
		t.Fatalf("unexpected periods after fee change: %+v", periods)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", base, nil))
	if rec.Code != 200 {
		t.Fatalf("detail page after fee change: status %d", rec.Code)
	}

	// A month after every period has ended is rejected.
	closedID, err := createMember(srv.db, Member{FullName: "Ayrılmış Üye", Status: "passive"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := createPeriod(srv.db, MembershipPeriod{MemberID: closedID, StartDate: "2024-01-01", EndDate: "2024-12-31", MonthlyFee: 2000}); err != nil {
		t.Fatal(err)
	}
	form = url.Values{"effective_month": {"2026-01"}, "monthly_fee": {"9000"}}
	req = httptest.NewRequest("POST", "/uyeler/"+strconv.FormatInt(closedID, 10)+"/aidat-guncelle", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if !strings.Contains(rec.Header().Get("Location"), "toast_type=error") {
		t.Fatalf("expected an error toast for a month before any period, got %q", rec.Header().Get("Location"))
	}
}
