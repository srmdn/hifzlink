package search

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type TranslationStore struct {
	byLang map[string]map[string]string
}

func LoadTranslations(dir string, langs ...string) (*TranslationStore, error) {
	store := &TranslationStore{byLang: map[string]map[string]string{}}
	for _, lang := range langs {
		lang = strings.ToLower(strings.TrimSpace(lang))
		if lang == "" {
			continue
		}

		path := filepath.Join(dir, lang+".json")
		b, err := os.ReadFile(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				store.byLang[lang] = map[string]string{}
				continue
			}
			return nil, fmt.Errorf("read translation %s: %w", lang, err)
		}

		parsed, err := parseTranslationJSON(b)
		if err != nil {
			return nil, fmt.Errorf("parse translation %s: %w", lang, err)
		}
		store.byLang[lang] = parsed
	}
	return store, nil
}

// LoadTranslationFiles loads a TranslationStore from explicit lang→filepath mappings.
// Useful when filenames don't match the lang key (e.g. tafsir files).
func LoadTranslationFiles(files map[string]string) (*TranslationStore, error) {
	store := &TranslationStore{byLang: map[string]map[string]string{}}
	for lang, path := range files {
		lang = strings.ToLower(strings.TrimSpace(lang))
		if lang == "" {
			continue
		}
		b, err := os.ReadFile(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				store.byLang[lang] = map[string]string{}
				continue
			}
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		parsed, err := parseTranslationJSON(b)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		store.byLang[lang] = parsed
	}
	return store, nil
}

func parseTranslationJSON(b []byte) (map[string]string, error) {
	var m map[string]string
	if err := json.Unmarshal(b, &m); err == nil {
		return m, nil
	}

	var rows []struct {
		Key  string `json:"key"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(b, &rows); err != nil {
		return nil, err
	}

	out := make(map[string]string, len(rows))
	for _, row := range rows {
		k := strings.TrimSpace(row.Key)
		if k == "" {
			continue
		}
		out[k] = strings.TrimSpace(row.Text)
	}
	return out, nil
}

func (t *TranslationStore) Get(lang string, surah, ayah int) (string, bool) {
	if t == nil {
		return "", false
	}
	lang = strings.ToLower(strings.TrimSpace(lang))
	if lang == "" || lang == "ar" {
		return "", false
	}
	langMap, ok := t.byLang[lang]
	if !ok {
		return "", false
	}
	text, ok := langMap[fmt.Sprintf("%d:%d", surah, ayah)]
	if !ok || strings.TrimSpace(text) == "" {
		return "", false
	}
	return text, true
}
