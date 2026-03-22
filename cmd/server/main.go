package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/srmdn/hifzlink/internal/db"
	"github.com/srmdn/hifzlink/internal/relations"
	"github.com/srmdn/hifzlink/internal/search"
)

type server struct {
	quran   *search.Store
	trans   *search.TranslationStore
	rels    *relations.Service
	tmpl    *template.Template
	baseDir string
}

type adminCategoryOption struct {
	Key   string
	Label string
	Hint  string
}

func main() {
	baseDir, err := resolveBaseDir()
	if err != nil {
		log.Fatalf("failed to locate project root: %v", err)
	}

	quran, err := search.Load(filepath.Join(baseDir, "data", "quran.json"))
	if err != nil {
		log.Fatalf("failed to load quran dataset: %v", err)
	}

	translations, err := search.LoadTranslations(filepath.Join(baseDir, "data", "translations"), "en", "id")
	if err != nil {
		log.Fatalf("failed to load translations: %v", err)
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
		quran:   quran,
		trans:   translations,
		rels:    relations.NewService(dbStore, quran),
		tmpl:    tmpl,
		baseDir: baseDir,
	}

	mux := http.NewServeMux()
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(filepath.Join(baseDir, "web", "static")))))

	mux.HandleFunc("/", s.handleHome)
	mux.HandleFunc("/search", s.handleSearch)
	mux.HandleFunc("/ayah/", s.handleAyahPage)
	mux.HandleFunc("/compare", s.handleComparePage)
	mux.HandleFunc("/surah/", s.handleSurahPage)
	mux.HandleFunc("/juz/", s.handleJuzPage)
	mux.HandleFunc("/admin/relations", s.handleAdminRelations)

	mux.HandleFunc("/api/ayah/", s.handleAPIAyah)
	mux.HandleFunc("/api/relations", s.handleAPIRelations)
	mux.HandleFunc("/api/surah/", s.handleAPISurah)
	mux.HandleFunc("/api/juz/", s.handleAPIJuz)

	addr := ":8080"
	log.Printf("server listening on %s", addr)
	if err := http.ListenAndServe(addr, logRequests(mux)); err != nil {
		log.Fatal(err)
	}
}

func (s *server) handleHome(w http.ResponseWriter, r *http.Request) {
	data := map[string]any{
		"Title":       "Quran Murojaah",
		"SearchQuery": r.URL.Query().Get("q"),
		"SearchError": searchErrorMessage(r.URL.Query().Get("error")),
	}
	s.render(w, "home.html", withCommonViewData(r, data))
}

func (s *server) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	raw := strings.TrimSpace(r.FormValue("ayah"))
	surah, ayah, err := relations.ParseAyahRef(raw)
	if err != nil {
		lang := sanitizeLang(r.FormValue("lang"))
		http.Redirect(w, r, fmt.Sprintf("/?error=invalid_ref&q=%s&lang=%s", raw, lang), http.StatusSeeOther)
		return
	}

	lang := sanitizeLang(r.FormValue("lang"))
	http.Redirect(w, r, withLang(fmt.Sprintf("/ayah/%d/%d", surah, ayah), lang), http.StatusSeeOther)
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	lang := pageLang(r)
	related = s.withTranslations(lang, related)
	ayahTranslation := s.translationFor(lang, surah, ayah)

	s.render(w, "ayah.html", withCommonViewData(r, map[string]any{
		"Title":           fmt.Sprintf("Ayah %d:%d (%s)", surah, ayah, a.SurahName),
		"AyahRef":         relations.FormatAyahRef(surah, ayah),
		"Ayah":            a,
		"AyahTranslation": ayahTranslation,
		"Related":         related,
		"SurahName":       a.SurahName,
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

	s.render(w, "compare.html", withCommonViewData(r, map[string]any{
		"Title":            "Compare",
		"Ayah1":            ayah1,
		"Ayah1Translation": s.translationFor(pageLang(r), s1, y1),
		"Ayah2":            ayah2,
		"Ayah2Translation": s.translationFor(pageLang(r), s2, y2),
		"Ref1":             relations.FormatAyahRef(s1, y1),
		"Ref2":             relations.FormatAyahRef(s2, y2),
	}))
}

func (s *server) handleSurahPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	surah, err := parseSingleIntPath(r.URL.Path, "/surah/", "/relations")
	if err != nil {
		http.NotFound(w, r)
		return
	}

	pairs, err := s.rels.PairsBySurah(surah)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.render(w, "surah.html", withCommonViewData(r, map[string]any{
		"Title":     fmt.Sprintf("Surah %d (%s) Relations", surah, s.quran.SurahName(surah)),
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
	juz, err := parseSingleIntPath(r.URL.Path, "/juz/", "/relations")
	if err != nil {
		http.NotFound(w, r)
		return
	}

	pairs, err := s.rels.PairsByJuz(juz)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.render(w, "juz.html", withCommonViewData(r, map[string]any{
		"Title": fmt.Sprintf("Juz %d Relations", juz),
		"Juz":   juz,
		"Pairs": pairs,
	}))
}

func (s *server) handleAdminRelations(w http.ResponseWriter, r *http.Request) {
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
			if err := s.rels.UpdateByID(id, ayah1, ayah2, note, category); err != nil {
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
			http.Error(w, err.Error(), http.StatusInternalServerError)
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
	if err != nil {
		http.NotFound(w, r)
		return
	}

	pairs, err := s.rels.PairsBySurah(value)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
	if err != nil {
		http.NotFound(w, r)
		return
	}

	pairs, err := s.rels.PairsByJuz(value)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
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

func withCommonViewData(r *http.Request, data map[string]any) map[string]any {
	lang := pageLang(r)
	data["Lang"] = lang
	data["HomeURL"] = withLang("/", lang)
	data["LangARURL"] = switchLang(r, "ar")
	data["LangENURL"] = switchLang(r, "en")
	data["LangIDURL"] = switchLang(r, "id")
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
	return text
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
	s.render(w, "not-found.html", withCommonViewData(r, map[string]any{
		"Title":   "Not Found",
		"Heading": heading,
		"Message": message,
	}))
}

func searchErrorMessage(code string) string {
	switch code {
	case "invalid_ref":
		return "Invalid format — use surah:ayah (e.g. 60:8)."
	default:
		return ""
	}
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	categoryFilter := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("category")))
	if categoryFilter != "" {
		filtered := make([]relations.AdminRelationView, 0, len(rows))
		for _, row := range rows {
			if strings.EqualFold(row.Category, categoryFilter) {
				filtered = append(filtered, row)
			}
		}
		rows = filtered
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
		"CategoryFilter":   categoryFilter,
		"CategoryOptions":  categoryOptions,
		"CategoryLabelMap": categoryLabelMap,
		"AdminError":       "",
	}
	for k, v := range overrides {
		data[k] = v
	}

	s.render(w, "admin-relations.html", withCommonViewData(r, data))
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

func adminCategoryOptions() []adminCategoryOption {
	return []adminCategoryOption{
		{Key: "lafzi", Label: "Lafzi (near-identical wording)", Hint: "Use when verses are very similar in wording with small wording changes."},
		{Key: "maana", Label: "Maana (similar meaning)", Hint: "Use when wording differs more, but meaning/theme overlaps."},
		{Key: "siyam", Label: "Siyam (fasting)", Hint: "Use for verses related to fasting rulings or guidance."},
		{Key: "aqidah", Label: "Aqidah (belief)", Hint: "Use for belief, tawhid, iman, and related theology."},
		{Key: "adab", Label: "Adab (manners/ethics)", Hint: "Use for etiquette and character guidance."},
		{Key: "other", Label: "Other", Hint: "Use only when none of the categories fit clearly."},
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
