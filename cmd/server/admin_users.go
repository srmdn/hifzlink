package main

import (
	"net/http"
)

func (s *server) handleAdminIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.requireAdmin(w, r) {
		return
	}
	http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
}

func (s *server) handleAdminUsers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.requireAdmin(w, r) {
		return
	}

	users, summary, err := s.db.AdminUsers(200)
	if err != nil {
		internalError(w, r, err)
		return
	}
	for i := range users {
		users[i].CreatedAt = formatUpdatedAt(users[i].CreatedAt)
		users[i].LastLogin = formatTimestamp(users[i].LastLogin)
	}
	s.render(w, "admin-users.html", s.learningData(r, map[string]any{
		"Title":        "User Overview - HifzLink Admin",
		"NoIndex":      true,
		"AdminUsers":   users,
		"AdminSummary": summary,
		"UsersLimited": summary.Users > len(users),
	}))
}
