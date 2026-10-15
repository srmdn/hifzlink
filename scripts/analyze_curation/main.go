// Analyze existing pairs without changing the catalog or Quran text.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/srmdn/hifzlink/internal/curation"
	"github.com/srmdn/hifzlink/internal/relations"
	"github.com/srmdn/hifzlink/internal/search"
)

func main() {
	quranPath := flag.String("quran", "data/quran.json", "Unchanged reference Quran JSON")
	flag.Parse()
	quran, err := search.Load(*quranPath)
	must(err)
	var pairs []struct {
		Ayah1 string `json:"ayah1"`
		Ayah2 string `json:"ayah2"`
	}
	must(json.NewDecoder(os.Stdin).Decode(&pairs))
	type result struct {
		Ayah1 string `json:"ayah1"`
		Ayah2 string `json:"ayah2"`
		curation.Analysis
	}
	out := make([]result, 0, len(pairs))
	for _, p := range pairs {
		s1, a1, e := relations.ParseAyahRef(p.Ayah1)
		must(e)
		s2, a2, e := relations.ParseAyahRef(p.Ayah2)
		must(e)
		v1, ok1 := quran.Get(s1, a1)
		v2, ok2 := quran.Get(s2, a2)
		if !ok1 || !ok2 || s1 > s2 || s1 == s2 && a1 >= a2 {
			must(fmt.Errorf("invalid canonical pair %s/%s", p.Ayah1, p.Ayah2))
		}
		out = append(out, result{p.Ayah1, p.Ayah2, curation.Analyze(v1.TextAR, v2.TextAR)})
	}
	must(json.NewEncoder(os.Stdout).Encode(out))
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
