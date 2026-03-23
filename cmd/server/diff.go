package main

import (
	"encoding/json"
	"html"
	"html/template"
	"strconv"
	"strings"
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
func buildHighlightsJSON(ayah1Indices, ayah2Indices string) string {
	h := HighlightData{
		Ayah1: parseIndexList(ayah1Indices),
		Ayah2: parseIndexList(ayah2Indices),
	}
	if len(h.Ayah1) == 0 && len(h.Ayah2) == 0 {
		return ""
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
