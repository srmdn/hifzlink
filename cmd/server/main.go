package main

import (
	"bufio"
	"crypto/subtle"
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
)

type server struct {
	quran   *search.Store
	trans   *search.TranslationStore
	tafsir  *search.TranslationStore
	db      *db.Store
	rels    *relations.Service
	tmpl    *template.Template
	baseDir string

	adminUser    string
	adminPass    string
	adminToken   string // random token for admin session cookie, generated at startup
	adminLimiter *adminRateLimiter

	umamiID string

	// Quran Foundation OAuth2
	qfClientID     string
	qfClientSecret string
	qfAuthEndpoint string
	qfAPIBase      string
	qfRedirectURI  string // overrides auto-detected redirect URI (QF_REDIRECT_URI)

	// Quran Foundation Content API client (nil when QF is not configured)
	qf *qfclient.Client
}

// adminRateLimiter is a simple sliding-window rate limiter for admin endpoints.
type adminRateLimiter struct {
	mu      sync.Mutex
	entries map[string][]time.Time
}

func newAdminRateLimiter() *adminRateLimiter {
	return &adminRateLimiter{entries: make(map[string][]time.Time)}
}

func (l *adminRateLimiter) allow(ip string) bool {
	const maxAttempts = 20
	const window = time.Minute
	now := time.Now()
	cutoff := now.Add(-window)
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
	if len(times) >= maxAttempts {
		l.entries[ip] = times
		return false
	}
	l.entries[ip] = append(times, now)
	return true
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

	tmpl, err := template.ParseGlob(filepath.Join(baseDir, "web", "templates", "*.html"))
	if err != nil {
		log.Fatalf("failed to parse templates: %v", err)
	}

	s := &server{
		quran:        quran,
		trans:        translations,
		tafsir:       tafsirStore,
		db:           dbStore,
		rels:         relations.NewService(dbStore, quran),
		tmpl:         tmpl,
		baseDir:      baseDir,
		adminUser:    strings.TrimSpace(os.Getenv("HIFZLINK_ADMIN_USER")),
		adminPass:    strings.TrimSpace(os.Getenv("HIFZLINK_ADMIN_PASS")),
		adminLimiter: newAdminRateLimiter(),
		umamiID:      strings.TrimSpace(os.Getenv("HIFZLINK_UMAMI_ID")),

		qfClientID:     strings.TrimSpace(os.Getenv("QF_CLIENT_ID")),
		qfClientSecret: strings.TrimSpace(os.Getenv("QF_CLIENT_SECRET")),
		qfAuthEndpoint: strings.TrimSpace(os.Getenv("QF_AUTH_ENDPOINT")),
		qfAPIBase:      strings.TrimSpace(os.Getenv("QF_API_BASE")),
		qfRedirectURI:  strings.TrimSpace(os.Getenv("QF_REDIRECT_URI")),
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

	// Clean up expired sessions on startup.
	if err := dbStore.DeleteExpiredSessions(); err != nil {
		log.Printf("warn: delete expired sessions: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(filepath.Join(baseDir, "web", "static")))))

	mux.HandleFunc("/", s.handleHome)
	mux.HandleFunc("/privacy", s.handlePrivacy)
	mux.HandleFunc("/terms", s.handleTerms)
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
	mux.HandleFunc("/admin/login", s.handleAdminLogin)
	mux.HandleFunc("/admin/logout", s.handleAdminLogout)
	mux.HandleFunc("/admin/relations", s.handleAdminRelations)
	mux.HandleFunc("/admin/relations/", s.handleAdminRelationEdit)

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

func (s *server) handleHome(w http.ResponseWriter, r *http.Request) {
	count, err := s.db.CountRelations()
	if err != nil {
		count = 0
	}
	s.render(w, "home.html", s.withCommonViewData(r, map[string]any{
		"Title":       "hifzlink: Quran mutashabihat review",
		"Description": "hifzlink helps you identify and review mutashabihat: similar Quran verses that are easy to confuse during memorization. Try it yourself at hifz.click.",
		"PairCount":   count,
	}))
}

func (s *server) handlePrivacy(w http.ResponseWriter, r *http.Request) {
	s.render(w, "privacy.html", s.withCommonViewData(r, map[string]any{
		"Title":       "Privacy Policy — hifzlink",
		"Description": "Privacy policy for hifzlink, the open-source Quran mutashabihat review tool.",
	}))
}

func (s *server) handleTerms(w http.ResponseWriter, r *http.Request) {
	s.render(w, "terms.html", s.withCommonViewData(r, map[string]any{
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

	if q != "" {
		var dbRels []db.Relation
		var err error

		if surahNum, ayahNum, parseErr := relations.ParseAyahRef(q); parseErr == nil {
			queryLabel = fmt.Sprintf("pairs involving %s", relations.FormatAyahRef(surahNum, ayahNum))
			dbRels, err = s.db.ByAyah(surahNum, ayahNum)
		} else if n, atoiErr := strconv.Atoi(q); atoiErr == nil && n >= 1 && n <= 114 {
			queryLabel = fmt.Sprintf("pairs in Surah %d — %s", n, s.quran.SurahName(n))
			dbRels, err = s.db.BySurah(n)
		} else if n := search.SurahByName(q); n > 0 {
			queryLabel = fmt.Sprintf("pairs in Surah %d — %s", n, s.quran.SurahName(n))
			dbRels, err = s.db.BySurah(n)
		} else {
			errMsg = fmt.Sprintf("Could not interpret %q — try an ayah ref (60:8), surah number (60), or surah name (Al-Mumtahanah).", q)
		}

		if err != nil {
			internalError(w, r, err)
			return
		}

		lang := pageLang(r)

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

	s.render(w, "search.html", s.withCommonViewData(r, map[string]any{
		"Title":          "Search pairs",
		"Description":    "Search for Quran verses and find their mutashabihat — similar verses that are easy to confuse. Try it yourself at hifz.click.",
		"Query":          q,
		"QueryLabel":     queryLabel,
		"Results":        results,
		"ErrMsg":         errMsg,
		"FilterPills":    filterPills,
		"CategoryFilter": categoryFilter,
		"CategoryLabelMap": categoryLabelMap,
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

	collections, err := s.db.Collections()
	if err != nil {
		internalError(w, r, err)
		return
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

	s.render(w, "ayah.html", s.withCommonViewData(r, map[string]any{
		"Title":           fmt.Sprintf("Ayah %d:%d (%s)", surah, ayah, a.SurahName),
		"Description":     fmt.Sprintf("Review %s %d:%d and its mutashabihat — similar verses that are easy to confuse. Try it yourself at hifz.click.", a.SurahName, surah, ayah),
		"AyahRef":         relations.FormatAyahRef(surah, ayah),
		"Ayah":            a,
		"AyahTranslation": ayahTranslation,
		"AyahTafsir":      tafsirContent,
		"TafsirSource":    tafsirSourceLabel(lang),
		"Related":         related,
		"SurahName":       a.SurahName,
		"Collections":     collections,
		"SaveStatus":      collectionStatusMessage(r.URL.Query().Get("saved")),
		"QFAudioURL":      qfAudioURL,
	}))
}

func (s *server) handleComparePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	a1 := r.URL.Query().Get("ayah1")
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

	collections, err := s.db.Collections()
	if err != nil {
		internalError(w, r, err)
		return
	}

	var diff1, diff2 template.HTML
	rel, relFound, _ := s.db.ByPair(s1, y1, s2, y2)
	if relFound && rel.Highlights != "" {
		h := parseHighlights(rel.Highlights)
		diff1 = applyHighlights(ayah1.TextAR, h.Ayah1)
		diff2 = applyHighlights(ayah2.TextAR, h.Ayah2)
	} else {
		diff1, diff2 = diffHighlight(ayah1.TextAR, ayah2.TextAR)
	}

	type relatedPairView struct {
		Ref1       string
		Ref2       string
		CompareURL string
		Category   string
	}

	var relatedPairs []relatedPairView
	lang := pageLang(r)
	seen := map[string]bool{
		fmt.Sprintf("%d:%d-%d:%d", s1, y1, s2, y2): true,
		fmt.Sprintf("%d:%d-%d:%d", s2, y2, s1, y1): true,
	}
	addRelated := func(rels []db.Relation) {
		for _, rel := range rels {
			key := fmt.Sprintf("%d:%d-%d:%d", rel.Ayah1Surah, rel.Ayah1Ayah, rel.Ayah2Surah, rel.Ayah2Ayah)
			if seen[key] {
				continue
			}
			seen[key] = true
			ref1 := relations.FormatAyahRef(rel.Ayah1Surah, rel.Ayah1Ayah)
			ref2 := relations.FormatAyahRef(rel.Ayah2Surah, rel.Ayah2Ayah)
			relatedPairs = append(relatedPairs, relatedPairView{
				Ref1:       ref1,
				Ref2:       ref2,
				CompareURL: withLang(fmt.Sprintf("/compare?ayah1=%s&ayah2=%s", ref1, ref2), lang),
				Category:   rel.Category,
			})
		}
	}
	if rels1, err := s.db.ByAyah(s1, y1); err == nil {
		addRelated(rels1)
	}
	if rels2, err := s.db.ByAyah(s2, y2); err == nil {
		addRelated(rels2)
	}

	compareData := map[string]any{
		"Title":            "Compare",
		"Description":      "Compare two similar Quran verses side by side and spot the differences. Try it yourself at hifz.click.",
		"Ayah1":            ayah1,
		"Ayah1Translation": s.translationFor(pageLang(r), s1, y1),
		"Ayah2":            ayah2,
		"Ayah2Translation": s.translationFor(pageLang(r), s2, y2),
		"Ref1":             relations.FormatAyahRef(s1, y1),
		"Ref2":             relations.FormatAyahRef(s2, y2),
		"DiffText1":        diff1,
		"DiffText2":        diff2,
		"Collections":      collections,
		"SaveStatus":       collectionStatusMessage(r.URL.Query().Get("saved")),
		"RelatedPairs":     relatedPairs,
	}
	if relFound {
		compareData["AdminEditURL"] = fmt.Sprintf("/admin/relations/%d/edit", rel.ID)
		compareData["RelationUpdatedAt"] = formatUpdatedAt(rel.UpdatedAt)
	}
	s.render(w, "compare.html", s.withCommonViewData(r, compareData))
}

type qfBookmarkView struct {
	Ref string
	URL string
}

type dashboardItemView struct {
	ItemType      string
	ItemTypeLabel string
	CollectionID  int64
	Collection    string
	Ref1          string
	Ref2          string
	PrimaryURL    string
	SecondaryURL  string
	CreatedAt     string
	Note          string
}

func (s *server) handleDashboardPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	collections, err := s.db.RecentCollections(6)
	if err != nil {
		internalError(w, r, err)
		return
	}
	recentItems, err := s.db.RecentCollectionItems(12)
	if err != nil {
		internalError(w, r, err)
		return
	}

	lang := pageLang(r)
	items := make([]dashboardItemView, 0, len(recentItems))
	var resumeAyahURL string
	var resumeRelationURL string

	for _, item := range recentItems {
		ref1 := relations.FormatAyahRef(item.Ayah1Surah, item.Ayah1Ayah)
		view := dashboardItemView{
			ItemType:      item.ItemType,
			ItemTypeLabel: collectionItemTypeLabel(item.ItemType),
			CollectionID:  item.Collection,
			Collection:    item.CollectionName,
			Ref1:          ref1,
			CreatedAt:     formatTimestamp(item.CreatedAt),
			Note:          item.Note,
			PrimaryURL:    withLang(fmt.Sprintf("/ayah/%d/%d", item.Ayah1Surah, item.Ayah1Ayah), lang),
		}
		if item.ItemType == "relation" {
			ref2 := relations.FormatAyahRef(item.Ayah2Surah, item.Ayah2Ayah)
			view.Ref2 = ref2
			view.SecondaryURL = withLang(fmt.Sprintf("/compare?ayah1=%s&ayah2=%s", ref1, ref2), lang)
			if resumeRelationURL == "" {
				resumeRelationURL = view.SecondaryURL
			}
		}
		if resumeAyahURL == "" {
			resumeAyahURL = view.PrimaryURL
		}
		items = append(items, view)
	}

	allCollections, err := s.db.Collections()
	if err != nil {
		internalError(w, r, err)
		return
	}

	// Fetch QF bookmarks for logged-in users.
	var qfBookmarks []qfBookmarkView
	if s.qf != nil {
		if sess, ok := s.currentSession(r); ok {
			if bms, err := s.qf.GetBookmarks(sess.AccessToken); err != nil {
				log.Printf("qfclient: get bookmarks: %v", err)
			} else {
				for _, b := range bms {
					ref := relations.FormatAyahRef(b.SurahNum, b.AyahNum)
					qfBookmarks = append(qfBookmarks, qfBookmarkView{
						Ref: ref,
						URL: withLang(fmt.Sprintf("/ayah/%d/%d", b.SurahNum, b.AyahNum), lang),
					})
				}
			}
		}
	}

	s.render(w, "dashboard.html", s.withCommonViewData(r, map[string]any{
		"Title":             "Dashboard",
		"RecentCollections": toCollectionViews(collections),
		"AllCollections":    toCollectionViews(allCollections),
		"RecentItems":       items,
		"ResumeAyahURL":     resumeAyahURL,
		"ResumePairURL":     resumeRelationURL,
		"StatusNotice":      collectionStatusMessage(r.URL.Query().Get("status")),
		"QFBookmarks":       qfBookmarks,
	}))
}

func (s *server) handleCollectionsPage(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		collections, err := s.db.Collections()
		if err != nil {
			internalError(w, r, err)
			return
		}
		s.render(w, "collections.html", s.withCommonViewData(r, map[string]any{
			"Title":        "Collections",
			"Collections":  toCollectionViews(collections),
			"StatusNotice": collectionStatusMessage(r.URL.Query().Get("status")),
		}))
		return
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
		name := strings.TrimSpace(r.FormValue("name"))
		description := strings.TrimSpace(r.FormValue("description"))
		lang := sanitizeLang(r.FormValue("lang"))
		if name == "" {
			collections, _ := s.db.Collections()
			s.render(w, "collections.html", s.withCommonViewData(r, map[string]any{
				"Title":           "Collections",
				"Collections":     toCollectionViews(collections),
				"StatusNotice":    "",
				"CollectionError": "Collection name is required.",
				"FormName":        name,
				"FormDescription": description,
			}))
			return
		}

		_, err := s.db.CreateCollection(name, description)
		if err != nil {
			collections, _ := s.db.Collections()
			s.render(w, "collections.html", s.withCommonViewData(r, map[string]any{
				"Title":           "Collections",
				"Collections":     toCollectionViews(collections),
				"CollectionError": "Collection name already exists or is invalid.",
				"FormName":        name,
				"FormDescription": description,
			}))
			return
		}
		http.Redirect(w, r, withLang("/dashboard?status=created", lang), http.StatusSeeOther)
		return
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

type collectionItemView struct {
	ID            int64
	ItemType      string
	ItemTypeLabel string
	Surah1        int
	Ayah1         int
	Surah2        int
	Ayah2         int
	Ref1          string
	Ref2          string
	Surah1Name    string
	Surah2Name    string
	Arabic1       string
	Arabic2       string
	Translation1  string
	Translation2  string
	CompareURL    string
	Note          string
	CreatedAt     string
}

func (s *server) handleCollectionDetailPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	idPath := strings.Trim(strings.TrimPrefix(r.URL.Path, "/collections/"), "/")
	if idPath == "" {
		http.NotFound(w, r)
		return
	}
	id, err := strconv.ParseInt(idPath, 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	collection, err := s.db.CollectionByID(id)
	if err != nil {
		s.renderNotFound(w, r, "Collection not found", "This collection does not exist.")
		return
	}
	items, err := s.db.CollectionItems(id)
	if err != nil {
		internalError(w, r, err)
		return
	}

	lang := pageLang(r)
	viewItems := make([]collectionItemView, 0, len(items))
	for _, item := range items {
		if item.ItemType == "ayah" {
			a1, ok := s.quran.Get(item.Ayah1Surah, item.Ayah1Ayah)
			if !ok {
				continue
			}
			viewItems = append(viewItems, collectionItemView{
				ID:            item.ID,
				ItemType:      item.ItemType,
				ItemTypeLabel: collectionItemTypeLabel(item.ItemType),
				Surah1:        item.Ayah1Surah,
				Ayah1:         item.Ayah1Ayah,
				Ref1:          relations.FormatAyahRef(item.Ayah1Surah, item.Ayah1Ayah),
				Surah1Name:    a1.SurahName,
				Arabic1:       a1.TextAR,
				Translation1:  s.translationFor(lang, item.Ayah1Surah, item.Ayah1Ayah),
				Note:          item.Note,
				CreatedAt:     formatTimestamp(item.CreatedAt),
			})
			continue
		}
		a1, ok1 := s.quran.Get(item.Ayah1Surah, item.Ayah1Ayah)
		a2, ok2 := s.quran.Get(item.Ayah2Surah, item.Ayah2Ayah)
		if !ok1 || !ok2 {
			continue
		}
		ref1 := relations.FormatAyahRef(item.Ayah1Surah, item.Ayah1Ayah)
		ref2 := relations.FormatAyahRef(item.Ayah2Surah, item.Ayah2Ayah)
		viewItems = append(viewItems, collectionItemView{
			ID:            item.ID,
			ItemType:      item.ItemType,
			ItemTypeLabel: collectionItemTypeLabel(item.ItemType),
			Surah1:        item.Ayah1Surah,
			Ayah1:         item.Ayah1Ayah,
			Surah2:        item.Ayah2Surah,
			Ayah2:         item.Ayah2Ayah,
			Ref1:          ref1,
			Ref2:          ref2,
			Surah1Name:    a1.SurahName,
			Surah2Name:    a2.SurahName,
			Arabic1:       a1.TextAR,
			Arabic2:       a2.TextAR,
			Translation1:  s.translationFor(lang, item.Ayah1Surah, item.Ayah1Ayah),
			Translation2:  s.translationFor(lang, item.Ayah2Surah, item.Ayah2Ayah),
			CompareURL:    withLang(fmt.Sprintf("/compare?ayah1=%s&ayah2=%s", ref1, ref2), lang),
			Note:          item.Note,
			CreatedAt:     formatTimestamp(item.CreatedAt),
		})
	}

	s.render(w, "collection-detail.html", s.withCommonViewData(r, map[string]any{
		"Title":        collection.Name,
		"Collection":   toCollectionView(collection),
		"Items":        viewItems,
		"StatusNotice": collectionStatusMessage(r.URL.Query().Get("status")),
	}))
}

func (s *server) handleCollectionItemsPost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	collectionID, err := strconv.ParseInt(strings.TrimSpace(r.FormValue("collection_id")), 10, 64)
	if err != nil || collectionID <= 0 {
		http.Error(w, "invalid collection", http.StatusBadRequest)
		return
	}
	itemType := strings.ToLower(strings.TrimSpace(r.FormValue("item_type")))
	note := strings.TrimSpace(r.FormValue("note"))
	lang := sanitizeLang(r.FormValue("lang"))

	ayah1S, ayah1A, err := relations.ParseAyahRef(strings.TrimSpace(r.FormValue("ayah1")))
	if err != nil {
		http.Error(w, "invalid ayah1", http.StatusBadRequest)
		return
	}
	if _, ok := s.quran.Get(ayah1S, ayah1A); !ok {
		http.Error(w, "ayah1 not found", http.StatusBadRequest)
		return
	}

	item := db.CollectionItem{
		Collection: collectionID,
		ItemType:   itemType,
		Ayah1Surah: ayah1S,
		Ayah1Ayah:  ayah1A,
		Note:       note,
	}
	if itemType == "relation" {
		ayah2S, ayah2A, err := relations.ParseAyahRef(strings.TrimSpace(r.FormValue("ayah2")))
		if err != nil {
			http.Error(w, "invalid ayah2", http.StatusBadRequest)
			return
		}
		if _, ok := s.quran.Get(ayah2S, ayah2A); !ok {
			http.Error(w, "ayah2 not found", http.StatusBadRequest)
			return
		}
		if ayahComesAfter(ayah1S, ayah1A, ayah2S, ayah2A) {
			ayah1S, ayah2S = ayah2S, ayah1S
			ayah1A, ayah2A = ayah2A, ayah1A
		}
		item.Ayah1Surah = ayah1S
		item.Ayah1Ayah = ayah1A
		item.Ayah2Surah = ayah2S
		item.Ayah2Ayah = ayah2A
	} else if itemType != "ayah" {
		http.Error(w, "invalid item type", http.StatusBadRequest)
		return
	}

	inserted, err := s.db.AddCollectionItem(item)
	if err != nil {
		internalError(w, r, err)
		return
	}

	// If user is logged in and QF is configured, sync the primary ayah as a QF bookmark.
	if s.qf != nil && inserted {
		if sess, ok := s.currentSession(r); ok {
			go func() {
				if _, err := s.qf.AddBookmark(sess.AccessToken, item.Ayah1Surah, item.Ayah1Ayah); err != nil {
					log.Printf("qfclient: sync bookmark %d:%d: %v", item.Ayah1Surah, item.Ayah1Ayah, err)
				}
			}()
		}
	}

	returnTo := strings.TrimSpace(r.FormValue("return_to"))
	if returnTo == "" || !isSafeRedirect(returnTo) {
		returnTo = fmt.Sprintf("/collections/%d", collectionID)
	}
	statusCode := "saved"
	if !inserted {
		statusCode = "duplicate"
	}
	redirectWithSaved := addQueryParam(returnTo, "saved", statusCode)
	http.Redirect(w, r, withLang(redirectWithSaved, lang), http.StatusSeeOther)
}

func (s *server) handleCollectionItemsDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	itemID, err := strconv.ParseInt(strings.TrimSpace(r.FormValue("item_id")), 10, 64)
	if err != nil || itemID <= 0 {
		http.Error(w, "invalid item", http.StatusBadRequest)
		return
	}
	collectionID, err := strconv.ParseInt(strings.TrimSpace(r.FormValue("collection_id")), 10, 64)
	if err != nil || collectionID <= 0 {
		http.Error(w, "invalid collection", http.StatusBadRequest)
		return
	}
	lang := sanitizeLang(r.FormValue("lang"))
	if err := s.db.DeleteCollectionItem(itemID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, withLang(fmt.Sprintf("/collections/%d?status=removed", collectionID), lang), http.StatusSeeOther)
}

type surahIndexItem struct {
	Number        int
	Name          string
	RelationCount int
}

type juzIndexItem struct {
	Number        int
	RelationCount int
}

func (s *server) handleSurahIndexPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	counts, err := s.db.RelationCountBySurah()
	if err != nil {
		internalError(w, r, err)
		return
	}

	items := make([]surahIndexItem, 114)
	for i := 1; i <= 114; i++ {
		items[i-1] = surahIndexItem{
			Number:        i,
			Name:          s.quran.SurahName(i),
			RelationCount: counts[i],
		}
	}

	s.render(w, "surah-index.html", s.withCommonViewData(r, map[string]any{
		"Title":       "Browse by Surah",
		"Description": "Browse all 114 surahs and their mutashabihat relations. Try it yourself at hifz.click.",
		"Surahs": items,
	}))
}

func (s *server) handleJuzIndexPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Compute juz relation counts from all relations via quran store.
	allRels, err := s.db.All()
	if err != nil {
		internalError(w, r, err)
		return
	}

	counts := make(map[int]int)
	seen := make(map[string]bool)
	for _, rel := range allRels {
		a1, ok1 := s.quran.Get(rel.Ayah1Surah, rel.Ayah1Ayah)
		a2, ok2 := s.quran.Get(rel.Ayah2Surah, rel.Ayah2Ayah)
		if !ok1 || !ok2 {
			continue
		}
		key1 := fmt.Sprintf("%d:%d", rel.ID, a1.Juz)
		if !seen[key1] {
			counts[a1.Juz]++
			seen[key1] = true
		}
		key2 := fmt.Sprintf("%d:%d", rel.ID, a2.Juz)
		if !seen[key2] {
			counts[a2.Juz]++
			seen[key2] = true
		}
	}

	items := make([]juzIndexItem, 30)
	for i := 1; i <= 30; i++ {
		items[i-1] = juzIndexItem{Number: i, RelationCount: counts[i]}
	}

	s.render(w, "juz-index.html", s.withCommonViewData(r, map[string]any{
		"Title":       "Browse by Juz",
		"Description": "Browse mutashabihat relations across all 30 juz of the Quran. Try it yourself at hifz.click.",
		"Juzs":  items,
	}))
}

func (s *server) handleSurahPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	surah, err := parseSingleIntPath(r.URL.Path, "/surah/", "")
	if err != nil || surah < 1 || surah > 114 {
		http.NotFound(w, r)
		return
	}

	pairs, err := s.rels.PairsBySurah(surah)
	if err != nil {
		internalError(w, r, err)
		return
	}

	s.render(w, "surah.html", s.withCommonViewData(r, map[string]any{
		"Title":       fmt.Sprintf("Surah %d (%s) Relations", surah, s.quran.SurahName(surah)),
		"Description": fmt.Sprintf("Browse mutashabihat relations in Surah %d — %s. Try it yourself at hifz.click.", surah, s.quran.SurahName(surah)),
		"Surah":     surah,
		"SurahName": s.quran.SurahName(surah),
		"Pairs":     pairs,
	}))
}

func (s *server) handleJuzPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	juz, err := parseSingleIntPath(r.URL.Path, "/juz/", "")
	if err != nil || juz < 1 || juz > 30 {
		http.NotFound(w, r)
		return
	}

	pairs, err := s.rels.PairsByJuz(juz)
	if err != nil {
		internalError(w, r, err)
		return
	}

	s.render(w, "juz.html", s.withCommonViewData(r, map[string]any{
		"Title":       fmt.Sprintf("Juz %d Relations", juz),
		"Description": fmt.Sprintf("Browse mutashabihat relations in Juz %d. Try it yourself at hifz.click.", juz),
		"Juz":   juz,
		"Pairs": pairs,
	}))
}

func (s *server) handleAdminRelations(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.renderAdminRelationsPage(w, r, nil)
		return
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}

		lang := sanitizeLang(r.FormValue("lang"))
		action := strings.ToLower(strings.TrimSpace(r.FormValue("action")))

		switch action {
		case "add":
			ayah1 := strings.TrimSpace(r.FormValue("ayah1"))
			ayah2 := strings.TrimSpace(r.FormValue("ayah2"))
			note := strings.TrimSpace(r.FormValue("note"))
			category := strings.TrimSpace(r.FormValue("category"))
			if err := s.rels.AddWithCategory(ayah1, ayah2, note, category); err != nil {
				s.renderAdminRelationsPage(w, r, map[string]any{
					"AdminError":   adminErrorMessage(err),
					"FormAyah1":    ayah1,
					"FormAyah2":    ayah2,
					"FormNote":     note,
					"FormCategory": category,
				})
				return
			}
			http.Redirect(w, r, withLang("/admin/relations?status=added", lang), http.StatusSeeOther)
			return
		case "edit":
			idValue := strings.TrimSpace(r.FormValue("id"))
			id, err := strconv.ParseInt(idValue, 10, 64)
			if err != nil {
				s.renderAdminRelationsPage(w, r, map[string]any{
					"AdminError": "invalid relation id",
				})
				return
			}
			ayah1 := strings.TrimSpace(r.FormValue("ayah1"))
			ayah2 := strings.TrimSpace(r.FormValue("ayah2"))
			note := strings.TrimSpace(r.FormValue("note"))
			category := strings.TrimSpace(r.FormValue("category"))
			// Only update highlights if the user interacted with the word picker.
			// highlights_current carries the existing value as a fallback.
			highlights := r.FormValue("highlights_current")
			if r.FormValue("highlights_modified") == "1" {
				highlights = buildHighlightsJSON(r.FormValue("highlights_ayah1"), r.FormValue("highlights_ayah2"))
			}
			if err := s.rels.UpdateByID(id, ayah1, ayah2, note, category, highlights); err != nil {
				s.renderAdminRelationsPage(w, r, map[string]any{
					"AdminError": adminErrorMessage(err),
				})
				return
			}
			http.Redirect(w, r, withLang("/admin/relations?status=edited", lang), http.StatusSeeOther)
			return
		case "delete":
			idValue := strings.TrimSpace(r.FormValue("id"))
			id, err := strconv.ParseInt(idValue, 10, 64)
			if err != nil {
				s.renderAdminRelationsPage(w, r, map[string]any{
					"AdminError": "invalid relation id",
				})
				return
			}
			if err := s.rels.DeleteByID(id); err != nil {
				s.renderAdminRelationsPage(w, r, map[string]any{
					"AdminError": adminErrorMessage(err),
				})
				return
			}
			http.Redirect(w, r, withLang("/admin/relations?status=deleted", lang), http.StatusSeeOther)
			return
		default:
			s.renderAdminRelationsPage(w, r, map[string]any{
				"AdminError": "invalid action",
			})
			return
		}
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleAdminRelationEdit handles GET /admin/relations/{id}/edit and POST /admin/relations/{id}/edit.
func (s *server) handleAdminRelationEdit(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}

	// Parse id from /admin/relations/{id}/edit
	trimmed := strings.TrimPrefix(r.URL.Path, "/admin/relations/")
	trimmed = strings.TrimSuffix(trimmed, "/edit")
	id, err := strconv.ParseInt(strings.TrimSpace(trimmed), 10, 64)
	if err != nil || id <= 0 {
		http.NotFound(w, r)
		return
	}

	returnCategory := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("return_category")))
	lang := pageLang(r)

	backURL := withLang("/admin/relations", lang)
	if returnCategory != "" {
		backURL = withLang("/admin/relations?category="+returnCategory, lang)
	}

	switch r.Method {
	case http.MethodGet:
		rel, ok, err := s.rels.RelationByID(id)
		if err != nil {
			internalError(w, r, err)
			return
		}
		if !ok {
			s.renderNotFound(w, r, "Relation not found", "This relation does not exist.")
			return
		}

		var w1, w2 []string
		var diff1, diff2 template.HTML
		s1, y1, err1 := relations.ParseAyahRef(rel.Ayah1)
		s2, y2, err2 := relations.ParseAyahRef(rel.Ayah2)
		if err1 == nil && err2 == nil {
			a1, ok1 := s.quran.Get(s1, y1)
			a2, ok2 := s.quran.Get(s2, y2)
			if ok1 {
				w1 = strings.Fields(a1.TextAR)
			}
			if ok2 {
				w2 = strings.Fields(a2.TextAR)
			}
			if ok1 && ok2 {
				if rel.Highlights != "" {
					h := parseHighlights(rel.Highlights)
					diff1 = applyHighlights(a1.TextAR, h.Ayah1)
					diff2 = applyHighlights(a2.TextAR, h.Ayah2)
				} else {
					diff1, diff2 = diffHighlight(a1.TextAR, a2.TextAR)
				}
			}
		}

		categoryOptions := adminCategoryOptions()
		s.render(w, "admin-edit.html", s.withCommonViewData(r, map[string]any{
			"Title":           "Edit Relation",
			"Relation":        rel,
			"RelationUpdatedAt": formatUpdatedAt(rel.UpdatedAt),
			"DiffText1":       diff1,
			"DiffText2":       diff2,
			"Ayah1Words":      w1,
			"Ayah2Words":      w2,
			"CategoryOptions": categoryOptions,
			"BackURL":         backURL,
			"ReturnCategory":  returnCategory,
			"AdminError":      "",
		}))

	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}

		returnCat := strings.ToLower(strings.TrimSpace(r.FormValue("return_category")))
		postLang := sanitizeLang(r.FormValue("lang"))

		ayah1 := strings.TrimSpace(r.FormValue("ayah1"))
		ayah2 := strings.TrimSpace(r.FormValue("ayah2"))
		note := strings.TrimSpace(r.FormValue("note"))
		category := strings.TrimSpace(r.FormValue("category"))
		highlights := r.FormValue("highlights_current")
		if r.FormValue("highlights_modified") == "1" {
			highlights = buildHighlightsJSON(r.FormValue("highlights_ayah1"), r.FormValue("highlights_ayah2"))
		}

		if err := s.rels.UpdateByID(id, ayah1, ayah2, note, category, highlights); err != nil {
			rel, _, _ := s.rels.RelationByID(id)
			var w1, w2 []string
			s1, y1, err1 := relations.ParseAyahRef(ayah1)
			s2, y2, err2 := relations.ParseAyahRef(ayah2)
			if err1 == nil && err2 == nil {
				if a1, ok1 := s.quran.Get(s1, y1); ok1 {
					w1 = strings.Fields(a1.TextAR)
				}
				if a2, ok2 := s.quran.Get(s2, y2); ok2 {
					w2 = strings.Fields(a2.TextAR)
				}
			}
			categoryOptions := adminCategoryOptions()
			backURL := withLang("/admin/relations", postLang)
			if returnCat != "" {
				backURL = withLang("/admin/relations?category="+returnCat, postLang)
			}
			s.render(w, "admin-edit.html", s.withCommonViewData(r, map[string]any{
				"Title":           "Edit Relation",
				"Relation":        rel,
				"Ayah1Words":      w1,
				"Ayah2Words":      w2,
				"CategoryOptions": categoryOptions,
				"BackURL":         backURL,
				"ReturnCategory":  returnCat,
				"AdminError":      adminErrorMessage(err),
			}))
			return
		}

		redirectURL := withLang("/admin/relations?status=edited", postLang)
		if returnCat != "" {
			redirectURL = withLang("/admin/relations?category="+returnCat+"&status=edited", postLang)
		}
		http.Redirect(w, r, redirectURL, http.StatusSeeOther)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *server) handleAPIAyah(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	trimmed := strings.TrimPrefix(r.URL.Path, "/api/ayah/")
	parts := strings.Split(strings.Trim(trimmed, "/"), "/")
	if len(parts) < 2 {
		http.NotFound(w, r)
		return
	}

	surah, err := strconv.Atoi(parts[0])
	if err != nil {
		http.NotFound(w, r)
		return
	}
	ayah, err := strconv.Atoi(parts[1])
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if len(parts) == 2 {
		a, ok := s.quran.Get(surah, ayah)
		if !ok {
			http.NotFound(w, r)
			return
		}
		resp := map[string]any{
			"surah":      a.Surah,
			"surah_name": a.SurahName,
			"ayah":       a.Ayah,
			"text":       a.TextAR,
			"juz":        a.Juz,
		}
		lang := pageLang(r)
		if tr := s.translationFor(lang, surah, ayah); tr != "" {
			resp["translation_lang"] = lang
			resp["translation_text"] = tr
		}
		writeJSON(w, http.StatusOK, resp)
		return
	}

	if len(parts) == 3 && parts[2] == "relations" {
		related, err := s.rels.RelatedAyahs(surah, ayah)
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
	data["OGImageURL"] = base + "/static/og-image.png"
	if _, ok := data["Description"]; !ok {
		data["Description"] = "Master Quran mutashabihat — review similar verses and strengthen your memorization."
	}
	if s.umamiID != "" {
		data["UmamiID"] = s.umamiID
	}

	// Auth state for templates.
	data["QFEnabled"] = s.qfConfigured()
	data["LoginURL"] = "/auth/login"
	data["LogoutURL"] = "/auth/logout"
	data["AdminURL"] = "/admin/relations"
	data["AdminLogoutURL"] = "/admin/logout"
	if sess, ok := s.currentSession(r); ok {
		data["CurrentUser"] = userView{Name: sess.Name, Email: sess.Email}
	}
	if c, err := r.Cookie(cookieAdminSession); err == nil && c.Value != "" &&
		subtle.ConstantTimeCompare([]byte(c.Value), []byte(s.adminToken)) == 1 {
		data["IsAdmin"] = true
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
		return "ar"
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
	s.render(w, "not-found.html", s.withCommonViewData(r, map[string]any{
		"Title":   "Not Found",
		"Heading": heading,
		"Message": message,
	}))
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

	s.render(w, "admin-relations.html", s.withCommonViewData(r, data))
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

func (s *server) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	if s == nil || strings.TrimSpace(s.adminUser) == "" || strings.TrimSpace(s.adminPass) == "" {
		http.Error(w, "admin auth not configured (set HIFZLINK_ADMIN_USER and HIFZLINK_ADMIN_PASS)", http.StatusServiceUnavailable)
		return false
	}
	c, err := r.Cookie(cookieAdminSession)
	if err == nil && c.Value != "" && subtle.ConstantTimeCompare([]byte(c.Value), []byte(s.adminToken)) == 1 {
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
