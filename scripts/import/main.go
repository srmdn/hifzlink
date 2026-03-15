package main

import (
	"bufio"
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
)

const (
	textURL = "https://tanzil.net/pub/download/index.php?quranType=uthmani&outType=txt-2&agree=true&marks=true&sajdah=true&tatweel=true"
	metaURL = "https://tanzil.net/res/text/metadata/quran-data.js"
)

type ayah struct {
	Surah     int    `json:"surah"`
	SurahName string `json:"surah_name"`
	Ayah      int    `json:"ayah"`
	Juz       int    `json:"juz"`
	TextAR    string `json:"text_ar"`
}

type point struct {
	Surah int
	Ayah  int
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "import failed: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	metadata, err := fetch(metaURL)
	if err != nil {
		return fmt.Errorf("fetch metadata: %w", err)
	}

	surahNames, err := parseSurahNames(metadata)
	if err != nil {
		return fmt.Errorf("parse surah names: %w", err)
	}

	juzStarts, err := parseJuzStarts(metadata)
	if err != nil {
		return fmt.Errorf("parse juz starts: %w", err)
	}

	textData, err := fetch(textURL)
	if err != nil {
		return fmt.Errorf("fetch quran text: %w", err)
	}

	ayahs, err := parseAyahLines(textData, surahNames, juzStarts)
	if err != nil {
		return fmt.Errorf("parse ayah lines: %w", err)
	}

	if err := validate(ayahs); err != nil {
		return err
	}

	outPath := filepath.Join("data", "quran.json")
	if err := writeJSON(outPath, ayahs); err != nil {
		return fmt.Errorf("write dataset: %w", err)
	}

	fmt.Printf("Imported %d ayahs into %s\n", len(ayahs), outPath)
	return nil
}

func fetch(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status: %s", resp.Status)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(b), nil
}

func parseSurahNames(meta string) (map[int]string, error) {
	section, err := extractArray(meta, "QuranData.Sura")
	if err != nil {
		return nil, err
	}

	re := regexp.MustCompile(`(?m)^\s*\[(\d+),\s*\d+,\s*\d+,\s*\d+,\s*'[^']*',\s*"([^"]+)"`)
	matches := re.FindAllStringSubmatch(section, -1)
	if len(matches) < 114 {
		return nil, fmt.Errorf("expected 114 surah rows, got %d", len(matches))
	}

	names := make(map[int]string, 114)
	for idx, m := range matches {
		n := idx + 1
		names[n] = strings.TrimSpace(m[2])
	}
	return names, nil
}

func parseJuzStarts(meta string) ([]point, error) {
	section, err := extractArray(meta, "QuranData.Juz")
	if err != nil {
		return nil, err
	}

	re := regexp.MustCompile(`\[(\d+),\s*(\d+)\]`)
	matches := re.FindAllStringSubmatch(section, -1)
	if len(matches) < 31 {
		return nil, fmt.Errorf("expected >=31 juz points, got %d", len(matches))
	}

	out := make([]point, 0, len(matches))
	for _, m := range matches {
		s, _ := strconv.Atoi(m[1])
		a, _ := strconv.Atoi(m[2])
		out = append(out, point{Surah: s, Ayah: a})
	}

	return out, nil
}

func extractArray(meta, marker string) (string, error) {
	start := strings.Index(meta, marker)
	if start == -1 {
		return "", fmt.Errorf("marker not found: %s", marker)
	}
	from := strings.Index(meta[start:], "[")
	if from == -1 {
		return "", fmt.Errorf("array start not found for: %s", marker)
	}
	idx := start + from

	depth := 0
	for i := idx; i < len(meta); i++ {
		switch meta[i] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return meta[idx : i+1], nil
			}
		}
	}

	return "", fmt.Errorf("array end not found for: %s", marker)
}

func parseAyahLines(text string, names map[int]string, juzStarts []point) ([]ayah, error) {
	scanner := bufio.NewScanner(strings.NewReader(text))
	scanner.Buffer(make([]byte, 0, 64*1024), 2*1024*1024)

	out := make([]ayah, 0, 6236)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.Contains(line, "|") {
			continue
		}
		parts := strings.SplitN(line, "|", 3)
		if len(parts) != 3 {
			return nil, fmt.Errorf("invalid text line: %q", line)
		}

		surah, err := strconv.Atoi(parts[0])
		if err != nil {
			return nil, fmt.Errorf("invalid surah in line %q", line)
		}
		ayahNum, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil, fmt.Errorf("invalid ayah in line %q", line)
		}

		name := names[surah]
		if name == "" {
			return nil, fmt.Errorf("missing surah name for surah %d", surah)
		}

		out = append(out, ayah{
			Surah:     surah,
			SurahName: name,
			Ayah:      ayahNum,
			Juz:       determineJuz(surah, ayahNum, juzStarts),
			TextAR:    parts[2],
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Surah == out[j].Surah {
			return out[i].Ayah < out[j].Ayah
		}
		return out[i].Surah < out[j].Surah
	})

	return out, nil
}

func determineJuz(surah, ayah int, starts []point) int {
	for i := 0; i < len(starts)-1; i++ {
		if before(surah, ayah, starts[i+1].Surah, starts[i+1].Ayah) {
			return i + 1
		}
	}
	return 30
}

func before(s1, a1, s2, a2 int) bool {
	if s1 != s2 {
		return s1 < s2
	}
	return a1 < a2
}

func validate(ayahs []ayah) error {
	if len(ayahs) != 6236 {
		return fmt.Errorf("expected 6236 ayahs, got %d", len(ayahs))
	}

	seen := make(map[string]struct{}, len(ayahs))
	for _, a := range ayahs {
		if a.Surah < 1 || a.Surah > 114 {
			return fmt.Errorf("invalid surah: %d", a.Surah)
		}
		if a.Ayah < 1 {
			return fmt.Errorf("invalid ayah: %d", a.Ayah)
		}
		if a.Juz < 1 || a.Juz > 30 {
			return fmt.Errorf("invalid juz: %d for %d:%d", a.Juz, a.Surah, a.Ayah)
		}
		if strings.TrimSpace(a.TextAR) == "" {
			return fmt.Errorf("empty text_ar for %d:%d", a.Surah, a.Ayah)
		}

		k := fmt.Sprintf("%d:%d", a.Surah, a.Ayah)
		if _, ok := seen[k]; ok {
			return fmt.Errorf("duplicate ayah key: %s", k)
		}
		seen[k] = struct{}{}
	}

	return nil
}

func writeJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

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
