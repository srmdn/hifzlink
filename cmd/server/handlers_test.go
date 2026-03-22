package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/srmdn/hifzlink/internal/db"
	"github.com/srmdn/hifzlink/internal/relations"
	"github.com/srmdn/hifzlink/internal/search"
)

func newTestServer(t *testing.T) *server {
	t.Helper()

	ayahs := []search.Ayah{
		{Surah: 1, SurahName: "Al-Fatihah", Ayah: 1, Juz: 1, TextAR: "بِسْمِ اللَّهِ"},
		{Surah: 60, SurahName: "Al-Mumtahanah", Ayah: 8, Juz: 28, TextAR: "لَا يَنْهَاكُمُ اللَّهُ"},
		{Surah: 60, SurahName: "Al-Mumtahanah", Ayah: 9, Juz: 28, TextAR: "إِنَّمَا يَنْهَاكُمُ اللَّهُ"},
	}

	qf, err := os.CreateTemp(t.TempDir(), "quran*.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.NewEncoder(qf).Encode(ayahs); err != nil {
		t.Fatal(err)
	}
	qf.Close()

	quranStore, err := search.Load(qf.Name())
	if err != nil {
		t.Fatal(err)
	}

	transDir := t.TempDir()
	os.WriteFile(transDir+"/en.json", []byte(`{"1:1":"In the name of Allah","60:8":"Allah does not forbid you","60:9":"Allah only forbids you"}`), 0644)
	os.WriteFile(transDir+"/id.json", []byte(`{"1:1":"Dengan nama Allah","60:8":"Allah tidak melarang kamu","60:9":"Sesungguhnya Allah hanya melarang"}`), 0644)

	transStore, err := search.LoadTranslations(transDir, "en", "id")
	if err != nil {
		t.Fatal(err)
	}

	dbStore, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { dbStore.Close() })

	return &server{
		quran: quranStore,
		trans: transStore,
		rels:  relations.NewService(dbStore, quranStore),
	}
}

// --- /api/ayah/{surah}/{ayah} ---

func TestHandleAPIAyah_Basic(t *testing.T) {
	s := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/ayah/60/8", nil)
	rr := httptest.NewRecorder()
	s.handleAPIAyah(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}

	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["surah"] != float64(60) || resp["ayah"] != float64(8) {
		t.Errorf("unexpected surah/ayah: %v / %v", resp["surah"], resp["ayah"])
	}
	if resp["text"] == nil || resp["text"] == "" {
		t.Error("expected non-empty text")
	}
	// Default lang=ar — no translation fields
	if _, ok := resp["translation_text"]; ok {
		t.Error("expected no translation_text for lang=ar")
	}
}

func TestHandleAPIAyah_LangEN(t *testing.T) {
	s := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/ayah/60/8?lang=en", nil)
	rr := httptest.NewRecorder()
	s.handleAPIAyah(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}

	var resp map[string]any
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp["translation_lang"] != "en" {
		t.Errorf("expected translation_lang=en, got %v", resp["translation_lang"])
	}
	if resp["translation_text"] == nil || resp["translation_text"] == "" {
		t.Error("expected non-empty translation_text")
	}
}

func TestHandleAPIAyah_LangID(t *testing.T) {
	s := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/ayah/60/8?lang=id", nil)
	rr := httptest.NewRecorder()
	s.handleAPIAyah(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}

	var resp map[string]any
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp["translation_lang"] != "id" {
		t.Errorf("expected translation_lang=id, got %v", resp["translation_lang"])
	}
	if resp["translation_text"] == nil || resp["translation_text"] == "" {
		t.Error("expected non-empty translation_text")
	}
}

func TestHandleAPIAyah_NotFound(t *testing.T) {
	s := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/ayah/999/1", nil)
	rr := httptest.NewRecorder()
	s.handleAPIAyah(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestHandleAPIAyah_InvalidPath(t *testing.T) {
	s := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/ayah/abc/def", nil)
	rr := httptest.NewRecorder()
	s.handleAPIAyah(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestHandleAPIAyah_MethodNotAllowed(t *testing.T) {
	s := newTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/ayah/60/8", nil)
	rr := httptest.NewRecorder()
	s.handleAPIAyah(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

// --- /api/ayah/{surah}/{ayah}/relations ---

func TestHandleAPIAyah_Relations(t *testing.T) {
	s := newTestServer(t)

	if err := s.rels.Add("60:8", "60:9", "test"); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/ayah/60/8/relations", nil)
	rr := httptest.NewRecorder()
	s.handleAPIAyah(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}

	var resp map[string]any
	json.NewDecoder(rr.Body).Decode(&resp)

	related, ok := resp["related"].([]any)
	if !ok {
		t.Fatal("expected related array")
	}
	if len(related) != 1 {
		t.Errorf("expected 1 related, got %d", len(related))
	}
}

func TestHandleAPIAyah_Relations_WithLang(t *testing.T) {
	s := newTestServer(t)

	if err := s.rels.Add("60:8", "60:9", ""); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/ayah/60/8/relations?lang=en", nil)
	rr := httptest.NewRecorder()
	s.handleAPIAyah(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}

	var resp map[string]any
	json.NewDecoder(rr.Body).Decode(&resp)

	related, _ := resp["related"].([]any)
	if len(related) != 1 {
		t.Fatalf("expected 1 related, got %d", len(related))
	}
	entry := related[0].(map[string]any)
	if entry["translation_text"] == nil || entry["translation_text"] == "" {
		t.Error("expected translation_text in related entry for lang=en")
	}
}

// --- /api/relations ---

func TestHandleAPIRelations_Post(t *testing.T) {
	s := newTestServer(t)

	body := `{"ayah1":"60:8","ayah2":"60:9","note":"mutashabihat"}`
	req := httptest.NewRequest(http.MethodPost, "/api/relations", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.handleAPIRelations(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleAPIRelations_InvalidJSON(t *testing.T) {
	s := newTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/relations", strings.NewReader("not json"))
	rr := httptest.NewRecorder()
	s.handleAPIRelations(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestHandleAPIRelations_UnknownAyah(t *testing.T) {
	s := newTestServer(t)

	body := `{"ayah1":"999:999","ayah2":"60:9","note":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/relations", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.handleAPIRelations(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for unknown ayah, got %d", rr.Code)
	}
}

func TestHandleAPIRelations_MethodNotAllowed(t *testing.T) {
	s := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/relations", nil)
	rr := httptest.NewRecorder()
	s.handleAPIRelations(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

// --- /api/surah/{surah}/relations ---

func TestHandleAPISurah(t *testing.T) {
	s := newTestServer(t)

	if err := s.rels.Add("60:8", "60:9", ""); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/surah/60/relations", nil)
	rr := httptest.NewRecorder()
	s.handleAPISurah(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}

	var resp map[string]any
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp["surah"] != float64(60) {
		t.Errorf("expected surah 60, got %v", resp["surah"])
	}
}

func TestHandleAPISurah_InvalidPath(t *testing.T) {
	s := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/surah/abc/relations", nil)
	rr := httptest.NewRecorder()
	s.handleAPISurah(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

// --- /api/juz/{juz}/relations ---

func TestHandleAPIJuz(t *testing.T) {
	s := newTestServer(t)

	if err := s.rels.Add("60:8", "60:9", ""); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/juz/28/relations", nil)
	rr := httptest.NewRecorder()
	s.handleAPIJuz(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}

	var resp map[string]any
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp["juz"] != float64(28) {
		t.Errorf("expected juz 28, got %v", resp["juz"])
	}
}

func TestHandleAPIJuz_InvalidPath(t *testing.T) {
	s := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/juz/abc/relations", nil)
	rr := httptest.NewRecorder()
	s.handleAPIJuz(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

// --- /admin/relations ---

func TestHandleAdminRelations_PostAddRedirect(t *testing.T) {
	s := newTestServer(t)

	form := url.Values{}
	form.Set("action", "add")
	form.Set("ayah1", "60:8")
	form.Set("ayah2", "60:9")
	form.Set("note", "test relation")
	form.Set("lang", "id")

	req := httptest.NewRequest(http.MethodPost, "/admin/relations", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	s.handleAdminRelations(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", rr.Code)
	}
	location := rr.Header().Get("Location")
	if !strings.Contains(location, "/admin/relations?status=added&lang=id") {
		t.Fatalf("unexpected redirect location: %s", location)
	}
}

func TestHandleAdminRelations_PostDeleteRedirect(t *testing.T) {
	s := newTestServer(t)

	if err := s.rels.Add("60:8", "60:9", "to delete"); err != nil {
		t.Fatal(err)
	}
	rows, err := s.rels.AllRelations()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 relation, got %d", len(rows))
	}

	form := url.Values{}
	form.Set("action", "delete")
	form.Set("id", strconv.FormatInt(rows[0].ID, 10))
	form.Set("lang", "en")

	req := httptest.NewRequest(http.MethodPost, "/admin/relations", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	s.handleAdminRelations(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", rr.Code)
	}
	location := rr.Header().Get("Location")
	if !strings.Contains(location, "/admin/relations?status=deleted&lang=en") {
		t.Fatalf("unexpected redirect location: %s", location)
	}
}

func TestHandleAdminRelations_PostEditRedirect(t *testing.T) {
	s := newTestServer(t)

	if err := s.rels.Add("60:8", "60:9", "before"); err != nil {
		t.Fatal(err)
	}
	rows, err := s.rels.AllRelations()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 relation, got %d", len(rows))
	}

	form := url.Values{}
	form.Set("action", "edit")
	form.Set("id", strconv.FormatInt(rows[0].ID, 10))
	form.Set("ayah1", "60:9")
	form.Set("ayah2", "60:8")
	form.Set("note", "after")
	form.Set("category", "lafzi")
	form.Set("lang", "id")

	req := httptest.NewRequest(http.MethodPost, "/admin/relations", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	s.handleAdminRelations(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", rr.Code)
	}
	location := rr.Header().Get("Location")
	if !strings.Contains(location, "/admin/relations?status=edited&lang=id") {
		t.Fatalf("unexpected redirect location: %s", location)
	}
}

// --- helpers ---

func TestSanitizeLang(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"ar", "ar"},
		{"en", "en"},
		{"id", "id"},
		{"EN", "en"},
		{"  ar  ", "ar"},
		{"fr", "ar"},
		{"", "ar"},
		{"xyz", "ar"},
	}
	for _, tc := range tests {
		if got := sanitizeLang(tc.input); got != tc.want {
			t.Errorf("sanitizeLang(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
