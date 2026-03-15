package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

type ayah struct {
	Surah     int    `json:"surah"`
	SurahName string `json:"surah_name"`
	Ayah      int    `json:"ayah"`
	Juz       int    `json:"juz"`
	TextAR    string `json:"text_ar"`
}

func main() {
	path := "data/quran.json"
	if len(os.Args) > 1 {
		path = os.Args[1]
	}

	if err := run(path); err != nil {
		fmt.Fprintf(os.Stderr, "validation failed: %v\n", err)
		os.Exit(1)
	}
}

func run(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	var ayahs []ayah
	if err := json.Unmarshal(b, &ayahs); err != nil {
		return fmt.Errorf("decode json: %w", err)
	}

	if len(ayahs) != 6236 {
		return fmt.Errorf("expected 6236 ayahs, got %d", len(ayahs))
	}

	seen := make(map[string]struct{}, len(ayahs))
	byJuz := make(map[int]int)
	for _, a := range ayahs {
		if a.Surah < 1 || a.Surah > 114 {
			return fmt.Errorf("invalid surah: %d", a.Surah)
		}
		if a.Ayah < 1 {
			return fmt.Errorf("invalid ayah: %d", a.Ayah)
		}
		if a.Juz < 1 || a.Juz > 30 {
			return fmt.Errorf("invalid juz %d for %d:%d", a.Juz, a.Surah, a.Ayah)
		}
		if strings.TrimSpace(a.SurahName) == "" {
			return fmt.Errorf("missing surah_name for %d:%d", a.Surah, a.Ayah)
		}
		if strings.TrimSpace(a.TextAR) == "" {
			return fmt.Errorf("empty text_ar for %d:%d", a.Surah, a.Ayah)
		}
		k := fmt.Sprintf("%d:%d", a.Surah, a.Ayah)
		if _, ok := seen[k]; ok {
			return fmt.Errorf("duplicate key: %s", k)
		}
		seen[k] = struct{}{}
		byJuz[a.Juz]++
	}

	missingJuz := make([]int, 0)
	for i := 1; i <= 30; i++ {
		if byJuz[i] == 0 {
			missingJuz = append(missingJuz, i)
		}
	}
	if len(missingJuz) > 0 {
		return fmt.Errorf("missing ayahs in juz values: %v", missingJuz)
	}

	juzValues := make([]int, 0, len(byJuz))
	for k := range byJuz {
		juzValues = append(juzValues, k)
	}
	sort.Ints(juzValues)

	fmt.Printf("OK: %d ayahs validated in %s\n", len(ayahs), path)
	fmt.Printf("Juz coverage: %d..%d\n", juzValues[0], juzValues[len(juzValues)-1])
	return nil
}
