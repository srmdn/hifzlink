package main

import (
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/srmdn/hifzlink/internal/curation"
	"github.com/srmdn/hifzlink/internal/db"
	"github.com/srmdn/hifzlink/internal/relations"
)

const curationPageSize = 25
const automaticCurationAdminNote = "Auto-curated by Codex; manual review pending."

type adminCurationCandidateView struct {
	ID                   int64
	Ayah1                string
	Ayah1Name            string
	Ayah1Text            template.HTML
	Ayah2                string
	Ayah2Name            string
	Ayah2Text            template.HTML
	SuggestedCategory    string
	SuggestedLabel       string
	ActiveLabel          string
	CoverageLabel        string
	Confidence           string
	BasisLabel           string
	Priority             string
	MatchType            string
	PhraseLen            int
	Jaccard              float64
	Ayah1Words           []string
	Ayah2Words           []string
	SuggestedHighlights1 string
	SuggestedHighlights2 string
	Category             string
	Note                 string
	InternalNote         string
	RelationID           int64
}

func (s *server) handleAdminCuration(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	switch r.Method {
	case http.MethodGet:
		s.renderAdminCuration(w, r, "")
	case http.MethodPost:
		r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
		if !s.validateCSRFToken(r, s.csrfTokenFor(s.adminToken)) {
			http.Error(w, "invalid CSRF token", http.StatusForbidden)
			return
		}
		decision := strings.TrimSpace(r.FormValue("decision"))
		if decision == "accept_all" {
			accepted, err := s.acceptAllPendingCuration()
			if err != nil {
				s.renderAdminCuration(w, r, "Bulk acceptance failed: "+err.Error())
				return
			}
			lang := sanitizeLang(r.FormValue("lang"))
			target := "/admin/curation?status=auto&result=bulk_accepted&count=" + strconv.Itoa(accepted)
			http.Redirect(w, r, withLang(target, lang), http.StatusSeeOther)
			return
		}
		id, err := strconv.ParseInt(strings.TrimSpace(r.FormValue("id")), 10, 64)
		if err != nil || id <= 0 {
			s.renderAdminCuration(w, r, "Invalid candidate ID.")
			return
		}
		if decision != "accept" && decision != "reject" && decision != "mark_reviewed" && decision != "save_review" {
			s.renderAdminCuration(w, r, "Choose a valid review action.")
			return
		}
		candidate, err := s.db.CurationCandidateByID(id)
		if err != nil {
			s.renderAdminCuration(w, r, "Candidate is no longer available.")
			return
		}
		manualReview := decision == "mark_reviewed" || decision == "save_review"
		if manualReview {
			if candidate.Status != "accepted" || candidate.RelationID == 0 || candidate.ManuallyReviewed {
				s.renderAdminCuration(w, r, "This pair is no longer awaiting manual review.")
				return
			}
		} else if candidate.Status != "pending" {
			s.renderAdminCuration(w, r, "Candidate is no longer pending.")
			return
		}
		if decision == "mark_reviewed" {
			if err := s.rels.SetReviewed(candidate.RelationID, true, s.adminUser); err != nil {
				s.renderAdminCuration(w, r, err.Error())
				return
			}
			s.redirectAfterManualCuration(w, r)
			return
		}
		category := ""
		if decision == "accept" || decision == "save_review" {
			category = strings.TrimSpace(r.FormValue("category"))
			valid := category == ""
			for _, option := range adminCategoryOptions() {
				if category == option.Key {
					valid = true
					break
				}
			}
			if !valid {
				s.renderAdminCuration(w, r, "Choose a valid category.")
				return
			}
		}
		highlights := ""
		if decision == "accept" || decision == "save_review" {
			a1, ok1 := s.quran.Get(candidate.Ayah1Surah, candidate.Ayah1Ayah)
			a2, ok2 := s.quran.Get(candidate.Ayah2Surah, candidate.Ayah2Ayah)
			if !ok1 || !ok2 {
				s.renderAdminCuration(w, r, "Candidate references an unavailable ayah.")
				return
			}
			indices1, err1 := validateHighlightIndexList(r.FormValue("highlights_ayah1"), len(strings.Fields(a1.TextAR)))
			indices2, err2 := validateHighlightIndexList(r.FormValue("highlights_ayah2"), len(strings.Fields(a2.TextAR)))
			if err1 != nil || err2 != nil {
				s.renderAdminCuration(w, r, "Highlight selection is invalid. Please adjust the words and try again.")
				return
			}
			highlights = buildHighlightsJSON(joinHighlightIndices(indices1), joinHighlightIndices(indices2))
		}
		note := strings.TrimSpace(r.FormValue("note"))
		if len(note) > 2000 {
			s.renderAdminCuration(w, r, "The note must be 2,000 characters or fewer.")
			return
		}
		if decision == "save_review" {
			if err := s.rels.UpdateByID(candidate.RelationID,
				relations.FormatAyahRef(candidate.Ayah1Surah, candidate.Ayah1Ayah),
				relations.FormatAyahRef(candidate.Ayah2Surah, candidate.Ayah2Ayah),
				note, category, highlights, s.adminUser); err != nil {
				s.renderAdminCuration(w, r, err.Error())
				return
			}
			s.redirectAfterManualCuration(w, r)
			return
		}
		if err := s.db.ReviewCurationCandidate(id, decision == "accept", category, note, highlights, s.adminUser); err != nil {
			s.renderAdminCuration(w, r, err.Error())
			return
		}
		lang := sanitizeLang(r.FormValue("lang"))
		result := "rejected"
		if decision == "accept" {
			result = "accepted"
		}
		http.Redirect(w, r, withLang("/admin/curation?status=pending&result="+result, lang), http.StatusSeeOther)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *server) redirectAfterManualCuration(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.FormValue("return_page"))
	if page < 1 {
		page = 1
	}
	target := fmt.Sprintf("/admin/curation?status=auto&result=reviewed&page=%d", page)
	http.Redirect(w, r, withLang(target, sanitizeLang(r.FormValue("lang"))), http.StatusSeeOther)
}

func (s *server) acceptAllPendingCuration() (int, error) {
	pending, _, _, err := s.db.CurationCandidateCounts()
	if err != nil {
		return 0, err
	}
	if pending == 0 {
		return 0, nil
	}
	candidates, err := s.db.CurationCandidates("pending", pending, 0)
	if err != nil {
		return 0, err
	}
	if len(candidates) != pending {
		return 0, fmt.Errorf("candidate count changed during bulk review; loaded %d of %d", len(candidates), pending)
	}

	validCategories := make(map[string]bool)
	for _, option := range adminCategoryOptions() {
		validCategories[option.Key] = true
	}
	items := make([]db.AutomaticCurationAcceptance, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.SuggestedCategory != "" && !validCategories[candidate.SuggestedCategory] {
			return 0, fmt.Errorf("candidate %d has an invalid suggested category", candidate.ID)
		}
		ayah1, ok1 := s.quran.Get(candidate.Ayah1Surah, candidate.Ayah1Ayah)
		ayah2, ok2 := s.quran.Get(candidate.Ayah2Surah, candidate.Ayah2Ayah)
		if !ok1 || !ok2 {
			return 0, fmt.Errorf("candidate %d references an unavailable ayah", candidate.ID)
		}
		analysis := curation.Analyze(ayah1.TextAR, ayah2.TextAR)
		highlights1, highlights2 := analysis.Highlights.Ayah1, analysis.Highlights.Ayah2
		items = append(items, db.AutomaticCurationAcceptance{
			ID:         candidate.ID,
			Category:   analysis.Category,
			Highlights: buildHighlightsJSON(joinHighlightIndices(highlights1), joinHighlightIndices(highlights2)),
		})
	}
	accepted, err := s.db.AcceptAllPendingCuration(items, automaticCurationAdminNote)
	if err != nil {
		return 0, err
	}
	return accepted, nil
}

func (s *server) renderAdminCuration(w http.ResponseWriter, r *http.Request, adminError string) {
	counts, err := s.db.CurationReviewCounts()
	if err != nil {
		internalError(w, r, err)
		return
	}
	status := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
	if status == "" && r.Method == http.MethodPost {
		status = strings.TrimSpace(r.FormValue("return_status"))
	}
	if status == "accepted" {
		status = "auto" // Earlier links pointed to one combined acceptance list.
	}
	if status == "" {
		switch {
		case counts.Auto > 0:
			status = "auto"
		case counts.Pending > 0:
			status = "pending"
		default:
			status = "manual"
		}
	}
	if status != "pending" && status != "auto" && status != "manual" && status != "rejected" {
		http.Error(w, "invalid candidate status", http.StatusBadRequest)
		return
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	total := map[string]int{"pending": counts.Pending, "auto": counts.Auto, "manual": counts.Manual, "rejected": counts.Rejected}[status]
	pageCount := (total + curationPageSize - 1) / curationPageSize
	if pageCount == 0 {
		pageCount = 1
	}
	if page > pageCount {
		page = pageCount
	}
	candidates, err := s.db.CurationCandidates(status, curationPageSize, (page-1)*curationPageSize)
	if err != nil {
		internalError(w, r, err)
		return
	}

	options := adminCategoryOptionsForLang(pageLang(r))
	labels := make(map[string]string, len(options))
	for _, option := range options {
		labels[option.Key] = option.Label
	}
	views := make([]adminCurationCandidateView, 0, len(candidates))
	for _, candidate := range candidates {
		ayah1, ok1 := s.quran.Get(candidate.Ayah1Surah, candidate.Ayah1Ayah)
		ayah2, ok2 := s.quran.Get(candidate.Ayah2Surah, candidate.Ayah2Ayah)
		if !ok1 || !ok2 {
			continue
		}
		analysis := curation.Analyze(ayah1.TextAR, ayah2.TextAR)
		highlights1, highlights2 := analysis.Highlights.Ayah1, analysis.Highlights.Ayah2
		category := analysis.Category
		if candidate.Status == "accepted" {
			saved := parseHighlights(candidate.Highlights)
			highlights1, highlights2 = saved.Ayah1, saved.Ayah2
			category = candidate.Category
		}
		views = append(views, adminCurationCandidateView{
			ID:        candidate.ID,
			Ayah1:     relations.FormatAyahRef(candidate.Ayah1Surah, candidate.Ayah1Ayah),
			Ayah1Name: ayah1.SurahName, Ayah1Text: applyHighlights(ayah1.TextAR, highlights1),
			Ayah2:     relations.FormatAyahRef(candidate.Ayah2Surah, candidate.Ayah2Ayah),
			Ayah2Name: ayah2.SurahName, Ayah2Text: applyHighlights(ayah2.TextAR, highlights2),
			ActiveLabel:       labels[category],
			CoverageLabel:     fmt.Sprintf("%.0f%% / %.0f%%", analysis.Coverage1*100, analysis.Coverage2*100),
			SuggestedCategory: analysis.Category,
			SuggestedLabel:    labels[analysis.Category],
			Confidence:        analysis.Confidence, BasisLabel: analysis.Basis,
			Priority: candidate.ReviewPriority, MatchType: candidate.MatchType,
			PhraseLen: analysis.PhraseLen, Jaccard: candidate.Jaccard,
			Ayah1Words: strings.Fields(ayah1.TextAR), Ayah2Words: strings.Fields(ayah2.TextAR),
			SuggestedHighlights1: joinHighlightIndices(highlights1),
			SuggestedHighlights2: joinHighlightIndices(highlights2),
			Category:             category, Note: candidate.Note,
			InternalNote: candidate.InternalNote, RelationID: candidate.RelationID,
		})
	}

	baseURL := "/admin/curation?status=" + url.QueryEscape(status)
	lang := pageLang(r)
	baseURL = withLang(baseURL, lang)
	data := map[string]any{
		"Title":            "Curation review",
		"NoIndex":          true,
		"AdminError":       adminError,
		"AdminCSRFToken":   s.csrfTokenFor(s.adminToken),
		"Candidates":       views,
		"CategoryOptions":  options,
		"CategoryLabelMap": labels,
		"Status":           status,
		"PendingCount":     counts.Pending,
		"AutoCount":        counts.Auto,
		"ManualCount":      counts.Manual,
		"RejectedCount":    counts.Rejected,
		"Total":            total,
		"Page":             page,
		"PageCount":        pageCount,
		"HasPrevious":      page > 1,
		"HasNext":          page < pageCount,
		"PreviousURL":      fmt.Sprintf("%s&page=%d", baseURL, page-1),
		"NextURL":          fmt.Sprintf("%s&page=%d", baseURL, page+1),
		"ResultNotice":     adminCurationResult(r.URL.Query().Get("result")),
		"Lang":             lang,
	}
	s.render(w, "admin-curation.html", s.learningData(r, data))
}

func curationBasisLabel(basis string) string {
	if strings.HasPrefix(basis, curation.Version+": ") {
		return strings.TrimPrefix(basis, curation.Version+": ")
	}
	switch basis {
	case "normalized_verses_identical":
		return "Normalized verse wording is identical"
	case "one_verse_adds_ordered_words":
		return "One verse contains the other sequence with extra words"
	case "same_words_different_order":
		return "Same words appear in a different order"
	case "shared_opening_different_later_words":
		return "Long shared opening, then wording diverges"
	case "same_length_few_word_changes":
		return "Same length with a few word changes"
	case "shared_phrase_covers_most_of_shorter_verse":
		return "Shared phrase covers most of the shorter verse"
	case "shared_exact_phrase":
		return "Contains a shared exact phrase of four or more words"
	case "high_word_overlap_without_clearer_pattern":
		return "High word overlap without a clearer pattern"
	default:
		return "Similarity heuristic"
	}
}

func adminCurationResult(result string) string {
	switch result {
	case "accepted":
		return "Candidate accepted and added to the public relations catalog."
	case "rejected":
		return "Candidate rejected."
	case "bulk_accepted":
		return "All pending candidates were added to the public relations catalog. Codex attribution is saved in the admin-only review note."
	case "reviewed":
		return "Pair moved to Manually reviewed. The admin-only note now records the reviewer and review time."
	default:
		return ""
	}
}
