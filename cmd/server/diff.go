package main

import (
	"encoding/json"
	"html"
	"html/template"
	"strconv"
	"strings"
	"unicode"
)

// HighlightData holds manually chosen word indices for each ayah in a pair.
type HighlightData struct {
	Ayah1 []int `json:"ayah1"`
	Ayah2 []int `json:"ayah2"`
}

// parseHighlights decodes a stored JSON highlights string. Returns zero-value on empty/invalid input.
func parseHighlights(raw string) HighlightData {
	if raw == "" {
		return HighlightData{}
	}
	var h HighlightData
	if err := json.Unmarshal([]byte(raw), &h); err != nil {
		return HighlightData{}
	}
	return h
}

// applyHighlights renders text with words at the given indices wrapped in <mark>.
func applyHighlights(text string, indices []int) template.HTML {
	words := strings.Fields(text)
	marked := make(map[int]bool, len(indices))
	for _, i := range indices {
		if i >= 0 && i < len(words) {
			marked[i] = true
		}
	}
	parts := make([]string, len(words))
	for i, w := range words {
		escaped := html.EscapeString(w)
		if marked[i] {
			parts[i] = `<mark class="word-unique">` + escaped + `</mark>`
		} else {
			parts[i] = escaped
		}
	}
	return template.HTML(strings.Join(parts, " "))
}

// buildHighlightsJSON converts comma-separated index strings from form inputs into a JSON highlights string.
// Always encodes — empty arrays mean "explicitly no highlights" (different from "" which means "not set").
func buildHighlightsJSON(ayah1Indices, ayah2Indices string) string {
	h := HighlightData{
		Ayah1: parseIndexList(ayah1Indices),
		Ayah2: parseIndexList(ayah2Indices),
	}
	b, err := json.Marshal(h)
	if err != nil {
		return ""
	}
	return string(b)
}

func parseIndexList(s string) []int {
	parts := strings.Split(strings.TrimSpace(s), ",")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		n, err := strconv.Atoi(p)
		if err == nil && n >= 0 {
			out = append(out, n)
		}
	}
	return out
}

func validateHighlightIndexList(raw string, wordCount int) ([]int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	indices := make([]int, 0, len(parts))
	previous := -1
	for _, part := range parts {
		value, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || value < 0 || value >= wordCount || value <= previous {
			return nil, strconv.ErrSyntax
		}
		indices = append(indices, value)
		previous = value
	}
	return indices, nil
}

func joinHighlightIndices(indices []int) string {
	parts := make([]string, len(indices))
	for i, index := range indices {
		parts[i] = strconv.Itoa(index)
	}
	return strings.Join(parts, ",")
}

// lcsIndices returns the indices in a and b that are aligned by the
// Longest Common Subsequence. Both slices are returned in ascending order.
func lcsIndices(a, b []string) ([]int, []int) {
	m, n := len(a), len(b)
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if a[i-1] == b[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else if dp[i-1][j] > dp[i][j-1] {
				dp[i][j] = dp[i-1][j]
			} else {
				dp[i][j] = dp[i][j-1]
			}
		}
	}

	var ai, bi []int
	i, j := m, n
	for i > 0 && j > 0 {
		if a[i-1] == b[j-1] {
			ai = append([]int{i - 1}, ai...)
			bi = append([]int{j - 1}, bi...)
			i--
			j--
		} else if dp[i-1][j] > dp[i][j-1] {
			i--
		} else {
			j--
		}
	}
	return ai, bi
}

// suggestedHighlightIndices marks words that do not align between two ayahs.
// Matching ignores Quranic combining marks, tatweel, and common alif variants;
// returned positions still address the original, untouched verse tokens.
func suggestedHighlightIndices(text1, text2 string) ([]int, []int) {
	words1, words2 := strings.Fields(text1), strings.Fields(text2)
	normalized1, normalized2 := make([]string, len(words1)), make([]string, len(words2))
	for i, word := range words1 {
		normalized1[i] = normalizeArabicWord(word)
	}
	for i, word := range words2 {
		normalized2[i] = normalizeArabicWord(word)
	}
	shared1, shared2 := lcsIndices(normalized1, normalized2)
	matched1, matched2 := make(map[int]bool, len(shared1)), make(map[int]bool, len(shared2))
	for _, i := range shared1 {
		matched1[i] = true
	}
	for _, i := range shared2 {
		matched2[i] = true
	}
	var highlight1, highlight2 []int
	for i := range words1 {
		if !matched1[i] {
			highlight1 = append(highlight1, i)
		}
	}
	for i := range words2 {
		if !matched2[i] {
			highlight2 = append(highlight2, i)
		}
	}
	return highlight1, highlight2
}

func normalizeArabicWord(word string) string {
	var b strings.Builder
	for _, r := range word {
		if r == '\u0640' || unicode.Is(unicode.Mn, r) {
			continue
		}
		switch r {
		case 'أ', 'إ', 'آ', 'ٱ':
			r = 'ا'
		}
		b.WriteRune(r)
	}
	return b.String()
}

// diffHighlight compares two Arabic texts word-by-word using LCS and returns
// safe HTML for each with shared words unmarked and unique words highlighted.
func diffHighlight(text1, text2 string) (template.HTML, template.HTML) {
	w1 := strings.Fields(text1)
	w2 := strings.Fields(text2)

	idx1, idx2 := lcsIndices(w1, w2)

	shared1 := make(map[int]bool, len(idx1))
	for _, i := range idx1 {
		shared1[i] = true
	}
	shared2 := make(map[int]bool, len(idx2))
	for _, i := range idx2 {
		shared2[i] = true
	}

	return renderHighlighted(w1, shared1), renderHighlighted(w2, shared2)
}

func renderHighlighted(words []string, shared map[int]bool) template.HTML {
	parts := make([]string, len(words))
	for i, w := range words {
		escaped := html.EscapeString(w)
		if shared[i] {
			parts[i] = escaped
		} else {
			parts[i] = `<mark class="word-unique">` + escaped + `</mark>`
		}
	}
	return template.HTML(strings.Join(parts, " "))
}
