Warning: truncated output (original token count: 17774)
Total output lines: 2317

package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	neturl "net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/srmdn/hifzlink/internal/db"
	"github.com/srmdn/hifzlink/internal/qfclient"
	"github.com/srmdn/hifzlink/internal/relations"
	"github.com/srmdn/hifzlink/internal/search"
	webassets "github.com/srmdn/hifzlink/web"
)

type server struct {
	quran   *search.Store
	trans   *search.TranslationStore
	tafsir  *search.TranslationStore
	db      *db.Store
	rels    *relations.Service
	tmpl    *template.Template
	baseDir string

	adminUser          string
	adminPass          string
	adminToken         string // random key used to derive admin CSRF tokens
	adminSessionsMu    sync.Mutex
	adminSessions      map[string]time.Time // server-side admin session expiry by token
	adminLimiter       *adminRateLimiter
	adminGlobalLimiter *adminRateLimiter
	csrfSecret         []byte // HMAC key for CSRF token derivation

	umamiID string

	// Quran Foundation Content API credentials (audio only).
	qfClientID     string
	qfClientSecret string
	qfAuthEndpoint string
	qfAPIBase      string

	googleClientID     string
	googleClientSecret string
	googleRedirectURI  string

	// Quran Foundation Content API client (nil when QF is not configured)
	qf *qfclient.Client
}

// adminRateLimiter is a simple sliding-window rate limiter for admin endpoints.
type adminRateLimiter struct {
	mu          sync.Mutex
	entries     map[string][]time.Time
	maxAttempts int
	window      time.Duration
}

func newAdminRateLimiter(maxAttempts int, window time.Duration) *adminRateLimiter {
	return &adminRateLimiter{entries: make(map[string][]time.Time), maxAttempts: maxAttempts, window: window}
}

func (l *adminRateLimiter) allow(ip string) bool {
	now := time.Now()
	cutoff := now.Add(-l.window)
	l.mu.Lock()
	defer l.mu.Unlock()
	times := l.entries[ip]
	i := 0
	for _, t := range times {
		if t.After(cutoff) {
			times[i] = t
			i++
		}
	}
	times = times[:i]
	if len(times) >= l.maxAttempts {
		l.entries[ip] = times
		return false
	}
	l.entries[ip] = append(times, now)
	return true
}

// prune removes IPs that have no request history within the rate-limit window,
// preventing the entries map from growing without bound.
func (l *adminRateLimiter) prune() {
	cutoff := time.Now().Add(-l.window)
	l.mu.Lock()
	defer l.mu.Unlock()
	for ip, times := range l.entries {
		i := 0
		for _, t := range times {
			if t.After(cutoff) {
				times[i] = t
				i++
			}
		}
		if i == 0 {
			delete(l.entries, ip)
		} else {
			l.entries[ip] = times[:i]
		}
	}
}

// internalError logs the error server-side and returns a generic 500 to the client.
func internalError(w http.ResponseWriter, r *http.Request, err error) {
	log.Printf("internal error %s %s: %v", r.Method, r.URL.Path, err)
	http.Error(w, "internal server error", http.StatusInternalServerError)
}

// isSafeRedirect returns true if target is a safe relative URL (no open redirect).
func isSafeRedirect(target string) bool {
	return strings.HasPrefix(target, "/") && !strings.HasPrefix(target, "//")
}

// realIP extracts the real client IP, preferring X-Real-IP set by nginx.
func realIP(r *http.Request) string {
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	addr := r.RemoteAddr
	if i := strings.LastIndex(addr, ":"); i >= 0 {
		return addr[:i]
	}
	return addr
}

type adminCategoryOption struct {
	Key   string
	Label string
	Hint  string
}

// userView holds the safe, template-visible subset of a session.
type userView struct {
	Name  string
	Email string
}

// maxBodyBytes is the maximum POST body size accepted by mutation handlers.
const maxBodyBytes = 64 * 1024 // 64 KB

func main() {
	baseDir, err := resolveBaseDir()
	if err != nil {
		log.Fatalf("failed to locate project root: %v", err)
	}

	loadDotEnv(filepath.Join(baseDir, ".env"))

	quran, err := search.Load(filepath.Join(baseDir, "data", "quran.json"))
	if err != nil {
		log.Fatalf("failed to load quran dataset: %v", err)
	}

	translations, err := search.LoadTranslations(filepath.Join(baseDir, "data", "translations"), "en", "id")
	if err != nil {
		log.Fatalf("failed to load translations: %v", err)
	}

	tafsirDir := filepath.Join(baseDir, "data", "tafsir")
	tafsirStore, err := search.LoadTranslationFiles(map[string]string{
		"en": filepath.Join(tafsirDir, "en.ibn-kathir.json"),
		"id": filepath.Join(tafsirDir, "id.kemenag.json"),
	})
	if err != nil {
		log.Fatalf("failed to load tafsir: %v", err)
	}

	dbStore, err := db.Open(filepath.Join(baseDir, "data", "relations.db"))
	if err != nil {
		log.Fatalf("failed to open db: %v", err)
	}
	defer dbStore.Close()

	staticFiles, err := embeddedStaticFiles()
	if err != nil {
		log.Fatalf("failed to locate embedded static files: %v", err)
	}
	assets, err := newAssetVersions(staticFiles)
	if err != nil {
		log.Fatalf("failed to fingerprint embedded static files: %v", err)
	}

	tmpl, err := template.New("").Funcs(assetFuncs(assets)).ParseFS(webassets.Files, "templates/*.html")
	if err != nil {
		log.Fatalf("failed to parse templates: %v", err)
	}

	s := &server{
		quran:              quran,
		trans:              translations,
		tafsir:             tafsirStore,
		db:                 dbStore,
		rels:               relations.NewService(dbStore, quran),
		tmpl:               tmpl,
		baseDir:            baseDir,
		adminUser:          strings.TrimSpace(os.Getenv("HIFZLINK_ADMIN_USER")),
		adminPass:          strings.TrimSpace(os.Getenv("HIFZLINK_ADMIN_PASS")),
		adminLimiter:       newAdminRateLimiter(5, 15*time.Minute),
		adminGlobalLimiter: newAdminRateLimiter(30, 15*time.Minute),
		umamiID:            strings.TrimSpace(os.Getenv("HIFZLINK_UMAMI_ID")),

		qfClientID:     strings.TrimSpace(os.Getenv("QF_CLIENT_ID")),
		qfClientSecret: strings.TrimSpace(os.Getenv("QF_CLIENT_SECRET")),
		qfAuthEndpoint: strings.TrimSpace(os.Getenv("QF_AUTH_ENDPOINT")),
		qfAPIBase:      strings.TrimSpace(os.Getenv("QF_API_BASE")),

		googleClientID:     strings.TrimSpace(os.Getenv("GOOGLE_CLIENT_ID")),
		googleClientSecret: strings.TrimSpace(os.Getenv("GOOGLE_CLIENT_SECRET")),
		googleRedirectURI:  strings.TrimSpace(os.Getenv("GOOGLE_REDIRECT_URI")),
	}
	s.qf = qfclient.New(s.qfClientID, s.qfClientSecret, s.qfAuthEndpoint, s.qfAPIBase)
	if s.qf != nil {
		log.Printf("qfclient: content API enabled (%s)", s.qfAPIBase)
	}

	tok, err := generateRandomBytes(24)
	if err != nil {
		log.Fatalf("failed to generate admin token: %v", err)
	}
	s.adminToken = tok

	csrfStr, err := generateRandomBytes(32)
	if err != nil {
		log.Fatalf("failed to generate CSRF secret: %v", err)
	}
	s.csrfSecret = []byte(csrfStr)

	// Periodically prune the admin rate-limiter map to prevent unbounded growth.
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			s.adminLimiter.prune()
			s.adminGlobalLimiter.prune()
		}
	}()

	// Clean up expired sessions on startup.
	if err := dbStore.DeleteExpiredSessions(); err != nil {
		log.Printf("warn: delete expired sessions: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/static/", http.StripPrefix("/static/", staticHandler(staticFiles)))

	mux.HandleFunc("/", s.handleHome)
	mux.HandleFunc("/practice", s.handlePractice)
	mux.HandleFunc("/privacy", s.handlePrivacy)
	mux.HandleFunc("/terms", s.handleTerms)
	mux.HandleFunc("/methodology", s.handleMethodology)
	mux.HandleFunc("/auth/login", s.handleAuthLogin)
	mux.HandleFunc("/auth/callback", s.handleAuthCallback)
	mux.HandleFunc("/auth/logout", s.handleAuthLogout)
	mux.HandleFunc("/search", s.handleSearch)
	mux.HandleFunc("/ayah/", s.handleAyahPage)
	mux.HandleFunc("/compare", s.handleComparePage)
	mux.HandleFunc("/surah", s.handleSurahIndexPage)
	mux.HandleFunc("/surah/", s.handleSurahPage)
	mux.HandleFunc("/juz", s.handleJuzIndexPage)
	mux.HandleFunc("/juz/", s.handleJuzPage)
	mux.HandleFunc("/dashboard", s.handleDashboardPage)
	mux.HandleFunc("/collections", s.handleCollectionsPage)
	mux.HandleFunc("/collections/", s.handleCollectionDetailPage)
	mux.HandleFunc("/collections/items", s.handleCollectionItemsPost)
	mux.HandleFunc("/collections/items/delete", s.handleCollectionItemsDelete)
	mux.HandleFunc("/collections/items/mastered/", s.handleToggleMastered)
	mux.HandleFunc("/admin/login", s.handleAdminLogin)
	mux.HandleFunc("/admin", s.handleAdminIndex)
	mux.HandleFunc("/admin/users", s.handleAdminUsers)
	mux.HandleFunc("/admin/logout", s.handleAdminLogout)
	mux.HandleFunc("/admin/relations", s.handleAdminRelations)
	mux.HandleFunc("/admin/relations/", s.handleAdminRelationEdit)

	mux.HandleFunc("/robots.txt", s.handleRobotsTxt)
	mux.HandleFunc("/sitemap.xml", s.handleSitemap)
	mux.HandleFunc("/llms.txt", s.handleLLMsTxt)

	mux.HandleFunc("/api/ayah/", s.handleAPIAyah)
	mux.HandleFunc("/api/relations", s.handleAPIRelations)
	mux.HandleFunc("/api/surah/", s.handleAPISurah)
	mux.HandleFunc("/api/juz/", s.handleAPIJuz)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := "127.0.0.1:" + port
	srv := &http.Server{
		Addr:           addr,
		Handler:        logRequests(mux),
		ReadTimeout:    15 * time.Second,
		WriteTimeout:   30 * time.Second,
		IdleTimeout:    60 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1 MB
	}
	log.Printf("server listening on %s", addr)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

func (s *server) handlePrivacy(w http.ResponseWriter, r *http.Request) {
	s.render(w, "privacy.html", s.learningData(r, map[string]any{
		"Title":       "Privacy Policy — hifzlink",
		"Description": "Privacy policy for hifzlink, the open-source Quran mutashabihat review tool.",
	}))
}

func (s *server) handleTerms(w http.ResponseWriter, r *http.Request) {
	s.render(w, "terms.html", s.learningData(r, map[string]any{
		"Title":       "Terms of Service — hifzlink",
		"Description": "Terms of service for hifzlink, the open-source Quran mutashabihat review tool.",
	}))
}

func (s *server) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
		raw := strings.TrimSpace(r.FormValue("ayah"))
		surah, ayah, err := relations.ParseAyahRef(raw)
		if err != nil {
			lang := sanitizeLang(r.FormValue("lang"))
			http.Redirect(w, r, fmt.Sprintf("/?error=invalid_ref&q=%s&lang=%s", neturl.QueryEscape(raw), lang), http.StatusSeeOther)
			return
		}
		lang := sanitizeLang(r.FormValue("lang"))
		http.Redirect(w, r, withLang(fmt.Sprintf("/ayah/%d/%d", surah, ayah), lang), http.StatusSeeOther)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	q := strings.TrimSpace(r.URL.Query().Get("q"))
	lang := pageLang(r)
	categoryFilter := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("category")))

	type pairResult struct {
		Ayah1         string
		Ayah1Name     string
		Ayah1URL      string
		Ayah2         string
		Ayah2Name     string
		Ayah2URL      string
		Category      string
		CategoryLabel string
		Note          string
		CompareURL    string
	}

	categoryOptions := adminCategoryOptions()
	categoryLabelMap := make(map[string]string, len(categoryOptions))
	for _, opt := range categoryOptions {
		categoryLabelMap[opt.Key] = opt.Label
	}

	type filterPill struct {
		Label  string
		URL    string
		Active bool
	}

	var results []pairResult
	var filterPills []filterPill
	var queryLabel string
	var errMsg string
	var anchorConnectionsURL string
	var anchorConnectionsRef string

	if q != "" {
		var dbRels []db.Relation
		var err error

		if surahNum, ayahNum, parseErr := relations.ParseAyahRef(q); parseErr == nil {
			anchorConnectionsRef = relations.FormatAyahRef(surahNum, ayahNum)
			anchorConnectionsURL = withLang(fmt.Sprintf("/ayah/%d/%d", surahNum, ayahNum), lang) + "#ayah-related"
			if lang == "id" {
				queryLabel = fmt.Sprintf("pasangan yang memuat ayat %s", relations.FormatAyahRef(surahNum, ayahNum))
			} else {
				queryLabel = fmt.Sprintf("pairs involving %s", relations.FormatAyahRef(surahNum, ayahNum))
			}
			dbRels, err = s.db.ByAyah(surahNum, ayahNum)
		} else if n, atoiErr := strconv.Atoi(q); atoiErr == nil && n >= 1 && n <= 114 {
			if lang == "id" {
				queryLabel = fmt.Sprintf("pasangan di Surah %d: %s", n, s.quran.SurahName(n))
			} else {
				queryLabel = fmt.Sprintf("pairs in Surah %d: %s", n, s.quran.SurahName(n))
			}
			dbRels, err = s.db.BySurah(n)
		} else if n := search.SurahByName(q); n > 0 {
			if lang == "id" {
				queryLabel = fmt.Sprintf("pasangan di Surah %d: %s", n, s.quran.SurahName(n))
			} else {
				queryLabel = fmt.Sprintf("pairs in Surah %d: %s", n, s.quran.SurahName(n))
			}
			dbRels, err = s.db.BySurah(n)
		} else {
			if lang == "id" {
				errMsg = fmt.Sprintf("Pencarian %q belum dikenali. Coba referensi ayat (60:8), nomor surah (60), atau nama surah (Al-Mumtahanah).", q)
			} else {
				errMsg = fmt.Sprintf("Could not interpret %q — try an ayah ref (60:8), surah number (60), or surah name (Al-Mumtahanah).", q)
			}
		}

		if err != nil {
			internalError(w, r, err)
			return
		}

		// Build all results without category filter first.
		var allResults []pairResult
		for _, rel := range dbRels {
			ref1 := relations.FormatAyahRef(rel.Ayah1Surah, rel.Ayah1Ayah)
			ref2 := relations.FormatAyahRef(rel.Ayah2Surah, rel.Ayah2Ayah)
			allResults = append(allResults, pairResult{
				Ayah1:         ref1,
				Ayah1Name:     s.quran.SurahName(rel.Ayah1Surah),
				Ayah1URL:      withLang(fmt.Sprintf("/ayah/%d/%d", rel.Ayah1Surah, rel.Ayah1Ayah), lang),
				Ayah2:         ref2,
				Ayah2Name:     s.quran.SurahName(rel.Ayah2Surah),
				Ayah2URL:      withLang(fmt.Sprintf("/ayah/%d/%d", rel.Ayah2Surah, rel.Ayah2Ayah), lang),
				Category:      rel.Category,
				CategoryLabel: categoryLabelMap[rel.Category],
				Note:          rel.Note,
				CompareURL:    withLang(fmt.Sprintf("/compare?ayah1=%s&ayah2=%s", ref1, ref2), lang),
			})
		}

		// Build filter pills from distinct categories present in results.
		seen := map[string]bool{}
		var presentCategories []string
		for _, r := range allResults {
			if r.Category != "" && !seen[r.Category] {
				seen[r.Category] = true
				presentCategories = append(presentCategories, r.Category)
			}
		}
		if len(presentCategories) >= 1 {
			baseURL := fmt.Sprintf("/search?q=%s&lang=%s", neturl.QueryEscape(q), lang)
			filterPills = append(filterPills, filterPill{Label: "All", URL: baseURL, Active: categoryFilter == ""})
			for _, opt := range categoryOptions {
				if seen[opt.Key] {
					filterPills = append(filterPills, filterPill{
						Label:  opt.Label,
						URL:    baseURL + "&category=" + opt.Key,
						Active: categoryFilter == opt.Key,
					})
				}
			}
		}

		// Apply category filter.
		if categoryFilter != "" {
			for _, r := range allResults {
				if strings.ToLower(r.Category) == categoryFilter {
					results = append(results, r)
				}
			}
		} else {
			results = allResults
		}
	}

	s.render(w, "search.html", s.learningData(r, map[string]any{
		"Title":                "Search pairs",
		"Description":          "Search for Quran verses and find their mutashabihat — similar verses that are easy to confuse. Try it yourself at hifz.click.",
		"Query":                q,
		"QueryLabel":           queryLabel,
		"Results":              results,
		"ErrMsg":               errMsg,
		"FilterPills":          filterPills,
		"CategoryFilter":       categoryFilter,
		"CategoryLabelMap":     categoryLabelMap,
		"AnchorConnectionsURL": anchorConnectionsURL,
		"AnchorConnectionsRef": anchorConnectionsRef,
	}))
}

func (s *server) handleAyahPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	surah, ayah, err := parsePathInts(r.URL.Path, "/ayah/")
	if err != nil {
		s.renderNotFound(w, r, "Ayah not found", "That ayah reference is not valid. Use the format surah:ayah (e.g. 60:8).")
		return
	}

	a, ok := s.quran.Get(surah, ayah)
	if !ok {
		s.renderNotFound(w, r, fmt.Sprintf("Ayah %d:%d not found", surah, ayah), "This ayah does not exist in the dataset. Double-check the surah and ayah numbers.")
		return
	}

	related, err := s.rels.RelatedAyahs(surah, ayah)
	if err != nil {
		internalError(w, r, err)
		return
	}
	lang := pageLang(r)
	related = s.withTranslations(lang, related)
	ayahTranslation := s.translationFor(lang, surah, ayah)

	var collections []db.Collection
	if sess, ok := s.currentSession(r); ok {
		if cols, err := s.db.Collections(sess.UserID); err == nil {
			collections = cols
		}
	}

	tafsirContent := s.tafsirFor(lang, surah, ayah)

	// Fetch verse data from QF Content API (audio, Uthmani text).
	var qfAudioURL string
	if s.qf != nil {
		verseKey := fmt.Sprintf("%d:%d", surah, ayah)
		if vd, err := s.qf.FetchVerse(verseKey, 7); err != nil {
			log.Printf("qfclient: verse %s: %v", verseKey, err)
		} else {
			qfAudioURL = vd.AudioURL
		}
	}

	langCopy := ayahCopy(lang)
	ref := relations.FormatAyahRef(surah, ayah)
	sharePath := fmt.Sprintf("/ayah/%d/%d", surah, ayah)
	canonicalPath := withLang(sharePath, lang)
	pageTitle := fmt.Sprintf("Ayah %s · %s | hifzlink", ref, a.SurahName)
	description := fmt.Sprintf("Read Ayah %s from Surah %s, with its translation, tafsir, and related verses.", ref, a.SurahName)
	if lang == "id" {
		pageTitle = fmt.Sprintf("Ayat %s · %s | hifzlink", ref, a.SurahName)
		description = fmt.Sprintf("Baca Ayat %s dari Surah %s, beserta terjemahan, tafsir, dan ayat yang mirip.", ref, a.SurahName)
	}
	viewData := s.learningData(r, map[string]any{
		"Title":           pageTitle,
		"Description":     description,
		"AyahUI":          langCopy,
		"AyahRef":         relations.FormatAyahRef(surah, ayah),
		"Ayah":            a,
		"AyahTranslation": ayahTranslation,
		"AyahTafsir":      tafsirContent,
		"TafsirSource":    tafsirSourceLabel(lang),
		"Related":         related,
		"SurahName":       a.SurahName,
		"Collections":     collections,
		"SaveStatus":      ayahSaveStatus(r.URL.Query().Get("saved"), lang),
		"QFAudioURL":      qfAudioURL,
		"JSONLD": breadcrumbJSONLD("https://"+r.Host, []breadcrumbItem{
			{Name: "Home", Path: withLang("/", lang)},
			{Name: a.SurahName, Path: fmt.Sprintf("/surah/%d", surah)},
			{Name: ref, Path: canonicalPath},
		}),
	})
	viewData["SurahURL"] = withLang(fmt.Sprintf("/surah/%d", surah), lang)
	viewData["JuzURL"] = withLang(fmt.Sprintf("/juz/%d", a.Juz), lang)
	viewData["ShareURL"] = canonicalPath
	viewData["CanonicalURL"] = "https://" + r.Host + canonicalPath
	viewData["OGImageAlt"] = "HifzLink social card about reviewing similar Quran verses"
	s.render(w, "ayah.html", viewData)
}

func (s *server) handleComparePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	a1 := r.URL.Query().Get("ayah1")
	if targets := r.URL.Query()["ayah2"]; len(targets) > 1 {
		s.handleMultiComparePage(w, r, a1, targets)
		return
	}
	a2 := r.URL.Query().Get("ayah2")
	s1, y1, err := relations.ParseAyahRef(a1)
	if err != nil {
		http.Error(w, "invalid ayah1", http.StatusBadRequest)
		return
	}
	s2, y2, err := relations.ParseAyahRef(a2)
	if err != nil {
		http.Error(w, "invalid ayah2", http.StatusBadRequest)
		return
	}

	ayah1, ok1 := s.quran.Get(s1, y1)
	ayah2, ok2 := s.quran.Get(s2, y2)
	if !ok1 || !ok2 {
		s.renderNotFound(w, r, "Ayah not found", "One or both ayah references in this comparison do not exist in the dataset.")
		return
	}

	var collections []db.Collection
	var savedItemID int64
	var savedItemMastered bool
	if sess, ok := s.currentSession(r); ok {
		if cols, err := s.db.Collections(sess.UserID); err == nil {
			collections = cols
		}
		if saved, found, _ := s.db.FindSavedRelation(sess.UserID, s1, y1, s2, y2); found {
			savedItemID = saved.ID
			savedItemMastered = saved.Mastered
		}
	}

	…7774 tokens truncated…related, err := s.rels.RelatedAyahs(surah, ayah)
		if err != nil {
			internalError(w, r, err)
			return
		}
		lang := pageLang(r)
		writeJSON(w, http.StatusOK, map[string]any{
			"ayah":    relations.FormatAyahRef(surah, ayah),
			"related": s.withTranslations(lang, related),
		})
		return
	}

	http.NotFound(w, r)
}

func (s *server) handleAPIRelations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.requireAdmin(w, r) {
		return
	}

	var body struct {
		Ayah1 string `json:"ayah1"`
		Ayah2 string `json:"ayah2"`
		Note  string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	if err := s.rels.Add(body.Ayah1, body.Ayah2, body.Note); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
}

func (s *server) handleAPISurah(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	value, err := parseSingleIntPath(r.URL.Path, "/api/surah/", "/relations")
	if err != nil || value < 1 || value > 114 {
		http.NotFound(w, r)
		return
	}

	pairs, err := s.rels.PairsBySurah(value)
	if err != nil {
		internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"surah":      value,
		"surah_name": s.quran.SurahName(value),
		"relations":  pairs,
	})
}

func (s *server) handleAPIJuz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	value, err := parseSingleIntPath(r.URL.Path, "/api/juz/", "/relations")
	if err != nil || value < 1 || value > 30 {
		http.NotFound(w, r)
		return
	}

	pairs, err := s.rels.PairsByJuz(value)
	if err != nil {
		internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"juz":       value,
		"relations": pairs,
	})
}

func (s *server) render(w http.ResponseWriter, name string, data map[string]any) {
	if _, ok := data["CurrentUser"]; ok {
		w.Header().Set("Cache-Control", "no-store")
	}
	if isAdmin, _ := data["IsAdmin"].(bool); isAdmin {
		w.Header().Set("Cache-Control", "no-store")
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("render error template=%s: %v", name, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func parsePathInts(path, prefix string) (int, int, error) {
	trimmed := strings.TrimPrefix(path, prefix)
	parts := strings.Split(strings.Trim(trimmed, "/"), "/")
	if len(parts) != 2 {
		return 0, 0, errors.New("invalid path")
	}
	a, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, err
	}
	b, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, err
	}
	return a, b, nil
}

func parseSingleIntPath(path, prefix, suffix string) (int, error) {
	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, suffix) {
		return 0, errors.New("invalid path")
	}
	inner := strings.TrimSuffix(strings.TrimPrefix(path, prefix), suffix)
	inner = strings.Trim(inner, "/")
	value, err := strconv.Atoi(inner)
	if err != nil {
		return 0, err
	}
	return value, nil
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func (s *server) withCommonViewData(r *http.Request, data map[string]any) map[string]any {
	lang := pageLang(r)
	data["Lang"] = lang
	data["HomeURL"] = withLang("/", lang)
	data["LangARURL"] = switchLang(r, "ar")
	data["LangENURL"] = switchLang(r, "en")
	data["LangIDURL"] = switchLang(r, "id")
	data["DashboardURL"] = withLang("/dashboard", lang)
	data["CollectionsURL"] = withLang("/collections", lang)
	data["SurahIndexURL"] = withLang("/surah", lang)
	data["JuzIndexURL"] = withLang("/juz", lang)
	data["SearchURL"] = withLang("/search", lang)

	base := "https://" + r.Host
	data["CanonicalURL"] = base + r.URL.Path
	data["OGImageURL"] = base + "/static/og-image-v3.png?v=1"
	data["OGImageAlt"] = "HifzLink social card about reviewing similar Quran verses"
	if _, ok := data["Description"]; !ok {
		data["Description"] = "Master Quran mutashabihat: review similar verses and strengthen your memorization."
	}
	if s.umamiID != "" {
		data["UmamiID"] = s.umamiID
	}

	// Auth state for templates.
	data["AuthEnabled"] = s.googleConfigured()
	data["LoginURL"] = "/auth/login"
	data["LogoutURL"] = "/auth/logout"
	data["AdminURL"] = "/admin/users"
	data["AdminLogoutURL"] = "/admin/logout"
	if sess, ok := s.currentSession(r); ok {
		data["CurrentUser"] = userView{Name: sess.Name, Email: sess.Email}
		data["CSRFToken"] = s.csrfTokenFor(sess.ID)
	}
	if c, err := r.Cookie(cookieAdminSession); err == nil && s.validAdminSession(c.Value) {
		data["IsAdmin"] = true
		data["AdminCSRFToken"] = s.csrfTokenFor(s.adminToken)
	}

	return data
}

func pageLang(r *http.Request) string {
	return sanitizeLang(r.URL.Query().Get("lang"))
}

func sanitizeLang(v string) string {
	lang := strings.ToLower(strings.TrimSpace(v))
	switch lang {
	case "ar", "en", "id":
		return lang
	default:
		return "en"
	}
}

func withLang(path, lang string) string {
	lang = sanitizeLang(lang)
	if strings.Contains(path, "?") {
		return path + "&lang=" + lang
	}
	return path + "?lang=" + lang
}

func switchLang(r *http.Request, lang string) string {
	query := r.URL.Query()
	query.Set("lang", sanitizeLang(lang))
	return r.URL.Path + "?" + query.Encode()
}

func (s *server) translationFor(lang string, surah, ayah int) string {
	if s.trans == nil {
		return ""
	}
	text, _ := s.trans.Get(lang, surah, ayah)
	return stripHTMLTags(text)
}

// stripHTMLTags removes HTML tags from s, returning plain text.
// For tags like <sup>, the tag AND its inner content are removed entirely.
// For all other tags, only the tag markers are removed (content is kept).
func stripHTMLTags(s string) string {
	if !strings.ContainsRune(s, '<') {
		return s
	}
	// Tags whose content should also be removed (not just the tag markers).
	contentDropTags := map[string]bool{"sup": true}
	var buf strings.Builder
	for len(s) > 0 {
		start := strings.IndexByte(s, '<')
		if start < 0 {
			buf.WriteString(s)
			break
		}
		buf.WriteString(s[:start])
		s = s[start:]
		end := strings.IndexByte(s, '>')
		if end < 0 {
			break
		}
		inner := strings.ToLower(strings.TrimSpace(s[1:end]))
		s = s[end+1:]
		if strings.HasPrefix(inner, "/") {
			continue // closing tag, already past it
		}
		tagName := strings.FieldsFunc(inner, func(r rune) bool { return r == ' ' || r == '\t' || r == '\n' })
		if len(tagName) > 0 && contentDropTags[tagName[0]] {
			// Skip everything until the matching closing tag.
			closeTag := "</" + tagName[0] + ">"
			if idx := strings.Index(strings.ToLower(s), closeTag); idx >= 0 {
				s = s[idx+len(closeTag):]
			}
		}
	}
	return buf.String()
}

func (s *server) tafsirFor(lang string, surah, ayah int) template.HTML {
	if s.tafsir == nil || lang == "ar" {
		return ""
	}
	text, ok := s.tafsir.Get(lang, surah, ayah)
	if !ok {
		return ""
	}
	if lang == "en" {
		// en.ibn-kathir is already HTML from a trusted local file.
		return template.HTML(text) //nolint:gosec
	}
	// id.kemenag is plain text with \n\n paragraph breaks.
	return idTafsirToHTML(text)
}

func idTafsirToHTML(text string) template.HTML {
	paragraphs := strings.Split(text, "\n\n")
	var buf strings.Builder
	for _, p := range paragraphs {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		p = template.HTMLEscapeString(p)
		p = strings.ReplaceAll(p, "\n", "<br>")
		buf.WriteString("<p>")
		buf.WriteString(p)
		buf.WriteString("</p>\n")
	}
	return template.HTML(buf.String()) //nolint:gosec
}

func tafsirSourceLabel(lang string) string {
	switch lang {
	case "en":
		return "Ibn Kathir"
	case "id":
		return "Kemenag RI"
	default:
		return ""
	}
}

func (s *server) withTranslations(lang string, related []relations.AyahView) []relations.AyahView {
	if len(related) == 0 {
		return related
	}
	out := make([]relations.AyahView, len(related))
	copy(out, related)
	for i := range out {
		out[i].Translation = s.translationFor(lang, out[i].Surah, out[i].Ayah)
	}
	return out
}

func (s *server) renderNotFound(w http.ResponseWriter, r *http.Request, heading, message string) {
	w.WriteHeader(http.StatusNotFound)
	if pageLang(r) == "id" {
		switch {
		case strings.Contains(heading, "Ayah"):
			heading = "Ayat tidak ditemukan"
			if strings.Contains(message, "format") {
				message = "Referensi ayat tidak valid. Gunakan format surah:ayat, misalnya 60:8."
			} else {
				message = "Ayat ini tidak ada dalam data Quran. Periksa kembali nomor surah dan ayatnya."
			}
		case strings.Contains(heading, "Collection"):
			heading, message = "Koleksi tidak ditemukan", "Koleksi ini tidak tersedia."
		case strings.Contains(heading, "Relation"):
			heading, message = "Pasangan tidak ditemukan", "Pasangan ayat ini tidak tersedia."
		}
	}
	title := "Page not found"
	if pageLang(r) == "id" {
		title = "Halaman tidak ditemukan"
	}
	s.render(w, "not-found.html", s.learningData(r, map[string]any{
		"Title":   title,
		"Heading": heading,
		"Message": message,
	}))
}

func (s *server) handleRobotsTxt(w http.ResponseWriter, r *http.Request) {
	base := "https://" + r.Host
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	fmt.Fprintf(w, "User-agent: *\nAllow: /\n\nSitemap: %s/sitemap.xml\n", base)
}

func (s *server) handleSitemap(w http.ResponseWriter, r *http.Request) {
	base := "https://" + r.Host
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?>`)
	fmt.Fprint(w, "\n<urlset xmlns=\"http://www.sitemaps.org/schemas/sitemap/0.9\">\n")

	static := []string{"/", "/surah", "/juz", "/search", "/privacy", "/terms", "/methodology"}
	for _, path := range static {
		fmt.Fprintf(w, "  <url><loc>%s%s</loc></url>\n", base, path)
	}
	for i := 1; i <= 114; i++ {
		fmt.Fprintf(w, "  <url><loc>%s/surah/%d</loc></url>\n", base, i)
	}
	for i := 1; i <= 30; i++ {
		fmt.Fprintf(w, "  <url><loc>%s/juz/%d</loc></url>\n", base, i)
	}

	fmt.Fprint(w, "</urlset>\n")
}

func (s *server) handleLLMsTxt(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprint(w, `# HifzLink
> Open source tool for Quran memorization: find and compare mutashabihat (similar verses that confuse reciters during recitation).

Author: Said Ramadhan
Language: en
Topics: Quran, mutashabihat, Quran memorization, hifz, Islamic tools, verse comparison
Content-Type: interactive tool, verse database
Update-Cadence: irregular

## Key Pages
- /: Home - overview and entry point
- /methodology: Pair definitions, categories, sources, and review process
- /surah: Browse mutashabihat pairs organized by surah
- /juz: Browse mutashabihat pairs organized by juz
- /search: Search by ayah reference (60:8), surah number, or surah name
- /compare: Side-by-side verse comparison with word-level highlights

## About
HifzLink helps memorizers review similar Quran verses (mutashabihat). It indexes all 6,236 ayahs and curates pairs where reciters commonly slip from one verse to another. Free to use and open source, with no account required for browsing. Sign in with Google to save pairs to personal collections.

## Contact
https://hifz.click
`)
}

// wib is the Asia/Jakarta timezone (UTC+7).
var wib = time.FixedZone("WIB", 7*60*60)

// formatTimestamp parses a SQLite CURRENT_TIMESTAMP string ("2006-01-02 15:04:05" UTC)
// and returns a human-readable WIB string like "Apr 8, 2026, 21:30 WIB".
func formatTimestamp(raw string) string {
	if raw == "" {
		return ""
	}
	t, err := time.Parse("2006-01-02 15:04:05", raw)
	if err != nil {
		return raw
	}
	return t.In(wib).Format("Jan 2, 2006, 15:04 WIB (UTC+7)")
}

// formatUpdatedAt parses a SQLite CURRENT_TIMESTAMP string ("2006-01-02 15:04:05")
// and returns a human-readable WIB date string like "Apr 8, 2026".
func formatUpdatedAt(raw string) string {
	if raw == "" {
		return ""
	}
	t, err := time.Parse("2006-01-02 15:04:05", raw)
	if err != nil {
		return ""
	}
	return t.In(wib).Format("Jan 2, 2006")
}

// collectionView is a display-ready version of db.Collection with formatted timestamps.
type collectionView struct {
	ID          int64
	Name        string
	Description string
	CreatedAt   string
	ItemCount   int
}

func toCollectionView(c db.Collection) collectionView {
	return collectionView{
		ID:          c.ID,
		Name:        c.Name,
		Description: c.Description,
		CreatedAt:   formatTimestamp(c.CreatedAt),
		ItemCount:   c.ItemCount,
	}
}

func toCollectionViews(cs []db.Collection) []collectionView {
	out := make([]collectionView, 0, len(cs))
	for _, c := range cs {
		out = append(out, toCollectionView(c))
	}
	return out
}

func adminStatusMessage(code string) string {
	switch code {
	case "added":
		return "Relation added."
	case "edited":
		return "Relation updated."
	case "deleted":
		return "Relation deleted."
	default:
		return ""
	}
}

func (s *server) renderAdminRelationsPage(w http.ResponseWriter, r *http.Request, overrides map[string]any) {
	rows, err := s.rels.AllRelations()
	if err != nil {
		internalError(w, r, err)
		return
	}

	categoryOptions := adminCategoryOptions()
	categoryLabelMap := make(map[string]string, len(categoryOptions))
	for _, option := range categoryOptions {
		categoryLabelMap[option.Key] = option.Label
	}

	data := map[string]any{
		"Title":            "Admin Relations",
		"NoIndex":          true,
		"Relations":        rows,
		"StatusNotice":     adminStatusMessage(r.URL.Query().Get("status")),
		"FormAyah1":        "",
		"FormAyah2":        "",
		"FormNote":         "",
		"FormCategory":     "",
		"CategoryOptions":  categoryOptions,
		"CategoryLabelMap": categoryLabelMap,
		"AdminError":       "",
	}
	for k, v := range overrides {
		data[k] = v
	}

	s.render(w, "admin-relations.html", s.learningData(r, data))
}

type breadcrumbItem struct {
	Name string
	Path string
}

func websiteJSONLD(base string) template.JS {
	type ld struct {
		Context     string `json:"@context"`
		Type        string `json:"@type"`
		ID          string `json:"@id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		URL         string `json:"url"`
	}
	b, _ := json.Marshal(ld{
		Context:     "https://schema.org",
		Type:        "WebSite",
		ID:          base + "/#website",
		Name:        "hifzlink",
		Description: "Open source Quran mutashabihat review tool for huffaz",
		URL:         base,
	})
	return template.JS(b)
}

func breadcrumbJSONLD(base string, crumbs []breadcrumbItem) template.JS {
	type listItem struct {
		Type     string `json:"@type"`
		Position int    `json:"position"`
		Name     string `json:"name"`
		Item     string `json:"item"`
	}
	type ld struct {
		Context  string     `json:"@context"`
		Type     string     `json:"@type"`
		ItemList []listItem `json:"itemListElement"`
	}
	items := make([]listItem, len(crumbs))
	for i, c := range crumbs {
		items[i] = listItem{Type: "ListItem", Position: i + 1, Name: c.Name, Item: base + c.Path}
	}
	b, _ := json.Marshal(ld{Context: "https://schema.org", Type: "BreadcrumbList", ItemList: items})
	return template.JS(b)
}

// loadDotEnv reads key=value pairs from path and sets them via os.Setenv,
// skipping any key that is already set in the environment.
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return // .env is optional
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if k != "" && os.Getenv(k) == "" {
			os.Setenv(k, v)
		}
	}
}

func resolveBaseDir() (string, error) {
	candidates := []string{".", "..", "../.."}
	for _, candidate := range candidates {
		base, err := filepath.Abs(candidate)
		if err != nil {
			continue
		}
		if _, err := os.Stat(filepath.Join(base, "data", "quran.json")); err == nil {
			return base, nil
		}
	}

	wd, _ := os.Getwd()
	return "", fmt.Errorf("could not find data/quran.json from working directory %q", wd)
}

// requireAdmin checks for a valid admin session cookie.
// Admin access remains separate from Google user accounts.
func (s *server) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	if s == nil || strings.TrimSpace(s.adminUser) == "" || strings.TrimSpace(s.adminPass) == "" {
		http.Error(w, "admin auth not configured (set HIFZLINK_ADMIN_USER and HIFZLINK_ADMIN_PASS)", http.StatusServiceUnavailable)
		return false
	}
	c, err := r.Cookie(cookieAdminSession)
	if err == nil && s.validAdminSession(c.Value) {
		return true
	}
	http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
	return false
}

func collectionStatusMessage(code string) string {
	switch code {
	case "created":
		return "Collection created."
	case "saved":
		return "Saved to collection."
	case "duplicate":
		return "Already saved in this collection."
	case "removed":
		return "Item removed from collection."
	default:
		return ""
	}
}

func collectionItemTypeLabel(itemType string) string {
	switch itemType {
	case "ayah":
		return "Ayah"
	case "relation":
		return "Relation Pair"
	default:
		return "Item"
	}
}

func ayahComesAfter(s1, a1, s2, a2 int) bool {
	if s1 != s2 {
		return s1 > s2
	}
	return a1 > a2
}

func addQueryParam(path, key, value string) string {
	u, err := neturl.Parse(path)
	if err != nil {
		return path
	}
	q := u.Query()
	q.Set(key, value)
	u.RawQuery = q.Encode()
	return u.String()
}

func adminCategoryOptions() []adminCategoryOption {
	return []adminCategoryOption{
		{Key: "lafzi", Label: "Lafzi (near-identical wording)", Hint: "Verses are almost the same word-for-word — very small differences."},
		{Key: "addition_omission", Label: "Addition / omission", Hint: "One verse adds or drops a word compared to the other."},
		{Key: "word_swap", Label: "Word swap", Hint: "A word is replaced by a similar or related one."},
		{Key: "ending_variation", Label: "Ending variation", Hint: "Same opening, but the ending differs."},
		{Key: "order_change", Label: "Order change", Hint: "Same words, different sequence."},
		{Key: "pronoun_shift", Label: "Pronoun shift", Hint: "A pronoun changes — e.g. هُوَ vs هُمْ, or كُمْ vs هُمْ."},
		{Key: "other", Label: "Other", Hint: "Use only when none of the patterns fit clearly."},
	}
}

func adminErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case strings.Contains(message, "ayah1: ayah must be in surah:ayah format"):
		return "Ayah 1 must use surah:ayah format (example: 60:8)."
	case strings.Contains(message, "ayah2: ayah must be in surah:ayah format"):
		return "Ayah 2 must use surah:ayah format (example: 60:9)."
	case strings.Contains(message, "ayah1 not found in dataset"):
		return "Ayah 1 was not found in the local Quran dataset."
	case strings.Contains(message, "ayah2 not found in dataset"):
		return "Ayah 2 was not found in the local Quran dataset."
	case strings.Contains(message, "relation already exists"):
		return "That relation already exists."
	case strings.Contains(message, "invalid relation id"):
		return "Invalid relation ID."
	case strings.Contains(message, "relation not found"):
		return "Relation not found."
	default:
		return err.Error()
	}
}
