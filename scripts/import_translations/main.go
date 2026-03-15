package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

var translationSources = map[string]string{
	"en": "https://tanzil.net/trans/?transID=en.sahih&type=txt-2",
	"id": "https://tanzil.net/trans/?transID=id.indonesian&type=txt-2",
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "import translations failed: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	outDir := filepath.Join("data", "translations")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}

	for lang, url := range translationSources {
		body, err := fetch(url)
		if err != nil {
			return fmt.Errorf("fetch %s: %w", lang, err)
		}

		parsed, err := parseTranslationLines(body)
		if err != nil {
			return fmt.Errorf("parse %s: %w", lang, err)
		}

		if len(parsed) != 6236 {
			return fmt.Errorf("expected 6236 entries for %s, got %d", lang, len(parsed))
		}

		outPath := filepath.Join(outDir, lang+".json")
		if err := writeJSON(outPath, parsed); err != nil {
			return fmt.Errorf("write %s: %w", outPath, err)
		}

		fmt.Printf("Imported %d %s translations into %s\n", len(parsed), lang, outPath)
	}

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

func parseTranslationLines(body string) (map[string]string, error) {
	scanner := bufio.NewScanner(strings.NewReader(body))
	scanner.Buffer(make([]byte, 0, 64*1024), 2*1024*1024)

	out := make(map[string]string, 6236)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "|", 3)
		if len(parts) != 3 {
			return nil, fmt.Errorf("invalid line: %q", line)
		}

		surah, err := strconv.Atoi(parts[0])
		if err != nil {
			return nil, fmt.Errorf("invalid surah in line: %q", line)
		}
		ayah, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil, fmt.Errorf("invalid ayah in line: %q", line)
		}

		key := fmt.Sprintf("%d:%d", surah, ayah)
		text := strings.TrimSpace(parts[2])
		if text == "" {
			return nil, fmt.Errorf("empty translation text for %s", key)
		}
		out[key] = text
	}

	if err := scanner.Err(); err != nil {
		return nil, err
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
