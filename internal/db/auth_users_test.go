package db

import (
	"path/filepath"
	"testing"
)

func TestCreateGoogleLoginAndAdminUsers(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "relations.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	user := User{ID: "google-sub-1", Email: "reader@example.com", Name: "Reader"}
	for _, id := range []string{"session-1", "session-2"} {
		if err := store.CreateGoogleLogin(user, Session{ID: id, ExpiresAt: 4102444800}); err != nil {
			t.Fatalf("create Google login: %v", err)
		}
	}
	if _, err := store.db.Exec(`INSERT INTO collections (name, user_id) VALUES ('Murojaah', ?)`, user.ID); err != nil {
		t.Fatalf("insert fixture collection: %v", err)
	}
	if _, err := store.db.Exec(`
		INSERT INTO collection_items (collection_id, item_type, ayah1_surah, ayah1_ayah, note)
		VALUES (1, 'ayah', 2, 25, 'private note')
	`); err != nil {
		t.Fatalf("insert fixture item: %v", err)
	}

	users, summary, err := store.AdminUsers(200)
	if err != nil {
		t.Fatalf("list admin users: %v", err)
	}
	if summary.Users != 1 || summary.ActiveSessions != 2 || summary.Collections != 1 || summary.SavedItems != 1 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if len(users) != 1 || users[0].Email != user.Email || users[0].Collections != 1 || users[0].SavedItems != 1 || users[0].ActiveSessions != 2 {
		t.Fatalf("unexpected user list: %+v", users)
	}
	if users[0].Name != user.Name {
		t.Fatalf("user profile was not updated: %+v", users[0])
	}

	var accessToken, refreshToken string
	if err := store.db.QueryRow(`SELECT access_token, refresh_token FROM sessions WHERE id = 'session-1'`).Scan(&accessToken, &refreshToken); err != nil {
		t.Fatal(err)
	}
	if accessToken != "" || refreshToken != "" {
		t.Fatalf("Google tokens must not be retained: access=%q refresh=%q", accessToken, refreshToken)
	}
}

func TestOpenClearsLegacyOAuthTokens(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`
		INSERT INTO sessions (id, user_id, email, access_token, refresh_token, expires_at)
		VALUES ('legacy-session', 'legacy-user', 'old@example.com', 'old-access-token', 'old-refresh-token', 4102444800)
	`); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	store, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	session, err := store.SessionByID("legacy-session")
	if err != nil {
		t.Fatal(err)
	}
	if session.Provider != "" {
		t.Fatalf("legacy session credentials were not cleared: %+v", session)
	}
	var accessToken, refreshToken string
	if err := store.db.QueryRow(`SELECT access_token, refresh_token FROM sessions WHERE id = 'legacy-session'`).Scan(&accessToken, &refreshToken); err != nil {
		t.Fatal(err)
	}
	if accessToken != "" || refreshToken != "" {
		t.Fatal("legacy OAuth tokens were not cleared")
	}
}
