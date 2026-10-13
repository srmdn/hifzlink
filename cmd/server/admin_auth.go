package main

import (
	"crypto/subtle"
	"net/http"
	"time"
)

const cookieAdminSession = "_admin_session"
const adminSessionMaxAge = 8 * 60 * 60 // 8 hours

// handleAdminLogin serves the login form (GET) and validates credentials (POST).
func (s *server) handleAdminLogin(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Cache-Control", "no-store")
		if cookie, err := r.Cookie(cookieAdminSession); err == nil && s.validAdminSession(cookie.Value) {
			http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
			return
		}
		s.render(w, "admin-login.html", s.learningData(r, map[string]any{
			"Title":      "Admin Login - HifzLink",
			"NoIndex":    true,
			"LoginError": r.URL.Query().Get("error") == "1",
		}))
	case http.MethodPost:
		w.Header().Set("Cache-Control", "no-store")
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
			if !s.adminGlobalLimiter.allow("global") {
				http.Error(w, "too many requests", http.StatusTooManyRequests)
				return
			}
			http.Redirect(w, r, "/admin/login?error=1", http.StatusSeeOther)
			return
		}
		sessionID, err := s.issueAdminSession()
		if err != nil {
			internalError(w, r, err)
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name:     cookieAdminSession,
			Value:    sessionID,
			Path:     "/",
			MaxAge:   adminSessionMaxAge,
			HttpOnly: true,
			Secure:   secureCookieFor(r),
			SameSite: http.SameSiteStrictMode,
		})
		http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
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
	if cookie, err := r.Cookie(cookieAdminSession); err == nil {
		s.revokeAdminSession(cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     cookieAdminSession,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secureCookieFor(r),
		SameSite: http.SameSiteStrictMode,
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *server) issueAdminSession() (string, error) {
	id, err := generateRandomBytes(32)
	if err != nil {
		return "", err
	}
	now := time.Now()
	s.adminSessionsMu.Lock()
	defer s.adminSessionsMu.Unlock()
	if s.adminSessions == nil {
		s.adminSessions = make(map[string]time.Time)
	}
	for token, expiresAt := range s.adminSessions {
		if !expiresAt.After(now) {
			delete(s.adminSessions, token)
		}
	}
	s.adminSessions[id] = now.Add(time.Duration(adminSessionMaxAge) * time.Second)
	return id, nil
}

func (s *server) validAdminSession(id string) bool {
	if s == nil || id == "" {
		return false
	}
	s.adminSessionsMu.Lock()
	defer s.adminSessionsMu.Unlock()
	expiresAt, ok := s.adminSessions[id]
	if !ok {
		return false
	}
	if !expiresAt.After(time.Now()) {
		delete(s.adminSessions, id)
		return false
	}
	return true
}

func (s *server) revokeAdminSession(id string) {
	if s == nil || id == "" {
		return
	}
	s.adminSessionsMu.Lock()
	delete(s.adminSessions, id)
	s.adminSessionsMu.Unlock()
}
