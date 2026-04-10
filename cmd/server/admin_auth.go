package main

import (
	"crypto/subtle"
	"net/http"
)

const cookieAdminSession = "_admin_session"
const adminSessionMaxAge = 8 * 60 * 60 // 8 hours

// handleAdminLogin serves the login form (GET) and validates credentials (POST).
func (s *server) handleAdminLogin(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.render(w, "admin-login.html", s.withCommonViewData(r, map[string]any{
			"Title":      "Admin Login — HifzLink",
			"LoginError": r.URL.Query().Get("error") == "1",
		}))
	case http.MethodPost:
		if !s.adminLimiter.allow(realIP(r)) {
			http.Error(w, "too many requests", http.StatusTooManyRequests)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1024)
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		user := r.FormValue("user")
		pass := r.FormValue("pass")
		uMatch := subtle.ConstantTimeCompare([]byte(user), []byte(s.adminUser))
		pMatch := subtle.ConstantTimeCompare([]byte(pass), []byte(s.adminPass))
		if uMatch != 1 || pMatch != 1 {
			http.Redirect(w, r, "/admin/login?error=1", http.StatusSeeOther)
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name:     cookieAdminSession,
			Value:    s.adminToken,
			Path:     "/",
			MaxAge:   adminSessionMaxAge,
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteStrictMode,
		})
		http.Redirect(w, r, "/admin/relations", http.StatusSeeOther)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleAdminLogout clears the admin session cookie.
func (s *server) handleAdminLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1024)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if !s.validateCSRFToken(r, s.csrfTokenFor(s.adminToken)) {
		http.Error(w, "invalid CSRF token", http.StatusForbidden)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     cookieAdminSession,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
