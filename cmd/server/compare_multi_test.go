package main

import (
	"html/template"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/srmdn/hifzlink/internal/db"
)

func TestHandleMultiCompareRejectsTooManyTargets(t *testing.T) {
	s := &server{}
	targets := make([]string, maxMultiCompareTargets+1)
	for i := range targets {
		targets[i] = "60:9"
	}

	req := httptest.NewRequest("GET", "/compare", nil)
	response := httptest.NewRecorder()
	s.handleMultiComparePage(response, req, "60:8", targets)

	if response.Code != 400 {
		t.Fatalf("expected 400 for too many targets, got %d", response.Code)
	}
}

func TestHandleMultiCompareRejectsUnrelatedTarget(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/compare", nil)
	response := httptest.NewRecorder()
	s.handleMultiComparePage(response, req, "60:8", []string{"60:9"})

	if response.Code != 400 || !strings.Contains(response.Body.String(), "not connected") {
		t.Fatalf("expected unrelated target to be rejected, got status %d and body %q", response.Code, response.Body.String())
	}
}

func TestHandleMultiCompareRendersConnectedTarget(t *testing.T) {
	s := newTestServer(t)
	if _, err := s.db.Add(db.Relation{Ayah1Surah: 60, Ayah1Ayah: 8, Ayah2Surah: 60, Ayah2Ayah: 9, Source: "seed"}); err != nil {
		t.Fatal(err)
	}
	s.tmpl = template.Must(template.New("root").Parse(`{{define "compare-multi.html"}}{{.Root.Ref}}{{range .Peers}} {{.Ref}}{{end}}{{end}}`))

	req := httptest.NewRequest("GET", "/compare?ayah1=60%3A8&ayah2=60%3A9", nil)
	response := httptest.NewRecorder()
	s.handleMultiComparePage(response, req, "60:8", []string{"60:9"})

	if response.Code != 200 || !strings.Contains(response.Body.String(), "60:8 60:9") {
		t.Fatalf("expected connected target to render, got status %d and body %q", response.Code, response.Body.String())
	}
}
