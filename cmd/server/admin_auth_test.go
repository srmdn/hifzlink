package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestAdminLoginCreatesServerSideSession(t *testing.T) {
	s := newTestServer(t)
	form := url.Values{"user": {s.adminUser}, "pass": {s.adminPass}}
	request := httptest.NewRequest(http.MethodPost, "/admin/login", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	s.handleAdminLogin(response, request)

	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/admin/users" {
		t.Fatalf("expected redirect to admin users, got %d to %q", response.Code, response.Header().Get("Location"))
	}
	var sessionCookie *http.Cookie
	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == cookieAdminSession {
			sessionCookie = cookie
		}
	}
	if sessionCookie == nil {
		t.Fatal("successful admin login did not set a session cookie")
	}
	if sessionCookie.Value == s.adminToken || sessionCookie.MaxAge != adminSessionMaxAge || !sessionCookie.HttpOnly || sessionCookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("unexpected admin session cookie attributes: %+v", sessionCookie)
	}
	if !s.validAdminSession(sessionCookie.Value) {
		t.Fatal("new admin session is not valid on the server")
	}
}

func TestAdminLoginHasGlobalLimitAcrossClientAddresses(t *testing.T) {
	s := newTestServer(t)
	s.adminLimiter = newAdminRateLimiter(10, time.Hour)
	s.adminGlobalLimiter = newAdminRateLimiter(2, time.Hour)

	for i := 0; i < 3; i++ {
		form := url.Values{"user": {"wrong"}, "pass": {"wrong"}}
		request := httptest.NewRequest(http.MethodPost, "/admin/login", strings.NewReader(form.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		request.Header.Set("X-Real-IP", "198.51.100."+strconv.Itoa(i+1))
		response := httptest.NewRecorder()
		s.handleAdminLogin(response, request)
		if i < 2 && response.Code != http.StatusSeeOther {
			t.Fatalf("attempt %d should reach credential check, got %d", i+1, response.Code)
		}
		if i == 2 && response.Code != http.StatusTooManyRequests {
			t.Fatalf("global limiter should reject the third attempt, got %d", response.Code)
		}
	}

	form := url.Values{"user": {s.adminUser}, "pass": {s.adminPass}}
	request := httptest.NewRequest(http.MethodPost, "/admin/login", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("X-Real-IP", "198.51.100.99")
	response := httptest.NewRecorder()
	s.handleAdminLogin(response, request)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/admin/users" {
		t.Fatalf("global failed-login limit should not block valid credentials, got %d", response.Code)
	}
}

func TestAdminSessionExpiresOnServer(t *testing.T) {
	s := newTestServer(t)
	s.adminSessions["expired-token"] = time.Now().Add(-time.Second)
	request := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	addAdminCookie(request, "expired-token")
	response := httptest.NewRecorder()
	s.handleAdminUsers(response, request)

	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/admin/login" {
		t.Fatalf("expected expired session redirect to login, got %d to %q", response.Code, response.Header().Get("Location"))
	}
	if _, ok := s.adminSessions["expired-token"]; ok {
		t.Fatal("expired server-side session was not removed")
	}
}

func TestAdminLogoutRevokesServerSideSession(t *testing.T) {
	s := newTestServer(t)
	sessionID, err := s.issueAdminSession()
	if err != nil {
		t.Fatal(err)
	}
	form := url.Values{"csrf_token": {s.csrfTokenFor(s.adminToken)}}
	request := httptest.NewRequest(http.MethodPost, "/admin/logout", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addAdminCookie(request, sessionID)
	response := httptest.NewRecorder()
	s.handleAdminLogout(response, request)

	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/" {
		t.Fatalf("expected logout redirect, got %d to %q", response.Code, response.Header().Get("Location"))
	}
	if s.validAdminSession(sessionID) {
		t.Fatal("admin session remains valid after logout")
	}
}
