package main

import (
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strings"

	"github.com/srmdn/hifzlink/internal/relations"
	"github.com/srmdn/hifzlink/internal/search"
)

const maxMultiCompareTargets = 100

type multiCompareVerse struct {
	Number      int
	Ref         string
	SurahName   string
	Juz         int
	DiffText    template.HTML
	Translation string
	Note        string
}

func (s *server) handleMultiComparePage(w http.ResponseWriter, r *http.Request, rootRef string, targetRefs []string) {
	if len(targetRefs) > maxMultiCompareTargets {
		http.Error(w, "too many ayah2 references", http.StatusBadRequest)
		return
	}
	rootSurah, rootAyah, err := relations.ParseAyahRef(rootRef)
	if err != nil {
		http.Error(w, "invalid ayah1", http.StatusBadRequest)
		return
	}
	root, ok := s.quran.Get(rootSurah, rootAyah)
	if !ok {
		s.renderNotFound(w, r, "Ayah not found", "The ayah reference in this comparison does not exist in the dataset.")
		return
	}

	type target struct {
		ayah   search.Ayah
		surah  int
		number int
		ref    string
	}
	targets := make([]target, 0, len(targetRefs))
	seen := map[string]bool{fmt.Sprintf("%d:%d", rootSurah, rootAyah): true}
	for _, raw := range targetRefs {
		surah, number, parseErr := relations.ParseAyahRef(strings.TrimSpace(raw))
		if parseErr != nil {
			http.Error(w, "invalid ayah2", http.StatusBadRequest)
			return
		}
		key := fmt.Sprintf("%d:%d", surah, number)
		if seen[key] {
			continue
		}
		verse, exists := s.quran.Get(surah, number)
		if !exists {
			s.renderNotFound(w, r, "Ayah not found", "One or more ayah references in this comparison do not exist in the dataset.")
			return
		}
		seen[key] = true
		targets = append(targets, target{ayah: verse, surah: surah, number: number, ref: relations.FormatAyahRef(surah, number)})
	}
	if len(targets) == 0 {
		http.Error(w, "at least one distinct ayah2 is required", http.StatusBadRequest)
		return
	}

	rootIndices := map[int]bool{}
	peers := make([]multiCompareVerse, 0, len(targets))
	lang := pageLang(r)
	for targetIndex, item := range targets {
		rel, found, relErr := s.db.ByPair(rootSurah, rootAyah, item.surah, item.number)
		if relErr != nil {
			internalError(w, r, relErr)
			return
		}
		if !found {
			http.Error(w, "ayah2 is not connected to ayah1", http.StatusBadRequest)
			return
		}
		var peerDiff template.HTML
		var note string
		if found {
			note = rel.Note
		}
		if found && rel.Highlights != "" {
			highlights := parseHighlights(rel.Highlights)
			rootIsAyah1 := rel.Ayah1Surah == rootSurah && rel.Ayah1Ayah == rootAyah
			rootWords, peerWords := highlights.Ayah2, highlights.Ayah1
			if rootIsAyah1 {
				rootWords, peerWords = highlights.Ayah1, highlights.Ayah2
			}
			for _, index := range rootWords {
				rootIndices[index] = true
			}
			peerDiff = applyHighlights(item.ayah.TextAR, peerWords)
		} else {
			_, peerDiff = diffHighlight(root.TextAR, item.ayah.TextAR)
			rootWords, _ := unmatchedWordIndices(root.TextAR, item.ayah.TextAR)
			for _, index := range rootWords {
				rootIndices[index] = true
			}
		}
		peers = append(peers, multiCompareVerse{
			Number: targetIndex + 1, Ref: item.ref, SurahName: item.ayah.SurahName, Juz: item.ayah.Juz,
			DiffText: peerDiff, Translation: s.translationFor(lang, item.surah, item.number), Note: note,
		})
	}

	indices := make([]int, 0, len(rootIndices))
	for index := range rootIndices {
		indices = append(indices, index)
	}
	rootRef = relations.FormatAyahRef(rootSurah, rootAyah)
	values := url.Values{}
	values.Set("ayah1", rootRef)
	for _, item := range targets {
		values.Add("ayah2", item.ref)
	}
	sharePath := "/compare?" + values.Encode()
	title := fmt.Sprintf("Compare %s and %d more verses | hifzlink", rootRef, len(peers))
	heading := "Compare connected verses"
	lead := "Read the anchor ayah once, then scan each connected verse for wording differences."
	if lang == "id" {
		title = fmt.Sprintf("Bandingkan %s dengan %d ayat terkait | hifzlink", rootRef, len(peers))
		heading = "Bandingkan ayat-ayat terkait"
		lead = "Baca ayat acuan sekali, lalu amati perbedaan lafaz pada setiap ayat yang terhubung."
	}
	data := s.learningData(r, map[string]any{
		"Title": title, "Description": lead, "MultiHeading": heading, "MultiLead": lead,
		"Root":  multiCompareVerse{Ref: rootRef, SurahName: root.SurahName, Juz: root.Juz, DiffText: applyHighlights(root.TextAR, indices), Translation: s.translationFor(lang, rootSurah, rootAyah)},
		"Peers": peers, "ShareURL": withLang(sharePath, lang), "CanonicalURL": "https://" + r.Host + withLang(sharePath, lang),
		"JSONLD": breadcrumbJSONLD("https://"+r.Host, []breadcrumbItem{{Name: "Home", Path: withLang("/", lang)}, {Name: heading, Path: withLang(sharePath, lang)}}),
	})
	data["MultiBackURL"] = withLang(fmt.Sprintf("/ayah/%d/%d", rootSurah, rootAyah), lang)
	data["OGImageAlt"] = "HifzLink social card about reviewing similar Quran verses"
	s.render(w, "compare-multi.html", data)
}

func unmatchedWordIndices(a, b string) ([]int, []int) {
	return suggestedHighlightIndices(a, b)
}
