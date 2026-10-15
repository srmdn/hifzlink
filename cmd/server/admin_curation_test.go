package main

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/srmdn/hifzlink/internal/db"
	webassets "github.com/srmdn/hifzlink/web"
)

func TestAdminAutomaticCurationManualReview(t *testing.T) {
	for _, decision := range []string{"mark_reviewed", "save_review"} {
		t.Run(decision, func(t *testing.T) {
			s := newTestServer(t)
			staticFiles, err := embeddedStaticFiles()
			if err != nil {
				t.Fatal(err)
			}
			assets, err := newAssetVersions(staticFiles)
			if err != nil {
				t.Fatal(err)
			}
			s.tmpl, err = template.New("").Funcs(assetFuncs(assets)).ParseFS(webassets.Files, "templates/*.html")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.db.ImportCurationCandidates([]db.CurationCandidate{{
				Ayah1Surah: 1, Ayah1Ayah: 1, Ayah2Surah: 60, Ayah2Ayah: 8,
				SuggestedCategory: "lafzi", ReviewPriority: "high",
			}}); err != nil {
				t.Fatal(err)
			}
			pending, _ := s.db.CurationCandidates("pending", 10, 0)
			if len(pending) != 1 {
				t.Fatalf("pending candidates = %#v", pending)
			}
			const originalHighlights = `{"ayah1":[1],"ayah2":[2]}`
			if _, err := s.db.AcceptAllPendingCuration([]db.AutomaticCurationAcceptance{{
				ID: pending[0].ID, Category: "lafzi", Highlights: originalHighlights,
			}}, automaticCurationAdminNote); err != nil {
				t.Fatal(err)
			}
			// Earlier accepted-list URLs must now open the automatic review group.
			get := httptest.NewRequest(http.MethodGet, "/admin/curation?status=accepted&lang=en", nil)
			addAdminCookie(get, s.adminToken)
			page := httptest.NewRecorder()
			s.handleAdminCuration(page, get)
			for _, want := range []string{"Auto-curated · needs review (1)", "Manually reviewed (0)", automaticCurationAdminNote, `value="mark_reviewed"`, `name="highlights_ayah1" value="1"`} {
				if !strings.Contains(page.Body.String(), want) {
					t.Fatalf("automatic page missing %q (status %d)", want, page.Code)
				}
			}
			form := url.Values{
				"id": {strconv.FormatInt(pending[0].ID, 10)}, "decision": {decision},
				"category": {"word_swap"}, "note": {"Public explanation"},
				"highlights_ayah1": {"0"}, "highlights_ayah2": {"1"},
				"lang": {"en"}, "return_status": {"auto"}, "return_page": {"1"},
			}
			post := func() *httptest.ResponseRecorder {
				r := httptest.NewRequest(http.MethodPost, "/admin/curation", strings.NewReader(form.Encode()))
				r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				addAdminCookie(r, s.adminToken)
				response := httptest.NewRecorder()
				s.handleAdminCuration(response, r)
				return response
			}
			if response := post(); response.Code != http.StatusForbidden {
				t.Fatalf("review without CSRF token = %d", response.Code)
			}
			form.Set("csrf_token", s.csrfTokenFor(s.adminToken))
			response := post()
			if response.Code != http.StatusSeeOther || !strings.Contains(response.Header().Get("Location"), "status=auto&result=reviewed") {
				t.Fatalf("manual review = %d, location=%q, body=%s", response.Code, response.Header().Get("Location"), response.Body.String())
			}
			candidate, err := s.db.CurationCandidateByID(pending[0].ID)
			if err != nil || !candidate.ManuallyReviewed || candidate.ManuallyReviewedBy != s.adminUser ||
				candidate.ManuallyReviewedAt == "" || strings.Contains(candidate.InternalNote, "pending") ||
				!strings.Contains(candidate.InternalNote, "Auto-curated by Codex") || !strings.Contains(candidate.InternalNote, "Manually reviewed by admin (admin)") {
				t.Fatalf("manual review did not replace the private note: %#v, %v", candidate, err)
			}
			relation, found, err := s.db.ByID(candidate.RelationID)
			if err != nil || !found || relation.Source != "auto" || !relation.Reviewed {
				t.Fatalf("reviewed relation lost origin or review flag: %#v, %v", relation, err)
			}
			if decision == "mark_reviewed" && (relation.Category != "lafzi" || relation.Note != "" || relation.Highlights != originalHighlights) {
				t.Fatalf("quick review changed saved content: %#v", relation)
			}
			if decision == "save_review" && (relation.Category != "word_swap" || relation.Note != "Public explanation" || relation.Highlights != `{"ayah1":[0],"ayah2":[1]}`) {
				t.Fatalf("review corrections were not saved: %#v", relation)
			}
			counts, err := s.db.CurationReviewCounts()
			if err != nil || counts.Auto != 0 || counts.Manual != 1 {
				t.Fatalf("reviewed pair did not change groups: %#v, %v", counts, err)
			}
			manualRequest := httptest.NewRequest(http.MethodGet, "/admin/curation?status=manual&lang=en", nil)
			addAdminCookie(manualRequest, s.adminToken)
			manualPage := httptest.NewRecorder()
			s.handleAdminCuration(manualPage, manualRequest)
			if manualPage.Code != http.StatusOK || !strings.Contains(manualPage.Body.String(), candidate.InternalNote) || strings.Contains(manualPage.Body.String(), `value="mark_reviewed"`) {
				t.Fatalf("manual group failed to render updated private note: status=%d", manualPage.Code)
			}
			publicPage := httptest.NewRecorder()
			s.handleComparePage(publicPage, httptest.NewRequest(http.MethodGet, "/compare?ayah1=1:1&ayah2=60:8&lang=en", nil))
			if publicPage.Code != http.StatusOK || strings.Contains(publicPage.Body.String(), "Manually reviewed by admin") || strings.Contains(publicPage.Body.String(), "Auto-curated by Codex") {
				t.Fatalf("public compare leaked private review note: status=%d", publicPage.Code)
			}
			// A stale submission must not repeat or overwrite the human review.
			if repeated := post(); repeated.Code != http.StatusOK || !strings.Contains(repeated.Body.String(), "no longer awaiting manual review") {
				t.Fatalf("stale manual review was not rejected: %d", repeated.Code)
			}
		})
	}
}
