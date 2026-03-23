package main

import (
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

	"github.com/srmdn/hifzlink/internal/db"
	"github.com/srmdn/hifzlink/internal/relations"
	"github.com/srmdn/hifzlink/internal/search"
)

type server struct {
	quran   *search.Store
	trans   *search.TranslationStore
	db      *db.Store
	rels    *relations.Service
	tmpl    *template.Template
	baseDir string

	adminUser string
	adminPass string
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
		quran:     quran,
		trans:     translations,
		db:        dbStore,
		rels:      relations.NewService(dbStore, quran),
		tmpl:      tmpl,
		baseDir:   baseDir,
		adminUser: strings.TrimSpace(os.Getenv("HIFZLINK_ADMIN_USER")),
		adminPass: strings.TrimSpace(os.Getenv("HIFZLINK_ADMIN_PASS")),
	}

	mux := http.NewServeMux()
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(filepath.Join(baseDir, "web", "static")))))

	mux.HandleFunc("/", s.handleHome)
	mux.HandleFunc("/search", s.handleSearch)
	mux.HandleFunc("/ayah/", s.handleAyahPage)
	mux.HandleFunc("/compare", s.handleComparePage)
	mux.HandleFunc("/surah/", s.handleSurahPage)
	mux.HandleFunc("/juz/", s.handleJuzPage)
	mux.HandleFunc("/dashboard", s.handleDashboardPage)
	mux.HandleFunc("/collections", s.handleCollectionsPage)
	mux.HandleFunc("/collections/", s.handleCollectionDetailPage)
	mux.HandleFunc("/collections/items", s.handleCollectionItemsPost)
	mux.HandleFunc("/collections/items/delete", s.handleCollectionItemsDelete)
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

	collections, err := s.db.Collections()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.render(w, "ayah.html", withCommonViewData(r, map[string]any{
		"Title":           fmt.Sprintf("Ayah %d:%d (%s)", surah, ayah, a.SurahName),
		"AyahRef":         relations.FormatAyahRef(surah, ayah),
		"Ayah":            a,
		"AyahTranslation": ayahTranslation,
		"Related":         related,
		"SurahName":       a.SurahName,
		"Collections":     collections,
		"SaveStatus":      collectionStatusMessage(r.URL.Query().Get("saved")),
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	diff1, diff2 := diffHighlight(ayah1.TextAR, ayah2.TextAR)
	s.render(w, "compare.html", withCommonViewData(r, map[string]any{
		"Title":            "Compare",
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
	}))
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	recentItems, err := s.db.RecentCollectionItems(12)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
			CreatedAt:     item.CreatedAt,
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

	s.render(w, "dashboard.html", withCommonViewData(r, map[string]any{
		"Title":             "Dashboard",
		"RecentCollections": collections,
		"RecentItems":       items,
		"ResumeAyahURL":     resumeAyahURL,
		"ResumePairURL":     resumeRelationURL,
	}))
}

func (s *server) handleCollectionsPage(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		collections, err := s.db.Collections()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.render(w, "collections.html", withCommonViewData(r, map[string]any{
			"Title":        "Collections",
			"Collections":  collections,
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
			s.render(w, "collections.html", withCommonViewData(r, map[string]any{
				"Title":           "Collections",
				"Collections":     collections,
				"StatusNotice":    "",
				"CollectionError": "Collection name is required.",
				"FormName":        name,
				"FormDescription": description,
			}))
			return
		}

		id, err := s.db.CreateCollection(name, description)
		if err != nil {
			collections, _ := s.db.Collections()
			s.render(w, "collections.html", withCommonViewData(r, map[string]any{
				"Title":           "Collections",
				"Collections":     collections,
				"CollectionError": "Collection name already exists or is invalid.",
				"FormName":        name,
				"FormDescription": description,
			}))
			return
		}
		http.Redirect(w, r, withLang(fmt.Sprintf("/collections/%d?status=created", id), lang), http.StatusSeeOther)
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
				CreatedAt:     item.CreatedAt,
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
			CreatedAt:     item.CreatedAt,
		})
	}

	s.render(w, "collection-detail.html", withCommonViewData(r, map[string]any{
		"Title":        collection.Name,
		"Collection":   collection,
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	returnTo := strings.TrimSpace(r.FormValue("return_to"))
	if returnTo == "" {
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
	data["DashboardURL"] = withLang("/dashboard", lang)
	data["CollectionsURL"] = withLang("/collections", lang)
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

func (s *server) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	if s == nil || strings.TrimSpace(s.adminUser) == "" || strings.TrimSpace(s.adminPass) == "" {
		http.Error(w, "admin auth not configured (set HIFZLINK_ADMIN_USER and HIFZLINK_ADMIN_PASS)", http.StatusServiceUnavailable)
		return false
	}

	user, pass, ok := r.BasicAuth()
	if ok && user == s.adminUser && pass == s.adminPass {
		return true
	}

	w.Header().Set("WWW-Authenticate", `Basic realm="HifzLink Admin", charset="UTF-8"`)
	http.Error(w, "admin authentication required", http.StatusUnauthorized)
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
