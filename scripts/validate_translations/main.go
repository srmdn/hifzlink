package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const expectedAyahCount = 6236

type quranAyah struct {
	Surah int `json:"surah"`
	Ayah  int `json:"ayah"`
}

type translationRow struct {
	Key  string `json:"key"`
	Text string `json:"text"`
}

func main() {
	quranPath := flag.String("quran", filepath.Join("data", "quran.json"), "path to quran dataset")
	translationDir := flag.String("dir", filepath.Join("data", "translations"), "translation directory")
	langs := flag.String("langs", "en,id", "comma-separated language codes")
	report := flag.Bool("report", false, "print coverage report details")
	flag.Parse()

	langList := parseLangs(*langs)
	if len(langList) == 0 {
		fmt.Fprintln(os.Stderr, "validation failed: no languages provided")
		os.Exit(1)
	}

	if err := run(*quranPath, *translationDir, langList, *report); err != nil {
		fmt.Fprintf(os.Stderr, "validation failed: %v\n", err)
		os.Exit(1)
	}
}

func run(quranPath, translationDir string, langs []string, report bool) error {
	ayahKeys, err := loadQuranAyahKeys(quranPath)
	if err != nil {
		return err
	}
	if len(ayahKeys) != expectedAyahCount {
		return fmt.Errorf("expected %d ayah keys in %s, got %d", expectedAyahCount, quranPath, len(ayahKeys))
	}

	for _, lang := range langs {
		path := filepath.Join(translationDir, lang+".json")
		entries, err := loadTranslationEntries(path)
		if err != nil {
			return fmt.Errorf("%s: %w", lang, err)
		}

		if err := validateTranslationEntries(lang, entries, ayahKeys, report); err != nil {
			return err
		}
	}

	fmt.Printf("OK: validated translations for languages: %s\n", strings.Join(langs, ", "))
	return nil
}

func parseLangs(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, p := range parts {
		lang := strings.ToLower(strings.TrimSpace(p))
		if lang == "" {
			continue
		}
		if _, ok := seen[lang]; ok {
			continue
		}
		seen[lang] = struct{}{}
		out = append(out, lang)
	}
	return out
}

func loadQuranAyahKeys(path string) (map[string]struct{}, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	var ayahs []quranAyah
	if err := json.Unmarshal(b, &ayahs); err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}

	keys := make(map[string]struct{}, len(ayahs))
	for _, a := range ayahs {
		if a.Surah < 1 || a.Surah > 114 {
			return nil, fmt.Errorf("invalid surah in %s: %d", path, a.Surah)
		}
		if a.Ayah < 1 {
			return nil, fmt.Errorf("invalid ayah in %s: %d", path, a.Ayah)
		}
		key := fmt.Sprintf("%d:%d", a.Surah, a.Ayah)
		if _, exists := keys[key]; exists {
			return nil, fmt.Errorf("duplicate ayah key in %s: %s", path, key)
		}
		keys[key] = struct{}{}
	}

	return keys, nil
}

func loadTranslationEntries(path string) (map[string]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	parsedMap := map[string]string{}
	if err := json.Unmarshal(b, &parsedMap); err == nil {
		return parsedMap, nil
	}

	var rows []translationRow
	if err := json.Unmarshal(b, &rows); err != nil {
		return nil, fmt.Errorf("decode %s: expected object map or [{key,text}] array: %w", path, err)
	}

	parsedRows := make(map[string]string, len(rows))
	for i, row := range rows {
		k := strings.TrimSpace(row.Key)
		if k == "" {
			return nil, fmt.Errorf("decode %s: row %d has empty key", path, i)
		}
		if _, exists := parsedRows[k]; exists {
			return nil, fmt.Errorf("decode %s: duplicate key %s", path, k)
		}
		parsedRows[k] = row.Text
	}
	return parsedRows, nil
}

func validateTranslationEntries(lang string, entries map[string]string, quranKeys map[string]struct{}, report bool) error {
	if len(entries) == 0 {
		return fmt.Errorf("%s: no translation entries found", lang)
	}

	invalidKeys := make([]string, 0)
	unknownKeys := make([]string, 0)
	emptyKeys := make([]string, 0)
	validCount := 0

	for key, text := range entries {
		surah, ayah, ok := parseAyahKey(key)
		if !ok {
			invalidKeys = append(invalidKeys, key)
			continue
		}
		if surah < 1 || surah > 114 || ayah < 1 {
			invalidKeys = append(invalidKeys, key)
			continue
		}
		if _, exists := quranKeys[key]; !exists {
			unknownKeys = append(unknownKeys, key)
			continue
		}
		if strings.TrimSpace(text) == "" {
			emptyKeys = append(emptyKeys, key)
			continue
		}
		validCount++
	}

	missing := make([]string, 0)
	for key := range quranKeys {
		text, ok := entries[key]
		if !ok || strings.TrimSpace(text) == "" {
			missing = append(missing, key)
		}
	}

	sort.Strings(invalidKeys)
	sort.Strings(unknownKeys)
	sort.Strings(emptyKeys)
	sort.Strings(missing)

	if report {
		fmt.Printf(
			"lang=%s total=%d valid=%d missing=%d invalid=%d unknown=%d empty=%d\n",
			lang,
			len(entries),
			validCount,
			len(missing),
			len(invalidKeys),
			len(unknownKeys),
			len(emptyKeys),
		)
		printSample("missing", missing)
		printSample("invalid", invalidKeys)
		printSample("unknown", unknownKeys)
		printSample("empty", emptyKeys)
	}

	if len(invalidKeys) > 0 {
		return fmt.Errorf("%s: found %d invalid key format entries (example: %s)", lang, len(invalidKeys), invalidKeys[0])
	}
	if len(unknownKeys) > 0 {
		return fmt.Errorf("%s: found %d unknown ayah keys not present in quran dataset (example: %s)", lang, len(unknownKeys), unknownKeys[0])
	}
	if len(emptyKeys) > 0 {
		return fmt.Errorf("%s: found %d empty translation strings (example: %s)", lang, len(emptyKeys), emptyKeys[0])
	}
	if len(missing) > 0 {
		return fmt.Errorf("%s: missing %d translations (example: %s)", lang, len(missing), missing[0])
	}

	return nil
}

func parseAyahKey(key string) (surah, ayah int, ok bool) {
	parts := strings.Split(key, ":")
	if len(parts) != 2 {
		return 0, 0, false
	}

	surah, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, false
	}
	ayah, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, false
	}
	return surah, ayah, true
}

func printSample(label string, entries []string) {
	const maxItems = 5
	if len(entries) == 0 {
		return
	}
	limit := len(entries)
	if limit > maxItems {
		limit = maxItems
	}
	fmt.Printf("  %s sample: %s\n", label, strings.Join(entries[:limit], ", "))
}

