package curation

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestAnalyzeWholeVerseAndPhrasePriority(t *testing.T) {
	for _, tc := range []struct{ name, a, b, category string }{
		{"identical", "a b c d", "a b c d", "lafzi"},
		{"small addition", "a b c d", "a b x c d", "addition_omission"},
		{"long addition is partial", "a b c d", "x y z a b c d", "shared_ending"},
		{"substitution", "a b c d e", "a b x d e", "word_swap"},
		{"order", "a b c d e", "a c b d e", "order_change"},
		{"opening", "a b c d x y", "a b c d z w", "shared_opening"},
		{"ending", "x y a b c d", "z w a b c d", "shared_ending"},
		{"middle", "x a b c d y", "z a b c d w", "shared_phrase"},
		{"mixed positions", "a b c d x y", "z a b c d w", "shared_phrase"},
		{"no inferred grammar", "a b x", "a c y", "other"},
		{"empty", "", "a b", "other"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := Analyze(tc.a, tc.b)
			if a.Category != tc.category {
				t.Fatalf("category=%s, want %s", a.Category, tc.category)
			}
			if a.Version != Version {
				t.Fatalf("missing rules version")
			}
		})
	}
}

func TestStopSignsRetainOriginalIndicesAndVocalization(t *testing.T) {
	a := Analyze("عَلِمَ ۖ زَيْدٌ", "عُلِمَ زَيْدٌ")
	if !reflect.DeepEqual(a.Highlights.Ayah1, []int{0}) || !reflect.DeepEqual(a.Highlights.Ayah2, []int{0}) {
		t.Fatalf("vocalization difference lost or stop sign highlighted: %+v", a.Highlights)
	}
	a = Analyze("عَلِمَ ۖ زَيْدٌ", "عَلِمَ عَمْرٌو")
	if !reflect.DeepEqual(a.Highlights.Ayah1, []int{2}) || !reflect.DeepEqual(a.Highlights.Ayah2, []int{1}) {
		t.Fatalf("original token indices lost: %+v", a.Highlights)
	}
}

func TestKnownQuranPairsAndVersionedCandidates(t *testing.T) {
	data, err := os.ReadFile("../../data/quran.json")
	if err != nil {
		t.Fatal(err)
	}
	var verses []struct {
		Surah int    `json:"surah"`
		Ayah  int    `json:"ayah"`
		Text  string `json:"text_ar"`
	}
	if err = json.Unmarshal(data, &verses); err != nil {
		t.Fatal(err)
	}
	texts := map[[2]int]string{}
	for _, v := range verses {
		texts[[2]int{v.Surah, v.Ayah}] = v.Text
	}
	a := Analyze(texts[[2]int{2, 114}], texts[[2]int{5, 41}])
	if a.Category != "shared_ending" || a.Coverage1 >= 0.9 || a.Coverage2 >= 0.9 {
		t.Fatalf("partial ending misclassified: %+v", a)
	}
	a = Analyze(texts[[2]int{2, 162}], texts[[2]int{3, 88}])
	if a.Category != "lafzi" || a.Confidence != "strong" || len(a.Highlights.Ayah1)+len(a.Highlights.Ayah2) != 0 {
		t.Fatalf("stop-sign-only pair differs: %+v", a)
	}
	data, err = os.ReadFile("../../data/curation_candidates.json")
	if err != nil {
		t.Fatal(err)
	}
	var candidates []struct {
		Ayah1    string `json:"ayah1"`
		Ayah2    string `json:"ayah2"`
		Category string `json:"suggested_category"`
	}
	if err = json.Unmarshal(data, &candidates); err != nil {
		t.Fatal(err)
	}
	for _, c := range candidates {
		parse := func(ref string) [2]int {
			var x [2]int
			parts := strings.Split(ref, ":")
			if len(parts) != 2 {
				t.Fatal(ref)
			}
			for i, p := range parts {
				for _, digit := range p {
					x[i] = x[i]*10 + int(digit-'0')
				}
			}
			return x
		}
		x, y := texts[parse(c.Ayah1)], texts[parse(c.Ayah2)]
		if x == "" || y == "" {
			t.Fatalf("unknown pair %s/%s", c.Ayah1, c.Ayah2)
		}
		a = Analyze(x, y)
		if a.Category != c.Category {
			t.Fatalf("stale category %s/%s: got %s, stored %s", c.Ayah1, c.Ayah2, a.Category, c.Category)
		}
		for side, indices := range [][]int{a.Highlights.Ayah1, a.Highlights.Ayah2} {
			words := strings.Fields([]string{x, y}[side])
			prev := -1
			for _, i := range indices {
				if i <= prev || i >= len(words) || len(tokens(words[i])) == 0 {
					t.Fatalf("invalid original word index %d", i)
				}
				prev = i
			}
		}
	}
}
