package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const expectedAyahCount = 6236
const englishTafsirResourceID = 169

var httpClient = &http.Client{Timeout: 60 * time.Second}

var indonesianSurahURLTemplate = "https://raw.githubusercontent.com/rioastamal/quran-json/master/surah/%d.json"
var quranChapterURLTemplate = "https://quran.com/%d"
var quranNextDataVerseURLTemplate = "https://quran.com/_next/data/%s/%d:%d.json"
var quranTafsirByChapterURLTemplate = "https://api.quran.com/api/v4/tafsirs/%d/by_chapter/%d?per_page=300"

type ayahRef struct {
	Surah int
	Ayah  int
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "import translations failed: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	translationDir := filepath.Join("data", "translations")
	if err := os.MkdirAll(translationDir, 0o755); err != nil {
		return err
	}
	tafsirDir := filepath.Join("data", "tafsir")
	if err := os.MkdirAll(tafsirDir, 0o755); err != nil {
		return err
	}

	en, err := importEnglishFromQuranCom()
	if err != nil {
		return err
	}
	if len(en) != expectedAyahCount {
		return fmt.Errorf("expected %d English entries, got %d", expectedAyahCount, len(en))
	}
	enPath := filepath.Join(translationDir, "en.json")
	if err := writeJSON(enPath, en); err != nil {
		return fmt.Errorf("write %s: %w", enPath, err)
	}
	fmt.Printf("Imported %d en translations into %s\n", len(en), enPath)

	idTranslations, idTafsir, err := importIndonesianFromQuranJSON(indonesianSurahURLTemplate)
	if err != nil {
		return err
	}
	if len(idTranslations) != expectedAyahCount {
		return fmt.Errorf("expected %d Indonesian entries, got %d", expectedAyahCount, len(idTranslations))
	}
	idPath := filepath.Join(translationDir, "id.json")
	if err := writeJSON(idPath, idTranslations); err != nil {
		return fmt.Errorf("write %s: %w", idPath, err)
	}
	fmt.Printf("Imported %d id translations into %s\n", len(idTranslations), idPath)

	if len(idTafsir) != expectedAyahCount {
		return fmt.Errorf("expected %d Indonesian tafsir entries, got %d", expectedAyahCount, len(idTafsir))
	}
	idTafsirPath := filepath.Join(tafsirDir, "id.kemenag.json")
	if err := writeJSON(idTafsirPath, idTafsir); err != nil {
		return fmt.Errorf("write %s: %w", idTafsirPath, err)
	}
	fmt.Printf("Imported %d id tafsir entries into %s\n", len(idTafsir), idTafsirPath)

	enTafsir, err := importEnglishTafsirFromQuranAPI(englishTafsirResourceID)
	if err != nil {
		return err
	}
	if len(enTafsir) != expectedAyahCount {
		return fmt.Errorf("expected %d English tafsir entries, got %d", expectedAyahCount, len(enTafsir))
	}
	enTafsirPath := filepath.Join(tafsirDir, "en.ibn-kathir.json")
	if err := writeJSON(enTafsirPath, enTafsir); err != nil {
		return fmt.Errorf("write %s: %w", enTafsirPath, err)
	}
	fmt.Printf("Imported %d en tafsir entries into %s\n", len(enTafsir), enTafsirPath)

	return nil
}

func importEnglishFromQuranCom() (map[string]string, error) {
	buildID, err := fetchQuranComBuildID()
	if err != nil {
		return nil, err
	}
	build := &buildState{ID: buildID}

	translations := make(map[string]string, expectedAyahCount)
	remaining := make([]ayahRef, 0, expectedAyahCount)

	for surah := 1; surah <= 114; surah++ {
		body, err := fetchQuranComVerseJSON(build, surah, 1)
		if err != nil {
			return nil, fmt.Errorf("fetch en surah %d ayah 1: %w", surah, err)
		}

		key, text, totalPages, err := parseQuranComVerseTranslation(body, surah, 1)
		if err != nil {
			return nil, fmt.Errorf("parse en surah %d ayah 1: %w", surah, err)
		}
		translations[key] = text

		for ayah := 2; ayah <= totalPages; ayah++ {
			remaining = append(remaining, ayahRef{Surah: surah, Ayah: ayah})
		}
	}

	if err := fetchRemainingEnglishVerses(build, remaining, translations); err != nil {
		return nil, err
	}

	return translations, nil
}

func fetchRemainingEnglishVerses(build *buildState, remaining []ayahRef, out map[string]string) error {
	type result struct {
		Key  string
		Text string
		Err  error
	}

	workerCount := 12
	jobs := make(chan ayahRef)
	results := make(chan result)

	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				body, err := fetchQuranComVerseJSON(build, job.Surah, job.Ayah)
				if err != nil {
					results <- result{Err: fmt.Errorf("fetch en %d:%d: %w", job.Surah, job.Ayah, err)}
					continue
				}
				key, text, _, err := parseQuranComVerseTranslation(body, job.Surah, job.Ayah)
				if err != nil {
					results <- result{Err: fmt.Errorf("parse en %d:%d: %w", job.Surah, job.Ayah, err)}
					continue
				}
				results <- result{Key: key, Text: text}
			}
		}()
	}

	go func() {
		for _, r := range remaining {
			jobs <- r
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()

	var firstErr error
	for res := range results {
		if res.Err != nil {
			if firstErr == nil {
				firstErr = res.Err
			}
			continue
		}
		if _, exists := out[res.Key]; exists {
			if firstErr == nil {
				firstErr = fmt.Errorf("duplicate translation key: %s", res.Key)
			}
			continue
		}
		out[res.Key] = res.Text
	}

	return firstErr
}

func importEnglishTafsirFromQuranAPI(resourceID int) (map[string]string, error) {
	out := make(map[string]string, expectedAyahCount)
	for surah := 1; surah <= 114; surah++ {
		url := fmt.Sprintf(quranTafsirByChapterURLTemplate, resourceID, surah)
		body, err := fetch(url)
		if err != nil {
			return nil, fmt.Errorf("fetch en tafsir surah %d: %w", surah, err)
		}

		surahTafsir, err := parseQuranComChapterTafsir(body, surah)
		if err != nil {
			return nil, fmt.Errorf("parse en tafsir surah %d: %w", surah, err)
		}

		for key, text := range surahTafsir {
			if _, exists := out[key]; exists {
				return nil, fmt.Errorf("duplicate tafsir key: %s", key)
			}
			out[key] = text
		}
	}
	return out, nil
}

func fetchQuranComBuildID() (string, error) {
	body, err := fetch(fmt.Sprintf(quranChapterURLTemplate, 1))
	if err != nil {
		return "", fmt.Errorf("fetch quran.com page: %w", err)
	}

	re := regexp.MustCompile(`"buildId":"([^"]+)"`)
	matches := re.FindStringSubmatch(body)
	if len(matches) < 2 {
		return "", fmt.Errorf("quran.com build id not found")
	}
	return matches[1], nil
}

type buildState struct {
	Mu sync.RWMutex
	ID string
}

func (b *buildState) current() string {
	b.Mu.RLock()
	defer b.Mu.RUnlock()
	return b.ID
}

func (b *buildState) set(id string) {
	b.Mu.Lock()
	b.ID = id
	b.Mu.Unlock()
}

func fetchQuranComVerseJSON(build *buildState, surah, ayah int) (string, error) {
	try := func(buildID string) (string, int, error) {
		url := fmt.Sprintf(quranNextDataVerseURLTemplate, buildID, surah, ayah)
		return fetchWithStatus(url)
	}

	body, status, err := try(build.current())
	if err != nil {
		return "", err
	}
	if status == http.StatusOK {
		return body, nil
	}
	if status != http.StatusNotFound {
		return "", fmt.Errorf("unexpected status: %d", status)
	}

	latestBuildID, err := fetchQuranComBuildID()
	if err != nil {
		return "", fmt.Errorf("refresh build id: %w", err)
	}
	build.set(latestBuildID)

	body, status, err = try(latestBuildID)
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("unexpected status after build refresh: %d", status)
	}
	return body, nil
}

func importIndonesianFromQuranJSON(urlTemplate string) (map[string]string, map[string]string, error) {
	translations := make(map[string]string, expectedAyahCount)
	tafsir := make(map[string]string, expectedAyahCount)

	for surah := 1; surah <= 114; surah++ {
		url := fmt.Sprintf(urlTemplate, surah)
		body, err := fetch(url)
		if err != nil {
			return nil, nil, fmt.Errorf("fetch id surah %d: %w", surah, err)
		}

		surahTranslations, surahTafsir, err := parseQuranJSONSurah(body, surah)
		if err != nil {
			return nil, nil, fmt.Errorf("parse id surah %d: %w", surah, err)
		}

		for key, text := range surahTranslations {
			if _, exists := translations[key]; exists {
				return nil, nil, fmt.Errorf("duplicate translation key: %s", key)
			}
			translations[key] = text
		}
		for key, text := range surahTafsir {
			if _, exists := tafsir[key]; exists {
				return nil, nil, fmt.Errorf("duplicate tafsir key: %s", key)
			}
			tafsir[key] = text
		}
	}

	return translations, tafsir, nil
}

func fetch(url string) (string, error) {
	body, status, err := fetchWithStatus(url)
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("unexpected status: %d", status)
	}
	return body, nil
}

func fetchWithStatus(url string) (string, int, error) {
	var lastErr error
	for attempt := 1; attempt <= 5; attempt++ {
		resp, err := httpClient.Get(url)
		if err != nil {
			lastErr = err
		} else {
			b, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			if readErr != nil {
				lastErr = readErr
			} else if resp.StatusCode == http.StatusOK {
				return string(b), resp.StatusCode, nil
			} else if resp.StatusCode == http.StatusNotFound {
				return "", resp.StatusCode, nil
			} else {
				lastErr = fmt.Errorf("unexpected status: %s", resp.Status)
				// Retry only for transient server errors.
				if resp.StatusCode < 500 || resp.StatusCode > 599 {
					return "", resp.StatusCode, nil
				}
			}
		}

		if attempt < 5 {
			time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
		}
	}

	return "", 0, lastErr
}

type quranComVersePayload struct {
	PageProps struct {
		VersesResponse struct {
			Verses []struct {
				VerseKey     string `json:"verseKey"`
				Translations []struct {
					Text string `json:"text"`
				} `json:"translations"`
			} `json:"verses"`
			Pagination struct {
				TotalPages int `json:"totalPages"`
			} `json:"pagination"`
		} `json:"versesResponse"`
	} `json:"pageProps"`
}

func parseQuranComVerseTranslation(body string, expectedSurah, expectedAyah int) (string, string, int, error) {
	var payload quranComVersePayload
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		return "", "", 0, fmt.Errorf("decode json: %w", err)
	}

	verses := payload.PageProps.VersesResponse.Verses
	if len(verses) != 1 {
		return "", "", 0, fmt.Errorf("expected single-verse payload, got %d", len(verses))
	}

	verse := verses[0]
	prefix := fmt.Sprintf("%d:", expectedSurah)
	key := strings.TrimSpace(verse.VerseKey)
	if !strings.HasPrefix(key, prefix) {
		return "", "", 0, fmt.Errorf("unexpected verse key %q for surah %d", key, expectedSurah)
	}
	parts := strings.SplitN(key, ":", 2)
	if len(parts) != 2 {
		return "", "", 0, fmt.Errorf("invalid verse key format %q", key)
	}
	ayah, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", "", 0, fmt.Errorf("invalid ayah in key %q", key)
	}
	if ayah != expectedAyah {
		return "", "", 0, fmt.Errorf("expected ayah %d, got %d (%s)", expectedAyah, ayah, key)
	}

	if len(verse.Translations) == 0 {
		return "", "", 0, fmt.Errorf("missing translation text for %s", key)
	}
	text := strings.TrimSpace(verse.Translations[0].Text)
	if text == "" {
		return "", "", 0, fmt.Errorf("empty translation text for %s", key)
	}

	totalPages := payload.PageProps.VersesResponse.Pagination.TotalPages
	if totalPages < 1 {
		return "", "", 0, fmt.Errorf("invalid total pages %d for surah %d", totalPages, expectedSurah)
	}

	return key, text, totalPages, nil
}

type quranAPITafsirPayload struct {
	Tafsirs []struct {
		VerseKey string `json:"verse_key"`
		Text     string `json:"text"`
	} `json:"tafsirs"`
}

func parseQuranComChapterTafsir(body string, expectedSurah int) (map[string]string, error) {
	var payload quranAPITafsirPayload
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		return nil, fmt.Errorf("decode json: %w", err)
	}

	if len(payload.Tafsirs) == 0 {
		return nil, fmt.Errorf("empty tafsir list")
	}

	out := make(map[string]string, len(payload.Tafsirs))
	prefix := fmt.Sprintf("%d:", expectedSurah)
	for _, item := range payload.Tafsirs {
		key := strings.TrimSpace(item.VerseKey)
		if !strings.HasPrefix(key, prefix) {
			return nil, fmt.Errorf("unexpected verse key %q for surah %d", key, expectedSurah)
		}
		text := strings.TrimSpace(item.Text)
		if _, exists := out[key]; exists {
			return nil, fmt.Errorf("duplicate verse key %s", key)
		}
		out[key] = text
	}

	return out, nil
}

type quranJSONSurahFile struct {
	Translations struct {
		ID struct {
			Text map[string]string `json:"text"`
		} `json:"id"`
	} `json:"translations"`
	Tafsir struct {
		ID struct {
			Kemenag struct {
				Text map[string]string `json:"text"`
			} `json:"kemenag"`
		} `json:"id"`
	} `json:"tafsir"`
}

func parseQuranJSONSurah(body string, expectedSurah int) (map[string]string, map[string]string, error) {
	var file map[string]quranJSONSurahFile
	if err := json.Unmarshal([]byte(body), &file); err != nil {
		return nil, nil, fmt.Errorf("decode json: %w", err)
	}

	if len(file) != 1 {
		return nil, nil, fmt.Errorf("expected one surah object, got %d", len(file))
	}

	surahKey := strconv.Itoa(expectedSurah)
	surahData, ok := file[surahKey]
	if !ok {
		return nil, nil, fmt.Errorf("missing surah key %s", surahKey)
	}

	translations, err := rekeyAyahMap(expectedSurah, surahData.Translations.ID.Text)
	if err != nil {
		return nil, nil, fmt.Errorf("translations: %w", err)
	}

	tafsir, err := rekeyAyahMap(expectedSurah, surahData.Tafsir.ID.Kemenag.Text)
	if err != nil {
		return nil, nil, fmt.Errorf("tafsir: %w", err)
	}

	return translations, tafsir, nil
}

func rekeyAyahMap(surah int, source map[string]string) (map[string]string, error) {
	if len(source) == 0 {
		return nil, fmt.Errorf("empty ayah map")
	}

	out := make(map[string]string, len(source))
	ayahNums := make([]int, 0, len(source))
	for ayahKey, text := range source {
		ayah, err := strconv.Atoi(ayahKey)
		if err != nil {
			return nil, fmt.Errorf("invalid ayah key %q", ayahKey)
		}
		if ayah < 1 {
			return nil, fmt.Errorf("invalid ayah number %d", ayah)
		}
		normalized := strings.TrimSpace(text)
		if normalized == "" {
			return nil, fmt.Errorf("empty text for %d:%d", surah, ayah)
		}
		key := fmt.Sprintf("%d:%d", surah, ayah)
		out[key] = normalized
		ayahNums = append(ayahNums, ayah)
	}

	sort.Ints(ayahNums)
	for i := 1; i < len(ayahNums); i++ {
		if ayahNums[i] == ayahNums[i-1] {
			return nil, fmt.Errorf("duplicate ayah number %d", ayahNums[i])
		}
	}

	return out, nil
}

func writeJSON(path string, v any) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}
