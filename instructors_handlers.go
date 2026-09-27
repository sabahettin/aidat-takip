package main

import (
	"errors"
	"net/http"
	"strconv"
)

// ---- Instructors (Eğitmen) ----

func (s *Server) handleInstructorsList(w http.ResponseWriter, r *http.Request) {
	instructors, err := listInstructors(s.db)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	type instructorRow struct {
		Instructor
		GroupCount int
	}
	rows := make([]instructorRow, 0, len(instructors))
	for _, i := range instructors {
		groups, err := listGroupsForInstructor(s.db, i.ID)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		rows = append(rows, instructorRow{Instructor: i, GroupCount: len(groups)})
	}
	s.render(w, "instructors_list", map[string]any{"Instructors": rows})
}

func (s *Server) handleInstructorNewForm(w http.ResponseWriter, r *http.Request) {
	s.render(w, "instructor_form", map[string]any{
		"IsNew":      true,
		"Instructor": Instructor{},
		"FormURL":    "/egitmenler",
	})
}

func parseInstructorForm(r *http.Request) (Instructor, error) {
	if err := r.ParseForm(); err != nil {
		return Instructor{}, err
	}
	name := r.FormValue("full_name")
	if name == "" {
		return Instructor{}, errors.New("ad soyad zorunludur")
	}
	return Instructor{
		FullName: name,
		Phone:    r.FormValue("phone"),
		Email:    r.FormValue("email"),
		Note:     r.FormValue("note"),
	}, nil
}

func (s *Server) handleInstructorCreate(w http.ResponseWriter, r *http.Request) {
	i, err := parseInstructorForm(r)
	if err != nil {
		s.render(w, "instructor_form", map[string]any{
			"IsNew": true, "Instructor": i, "FormURL": "/egitmenler", "Error": err.Error(),
		})
		return
	}
	id, err := createInstructor(s.db, i)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	redirectWithToast(w, r, "/egitmenler/"+strconv.FormatInt(id, 10)+"/duzenle", "Eğitmen eklendi.", "success")
}

func (s *Server) handleInstructorEditForm(w http.ResponseWriter, r *http.Request) {
	id, err := idFromPath(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	i, err := getInstructor(s.db, id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	groups, err := listGroupsForInstructor(s.db, id)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	s.render(w, "instructor_form", map[string]any{
		"IsNew":      false,
		"Instructor": i,
		"FormURL":    "/egitmenler/" + strconv.FormatInt(id, 10) + "/duzenle",
		"Groups":     groups,
	})
}

func (s *Server) handleInstructorUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := idFromPath(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	i, err := parseInstructorForm(r)
	if err != nil {
		i.ID = id
		groups, _ := listGroupsForInstructor(s.db, id)
		s.render(w, "instructor_form", map[string]any{
			"IsNew": false, "Instructor": i, "FormURL": "/egitmenler/" + strconv.FormatInt(id, 10) + "/duzenle",
			"Error": err.Error(), "Groups": groups,
		})
		return
	}
	i.ID = id
	if err := updateInstructor(s.db, i); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	redirectWithToast(w, r, "/egitmenler/"+strconv.FormatInt(id, 10)+"/duzenle", "Eğitmen güncellendi.", "success")
}

func (s *Server) handleInstructorDelete(w http.ResponseWriter, r *http.Request) {
	id, err := idFromPath(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := deleteInstructor(s.db, id); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	redirectWithToast(w, r, "/egitmenler", "Eğitmen silindi.", "success")
}
