package main

import (
	"html"
	"html/template"
	"strings"
)

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
