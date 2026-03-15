package search

import (
	"encoding/json"
	"os"
	"testing"
)

func writeTempQuranJSON(t *testing.T, ayahs []Ayah) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "quran*.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.NewEncoder(f).Encode(ayahs); err != nil {
		t.Fatal(err)
	}
	f.Close()
	return f.Name()
}

func testAyahs() []Ayah {
	return []Ayah{
		{Surah: 1, SurahName: "Al-Fatihah", Ayah: 1, Juz: 1, TextAR: "بِسْمِ اللَّهِ"},
		{Surah: 1, SurahName: "Al-Fatihah", Ayah: 2, Juz: 1, TextAR: "الْحَمْدُ لِلَّهِ"},
		{Surah: 2, SurahName: "Al-Baqarah", Ayah: 1, Juz: 1, TextAR: "الم"},
		{Surah: 60, SurahName: "Al-Mumtahanah", Ayah: 8, Juz: 28, TextAR: "لَا يَنْهَاكُمُ اللَّهُ"},
		{Surah: 60, SurahName: "Al-Mumtahanah", Ayah: 9, Juz: 28, TextAR: "إِنَّمَا يَنْهَاكُمُ اللَّهُ"},
	}
}

func TestLoad(t *testing.T) {
	store, err := Load(writeTempQuranJSON(t, testAyahs()))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if store == nil {
		t.Fatal("expected non-nil store")
	}
}

func TestLoad_InvalidPath(t *testing.T) {
	_, err := Load("/does/not/exist.json")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestStore_Get_Found(t *testing.T) {
	store, _ := Load(writeTempQuranJSON(t, testAyahs()))

	a, ok := store.Get(60, 8)
	if !ok {
		t.Fatal("expected to find 60:8")
	}
	if a.Surah != 60 || a.Ayah != 8 {
		t.Errorf("unexpected ayah: got %d:%d", a.Surah, a.Ayah)
	}
	if a.TextAR == "" {
		t.Error("expected non-empty TextAR")
	}
}

func TestStore_Get_NotFound(t *testing.T) {
	store, _ := Load(writeTempQuranJSON(t, testAyahs()))

	_, ok := store.Get(999, 1)
	if ok {
		t.Error("expected not found for non-existent ayah")
	}
}

func TestStore_BySurah(t *testing.T) {
	store, _ := Load(writeTempQuranJSON(t, testAyahs()))

	ayahs := store.BySurah(1)
	if len(ayahs) != 2 {
		t.Errorf("expected 2 ayahs for surah 1, got %d", len(ayahs))
	}
	if ayahs[0].Ayah > ayahs[1].Ayah {
		t.Error("expected ayahs sorted ascending by ayah number")
	}
}

func TestStore_BySurah_Empty(t *testing.T) {
	store, _ := Load(writeTempQuranJSON(t, testAyahs()))

	if got := store.BySurah(999); len(got) != 0 {
		t.Errorf("expected empty slice for unknown surah, got %d", len(got))
	}
}

func TestStore_ByJuz(t *testing.T) {
	store, _ := Load(writeTempQuranJSON(t, testAyahs()))

	ayahs := store.ByJuz(28)
	if len(ayahs) != 2 {
		t.Errorf("expected 2 ayahs for juz 28, got %d", len(ayahs))
	}
}

func TestStore_ByJuz_Empty(t *testing.T) {
	store, _ := Load(writeTempQuranJSON(t, testAyahs()))

	if got := store.ByJuz(99); len(got) != 0 {
		t.Errorf("expected empty slice for unknown juz, got %d", len(got))
	}
}

func TestStore_SurahName(t *testing.T) {
	store, _ := Load(writeTempQuranJSON(t, testAyahs()))

	if name := store.SurahName(1); name == "" {
		t.Error("expected non-empty surah name for surah 1")
	}
}

func TestStore_BySurah_ReturnsCopy(t *testing.T) {
	store, _ := Load(writeTempQuranJSON(t, testAyahs()))

	a1 := store.BySurah(1)
	a2 := store.BySurah(1)
	a1[0].TextAR = "mutated"
	if a2[0].TextAR == "mutated" {
		t.Error("BySurah should return an independent copy")
	}
}

func TestLoad_SurahNameFallback(t *testing.T) {
	ayahs := []Ayah{
		{Surah: 1, Ayah: 1, Juz: 1, TextAR: "بِسْمِ اللَّهِ"},
	}
	store, _ := Load(writeTempQuranJSON(t, ayahs))

	a, ok := store.Get(1, 1)
	if !ok {
		t.Fatal("expected to find 1:1")
	}
	if a.SurahName == "" {
		t.Error("expected SurahName to be populated via fallback lookup")
	}
}
