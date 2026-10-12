package main

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/srmdn/hifzlink/internal/search"
)

func newLearningTestServer(t *testing.T) *server {
	t.Helper()
	s := newTestServer(t)
	root := filepath.Join("..", "..")
	var err error
	s.quran, err = search.Load(filepath.Join(root, "data", "quran.json"))
	if err != nil {
		t.Fatal(err)
	}
	s.trans, err = search.LoadTranslations(filepath.Join(root, "data", "translations"), "en", "id")
	if err != nil {
		t.Fatal(err)
	}
	s.tmpl, err = template.New("").Funcs(assetFuncs(newAssetVersions(filepath.Join(root, "web", "static")))).ParseGlob(filepath.Join(root, "web", "templates", "*.html"))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestLearningPagesRenderCanonicalVerses(t *testing.T) {
	s := newLearningTestServer(t)
	for _, tc := range []struct {
		url         string
		surah, ayah int
		label       string
	}{
		{"/", 78, 4, "A familiar ayah."},
		{"/practice?surah=78", 78, 5, "Reveal 78:5"},
		{"/practice?surah=102&lang=id", 102, 4, "Tampilkan 102:4"},
	} {
		t.Run(tc.url, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.url, nil)
			w := httptest.NewRecorder()
			if req.URL.Path == "/" {
				s.handleHome(w, req)
			} else {
				s.handlePractice(w, req)
			}
			if w.Code != http.StatusOK {
				t.Fatalf("status %d: %s", w.Code, w.Body.String())
			}
			body := w.Body.String()
			ayah, _ := s.quran.Get(tc.surah, tc.ayah)
			if !strings.Contains(body, ayah.TextAR) {
				t.Fatal("full canonical verse missing")
			}
			if !strings.Contains(body, tc.label) {
				t.Fatalf("missing label %q", tc.label)
			}
			if !strings.Contains(body, `<details class="practice-answer" data-practice-answer>`) {
				t.Fatal("answer should be closed by default")
			}
			if strings.Contains(body, "ZgotmplZ") {
				t.Fatal("unsafe or invalid template URL")
			}
		})
	}
}

func TestHomepageIncludesLocalizedOriginStory(t *testing.T) {
	s := newLearningTestServer(t)
	for _, tc := range []struct{ url, label string }{
		{"/?lang=en", "Why I built this"},
		{"/?lang=id", "Kenapa gue membuat ini"},
	} {
		req := httptest.NewRequest(http.MethodGet, tc.url, nil)
		w := httptest.NewRecorder()
		s.handleHome(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("%s: status %d", tc.url, w.Code)
		}
		if !strings.Contains(w.Body.String(), tc.label) || !strings.Contains(w.Body.String(), `class="home-story"`) {
			t.Errorf("%s: localized origin story missing", tc.url)
		}
	}
}

func TestPracticeSetupAndInvalidRequests(t *testing.T) {
	s := newLearningTestServer(t)
	for _, tc := range []struct {
		url, method string
		status      int
	}{
		{"/practice", http.MethodGet, 200}, {"/practice?surah=1", http.MethodGet, 400},
		{"/practice?surah=oops", http.MethodGet, 400}, {"/practice", http.MethodPost, 405},
		{"/not-a-page", http.MethodGet, 404},
	} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(tc.method, tc.url, nil)
		if req.URL.Path == "/not-a-page" {
			s.handleHome(w, req)
		} else {
			s.handlePractice(w, req)
		}
		if w.Code != tc.status {
			t.Errorf("%s %s: got %d, want %d", tc.method, tc.url, w.Code, tc.status)
		}
		if tc.url == "/practice" && tc.method == http.MethodGet && !strings.Contains(w.Body.String(), `name="surah"`) {
			t.Error("practice setup missing")
		}
	}
}

func TestAyahPageUsesLearningLayoutAndLocalizedShareMetadata(t *testing.T) {
	s := newLearningTestServer(t)
	for _, tc := range []struct {
		url, lang, expectedTranslation, expectedShare string
	}{
		{"/ayah/60/8?lang=en", "en", "Allah does not forbid you", "Share ayah"},
		{"/ayah/60/8?lang=id", "id", "Allah tidak melarang kamu", "Bagikan ayat"},
	} {
		t.Run(tc.lang, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.url, nil)
			w := httptest.NewRecorder()
			s.handleAyahPage(w, req)
			if w.Code != http.StatusOK {
				t.Fatalf("status %d: %s", w.Code, w.Body.String())
			}
			body := w.Body.String()
			ayah, _ := s.quran.Get(60, 8)
			for _, want := range []string{
				ayah.TextAR, tc.expectedTranslation, tc.expectedShare,
				`class="learning-page catalog-page ayah-page"`,
				`property="og:url" content="https://example.com/ayah/60/8?lang=` + tc.lang + `"`,
				`data-share-url="/ayah/60/8?lang=` + tc.lang + `"`,
				`/static/ayah.css?v=`,
			} {
				if !strings.Contains(body, want) {
					t.Errorf("rendered page missing %q", want)
				}
			}
			if strings.Contains(body, "ZgotmplZ") {
				t.Fatal("unsafe or invalid template URL")
			}
		})
	}
}

func TestLegalPagesUseSharedLearningNavigationAndFooter(t *testing.T) {
	s := newLearningTestServer(t)
	for _, tc := range []struct {
		path, heading, privacyLabel, browseLabel, content string
	}{
		{"/privacy?lang=en", "Privacy Policy", "Privacy", "Browse Quran", "OAuth access and refresh tokens"},
		{"/terms?lang=id", "Terms of Service", "Privasi", "Baca per surah", "MIT License governs reuse"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			w := httptest.NewRecorder()
			if req.URL.Path == "/privacy" {
				s.handlePrivacy(w, req)
			} else {
				s.handleTerms(w, req)
			}
			if w.Code != http.StatusOK {
				t.Fatalf("status %d: %s", w.Code, w.Body.String())
			}
			body := w.Body.String()
			for _, want := range []string{
				tc.heading, `class="learning-page legal-page"`, `class="learning-footer"`,
				`class="mobile-drawer"`, tc.browseLabel, tc.privacyLabel, `/static/legal.css?v=`,
				"mail@srmdn.com", tc.content,
			} {
				if !strings.Contains(body, want) {
					t.Errorf("rendered page missing %q", want)
				}
			}
			if strings.Contains(body, `class="footer"`) || strings.Contains(body, `class="lang-switch"`) {
				t.Error("legal page retained the legacy footer or navigation")
			}
		})
	}
}
