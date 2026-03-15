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
	rels    *relations.Service
	tmpl    *template.Template
	baseDir string
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

func (s *server) handleHome(w http.ResponseWriter, _ *http.Request) {
	s.render(w, "home.html", map[string]any{"Title": "Quran Murojaah"})
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

	surah, ayah, err := relations.ParseAyahRef(r.FormValue("ayah"))
	if err != nil {
		http.Error(w, "invalid ayah format, use surah:ayah", http.StatusBadRequest)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/ayah/%d/%d", surah, ayah), http.StatusSeeOther)
}

func (s *server) handleAyahPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	surah, ayah, err := parsePathInts(r.URL.Path, "/ayah/")
	if err != nil {
		http.NotFound(w, r)
		return
	}

	a, ok := s.quran.Get(surah, ayah)
	if !ok {
		http.NotFound(w, r)
		return
	}

	related, err := s.rels.RelatedAyahs(surah, ayah)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.render(w, "ayah.html", map[string]any{
		"Title":     fmt.Sprintf("Ayah %d:%d (%s)", surah, ayah, a.SurahName),
		"AyahRef":   relations.FormatAyahRef(surah, ayah),
		"Ayah":      a,
		"Related":   related,
		"SurahName": a.SurahName,
	})
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
		http.NotFound(w, r)
		return
	}

	s.render(w, "compare.html", map[string]any{
		"Title": "Compare",
		"Ayah1": ayah1,
		"Ayah2": ayah2,
		"Ref1":  relations.FormatAyahRef(s1, y1),
		"Ref2":  relations.FormatAyahRef(s2, y2),
	})
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

	s.render(w, "surah.html", map[string]any{
		"Title":     fmt.Sprintf("Surah %d (%s) Relations", surah, s.quran.SurahName(surah)),
		"Surah":     surah,
		"SurahName": s.quran.SurahName(surah),
		"Pairs":     pairs,
	})
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

	s.render(w, "juz.html", map[string]any{
		"Title": fmt.Sprintf("Juz %d Relations", juz),
		"Juz":   juz,
		"Pairs": pairs,
	})
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
		writeJSON(w, http.StatusOK, map[string]any{
			"surah":      a.Surah,
			"surah_name": a.SurahName,
			"ayah":       a.Ayah,
			"text":       a.TextAR,
			"juz":        a.Juz,
		})
		return
	}

	if len(parts) == 3 && parts[2] == "relations" {
		related, err := s.rels.RelatedAyahs(surah, ayah)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ayah":    relations.FormatAyahRef(surah, ayah),
			"related": related,
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
