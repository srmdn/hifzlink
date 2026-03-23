package main

import (
	"strings"
	"testing"
)

func TestDiffHighlight_sharedWordsUnmarked(t *testing.T) {
	h1, h2 := diffHighlight("كلمة مشتركة فريدة1", "كلمة مشتركة فريدة2")
	s1, s2 := string(h1), string(h2)

	// shared words must not be wrapped
	if strings.Contains(s1, `<mark`) && !strings.Contains(s1, "فريدة1") {
		t.Error("expected shared words to be plain text in h1")
	}
	if strings.Contains(s2, `<mark`) && !strings.Contains(s2, "فريدة2") {
		t.Error("expected shared words to be plain text in h2")
	}

	// unique words must be highlighted
	if !strings.Contains(s1, `<mark class="word-unique">فريدة1</mark>`) {
		t.Errorf("expected unique word highlighted in h1, got: %s", s1)
	}
	if !strings.Contains(s2, `<mark class="word-unique">فريدة2</mark>`) {
		t.Errorf("expected unique word highlighted in h2, got: %s", s2)
	}
}

func TestDiffHighlight_identicalTexts(t *testing.T) {
	text := "بِسْمِ اللَّهِ الرَّحْمَٰنِ الرَّحِيمِ"
	h1, h2 := diffHighlight(text, text)

	// no highlights when texts are identical
	if strings.Contains(string(h1), "<mark") {
		t.Error("expected no highlights for identical texts in h1")
	}
	if strings.Contains(string(h2), "<mark") {
		t.Error("expected no highlights for identical texts in h2")
	}
}

func TestDiffHighlight_completelyDifferent(t *testing.T) {
	h1, h2 := diffHighlight("الف باء تاء", "جيم دال راء")

	// all words unique — all should be highlighted
	if strings.Count(string(h1), "<mark") != 3 {
		t.Errorf("expected 3 highlights in h1, got: %s", h1)
	}
	if strings.Count(string(h2), "<mark") != 3 {
		t.Errorf("expected 3 highlights in h2, got: %s", h2)
	}
}

func TestLCSIndices_basic(t *testing.T) {
	a := []string{"a", "b", "c", "d"}
	b := []string{"a", "c", "d", "e"}
	ai, bi := lcsIndices(a, b)

	// LCS is ["a", "c", "d"] — indices 0,2,3 in a and 0,1,2 in b
	wantA := []int{0, 2, 3}
	wantB := []int{0, 1, 2}
	for i, v := range wantA {
		if ai[i] != v {
			t.Errorf("ai[%d] = %d, want %d", i, ai[i], v)
		}
	}
	for i, v := range wantB {
		if bi[i] != v {
			t.Errorf("bi[%d] = %d, want %d", i, bi[i], v)
		}
	}
}
