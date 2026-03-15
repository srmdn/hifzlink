package search

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadTranslations_MapFormat(t *testing.T) {
	dir := t.TempDir()
	enData := `{"1:1": "In the name of Allah", "60:8": "Allah does not forbid you"}`
	if err := os.WriteFile(filepath.Join(dir, "en.json"), []byte(enData), 0644); err != nil {
		t.Fatal(err)
	}

	store, err := LoadTranslations(dir, "en")
	if err != nil {
		t.Fatalf("LoadTranslations: %v", err)
	}

	text, ok := store.Get("en", 1, 1)
	if !ok || text != "In the name of Allah" {
		t.Errorf("unexpected: %q %v", text, ok)
	}

	text, ok = store.Get("en", 60, 8)
	if !ok || text != "Allah does not forbid you" {
		t.Errorf("unexpected: %q %v", text, ok)
	}
}

func TestLoadTranslations_ArrayFormat(t *testing.T) {
	dir := t.TempDir()
	enData := `[{"key":"1:1","text":"In the name of Allah"},{"key":"60:8","text":"Allah does not forbid you"}]`
	if err := os.WriteFile(filepath.Join(dir, "en.json"), []byte(enData), 0644); err != nil {
		t.Fatal(err)
	}

	store, err := LoadTranslations(dir, "en")
	if err != nil {
		t.Fatalf("LoadTranslations: %v", err)
	}

	text, ok := store.Get("en", 60, 8)
	if !ok || text != "Allah does not forbid you" {
		t.Errorf("unexpected: %q %v", text, ok)
	}
}

func TestLoadTranslations_MissingFile(t *testing.T) {
	dir := t.TempDir()

	store, err := LoadTranslations(dir, "en", "id")
	if err != nil {
		t.Fatalf("LoadTranslations should not fail on missing files: %v", err)
	}

	text, ok := store.Get("en", 1, 1)
	if ok || text != "" {
		t.Error("expected empty result for missing translation file")
	}
}

func TestTranslationStore_Get_LangAR(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "en.json"), []byte(`{"1:1":"In the name of Allah"}`), 0644)
	store, _ := LoadTranslations(dir, "en")

	text, ok := store.Get("ar", 1, 1)
	if ok || text != "" {
		t.Error("expected empty result for lang=ar")
	}
}

func TestTranslationStore_Get_EmptyLang(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "en.json"), []byte(`{"1:1":"In the name of Allah"}`), 0644)
	store, _ := LoadTranslations(dir, "en")

	text, ok := store.Get("", 1, 1)
	if ok || text != "" {
		t.Error("expected empty result for empty lang")
	}
}

func TestTranslationStore_Get_NilStore(t *testing.T) {
	var store *TranslationStore

	text, ok := store.Get("en", 1, 1)
	if ok || text != "" {
		t.Error("nil store should return empty without panic")
	}
}

func TestTranslationStore_Get_MissingLang(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "en.json"), []byte(`{"1:1":"In the name of Allah"}`), 0644)
	store, _ := LoadTranslations(dir, "en")

	text, ok := store.Get("id", 1, 1)
	if ok || text != "" {
		t.Error("expected empty result for lang not loaded")
	}
}

func TestTranslationStore_Get_MissingKey(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "en.json"), []byte(`{"1:1":"In the name of Allah"}`), 0644)
	store, _ := LoadTranslations(dir, "en")

	text, ok := store.Get("en", 60, 8)
	if ok || text != "" {
		t.Error("expected empty result for missing key")
	}
}

func TestLoadTranslations_MultipleLanguages(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "en.json"), []byte(`{"1:1":"In the name of Allah"}`), 0644)
	os.WriteFile(filepath.Join(dir, "id.json"), []byte(`{"1:1":"Dengan nama Allah"}`), 0644)

	store, err := LoadTranslations(dir, "en", "id")
	if err != nil {
		t.Fatalf("LoadTranslations: %v", err)
	}

	en, okEN := store.Get("en", 1, 1)
	id, okID := store.Get("id", 1, 1)

	if !okEN || en != "In the name of Allah" {
		t.Errorf("en: %q %v", en, okEN)
	}
	if !okID || id != "Dengan nama Allah" {
		t.Errorf("id: %q %v", id, okID)
	}
}
