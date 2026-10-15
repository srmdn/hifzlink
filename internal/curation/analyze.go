// Package curation provides provisional wording analysis, never Quran text edits.
package curation

import (
	"strings"
	"unicode"
)

const Version = "wording-v2"

type Analysis struct {
	Category   string     `json:"suggested_category"`
	Confidence string     `json:"category_suggestion_confidence"`
	Basis      string     `json:"category_suggestion_basis"`
	Version    string     `json:"rule_version"`
	Coverage1  float64    `json:"coverage1"`
	Coverage2  float64    `json:"coverage2"`
	PhraseLen  int        `json:"phrase_len"`
	Highlights Highlights `json:"highlights"`
}

type Highlights struct {
	Ayah1 []int `json:"ayah1"`
	Ayah2 []int `json:"ayah2"`
}

type token struct {
	key, reading string
	index        int
}

// Only stop/section signs are ignored for reading comparison. Harakat remain:
// stripping all combining marks would hide genuine vocalization differences.
func stopSign(r rune) bool {
	return r >= '\u06d6' && r <= '\u06dc' || r == '\u06dd' || r == '\u06de' || r == '\u06e9'
}

func tokens(text string) []token {
	var out []token
	for i, word := range strings.Fields(text) {
		var key, reading strings.Builder
		for _, r := range word {
			if stopSign(r) || r == '\u0640' {
				continue
			}
			reading.WriteRune(r)
			if unicode.Is(unicode.Mn, r) {
				continue
			}
			switch r {
			case 'أ', 'إ', 'آ', 'ٱ':
				r = 'ا'
			}
			key.WriteRune(r)
		}
		if key.Len() > 0 {
			out = append(out, token{key.String(), reading.String(), i})
		}
	}
	return out
}

// alignment aligns normalized words; the reading is compared after alignment.
func alignment(a, b []token) ([]int, []int) {
	dp := make([][]int, len(a)+1)
	for i := range dp {
		dp[i] = make([]int, len(b)+1)
	}
	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			if a[i-1].key == b[j-1].key {
				dp[i][j] = dp[i-1][j-1] + 1
			} else {
				dp[i][j] = max(dp[i-1][j], dp[i][j-1])
			}
		}
	}
	var x, y []int
	for i, j := len(a), len(b); i > 0 && j > 0; {
		if a[i-1].key == b[j-1].key {
			x = append(x, i-1)
			y = append(y, j-1)
			i--
			j--
		} else if dp[i-1][j] > dp[i][j-1] {
			i--
		} else {
			j--
		}
	}
	return x, y
}

func Analyze(text1, text2 string) Analysis {
	a, b := tokens(text1), tokens(text2)
	x, y := alignment(a, b)
	r := Analysis{Category: "other", Confidence: "tentative", Basis: "No clear wording pattern; manual review needed", Version: Version,
		Highlights: Highlights{Ayah1: []int{}, Ayah2: []int{}}}
	matched1, matched2 := make(map[int]bool), make(map[int]bool)
	for k, i := range x {
		j := y[k]
		if a[i].reading == b[j].reading {
			matched1[i] = true
			matched2[j] = true
		}
	}
	for i, t := range a {
		if !matched1[i] {
			r.Highlights.Ayah1 = append(r.Highlights.Ayah1, t.index)
		}
	}
	for i, t := range b {
		if !matched2[i] {
			r.Highlights.Ayah2 = append(r.Highlights.Ayah2, t.index)
		}
	}
	if len(a) == 0 || len(b) == 0 {
		return r
	}
	r.Coverage1 = float64(len(x)) / float64(len(a))
	r.Coverage2 = float64(len(x)) / float64(len(b))
	set := func(category, basis string) Analysis { r.Category = category; r.Basis = basis; return r }
	if len(x) == len(a) && len(x) == len(b) {
		r.PhraseLen = len(a)
		if len(r.Highlights.Ayah1) == 0 && len(r.Highlights.Ayah2) == 0 {
			r.Confidence = "strong"
			return set("lafzi", "Identical wording and vocalization; stop signs ignored")
		}
		return set("lafzi", "Same normalized words throughout; inspect highlighted vocalization or spelling differences")
	}
	// Find the longest contiguous phrase across both complete verses, without
	// the candidate detector's nine-word cap. Ties prefer an anchored phrase.
	bestI, bestJ := 0, 0
	for i := range a {
		for j := range b {
			n := 0
			for i+n < len(a) && j+n < len(b) && a[i+n].key == b[j+n].key {
				n++
			}
			anchored := i == 0 && j == 0 || i+n == len(a) && j+n == len(b)
			if n > r.PhraseLen || n == r.PhraseLen && anchored {
				r.PhraseLen = n
				bestI = i
				bestJ = j
			}
		}
	}
	// Specific whole-verse patterns precede partial phrase labels.
	if len(x) == min(len(a), len(b)) && r.Coverage1 >= 0.8 && r.Coverage2 >= 0.8 {
		return set("addition_omission", "One full word sequence is retained, with additions; at least 80% coverage of each verse")
	}
	if len(a) == len(b) {
		counts := make(map[string]int)
		for _, t := range a {
			counts[t.key]++
		}
		for _, t := range b {
			counts[t.key]--
		}
		same := true
		for _, n := range counts {
			if n != 0 {
				same = false
			}
		}
		if same {
			return set("order_change", "The complete word multisets match, in a different order")
		}
		differences := 0
		for i := range a {
			if a[i].key != b[i].key {
				differences++
			}
		}
		if differences <= 2 && differences*5 <= len(a) {
			return set("word_swap", "Equal verse lengths; at most two substitutions and at least 80% of positions identical")
		}
	}
	if r.Coverage1 >= 0.9 && r.Coverage2 >= 0.9 {
		return set("lafzi", "At least 90% of the words in each complete verse align in order")
	}
	prefix := 0
	for prefix < min(len(a), len(b)) && a[prefix].key == b[prefix].key {
		prefix++
	}
	if prefix >= 4 && prefix*5 >= len(a)*4 && prefix*5 >= len(b)*4 {
		return set("ending_variation", "An identical opening covers at least 80% of each verse; only the tail diverges")
	}
	if r.PhraseLen >= 4 {
		if bestI == 0 && bestJ == 0 {
			return set("shared_opening", "The longest shared phrase starts both verses; whole-verse similarity is insufficient")
		}
		if bestI+r.PhraseLen == len(a) && bestJ+r.PhraseLen == len(b) {
			return set("shared_ending", "The longest shared phrase ends both verses; other wording differs")
		}
		return set("shared_phrase", "A shared phrase occurs within or at different positions in the verses")
	}
	// Grammar and pronoun interpretation are not inferred from word overlap.
	return r
}

func DifferenceIndices(text1, text2 string) ([]int, []int) {
	a := Analyze(text1, text2)
	return a.Highlights.Ayah1, a.Highlights.Ayah2
}
