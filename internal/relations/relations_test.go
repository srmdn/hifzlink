package relations

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/srmdn/hifzlink/internal/db"
	"github.com/srmdn/hifzlink/internal/search"
)

func testQuranStore(t *testing.T) *search.Store {
	t.Helper()
	ayahs := []search.Ayah{
		{Surah: 1, SurahName: "Al-Fatihah", Ayah: 1, Juz: 1, TextAR: "بِسْمِ اللَّهِ"},
		{Surah: 60, SurahName: "Al-Mumtahanah", Ayah: 8, Juz: 28, TextAR: "لَا يَنْهَاكُمُ اللَّهُ"},
		{Surah: 60, SurahName: "Al-Mumtahanah", Ayah: 9, Juz: 28, TextAR: "إِنَّمَا يَنْهَاكُمُ اللَّهُ"},
	}
	f, err := os.CreateTemp(t.TempDir(), "quran*.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.NewEncoder(f).Encode(ayahs); err != nil {
		t.Fatal(err)
	}
	f.Close()
	store, err := search.Load(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func testDB(t *testing.T) *db.Store {
	t.Helper()
	store, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestParseAyahRef(t *testing.T) {
	tests := []struct {
		input   string
		wantS   int
		wantA   int
		wantErr bool
	}{
		{"60:8", 60, 8, false},
		{"1:1", 1, 1, false},
		{"114:6", 114, 6, false},
		{"", 0, 0, true},
		{"60", 0, 0, true},
		{"60:abc", 0, 0, true},
		{"0:1", 0, 0, true},
		{"60:0", 0, 0, true},
		{":8", 0, 0, true},
	}

	for _, tc := range tests {
		s, a, err := ParseAyahRef(tc.input)
		if tc.wantErr {
			if err == nil {
				t.Errorf("ParseAyahRef(%q): expected error, got nil", tc.input)
			}
		} else {
			if err != nil {
				t.Errorf("ParseAyahRef(%q): unexpected error: %v", tc.input, err)
			}
			if s != tc.wantS || a != tc.wantA {
				t.Errorf("ParseAyahRef(%q): got %d:%d, want %d:%d", tc.input, s, a, tc.wantS, tc.wantA)
			}
		}
	}
}

func TestFormatAyahRef(t *testing.T) {
	if got := FormatAyahRef(60, 8); got != "60:8" {
		t.Errorf("FormatAyahRef(60, 8) = %q, want %q", got, "60:8")
	}
}

func TestService_Add_Valid(t *testing.T) {
	svc := NewService(testDB(t), testQuranStore(t))

	if err := svc.Add("60:8", "60:9", "mutashabihat"); err != nil {
		t.Fatalf("Add: %v", err)
	}
}

func TestService_Add_InvalidRef(t *testing.T) {
	svc := NewService(testDB(t), testQuranStore(t))

	if err := svc.Add("invalid", "60:9", ""); err == nil {
		t.Error("expected error for invalid ayah1 ref")
	}
	if err := svc.Add("60:8", "invalid", ""); err == nil {
		t.Error("expected error for invalid ayah2 ref")
	}
}

func TestService_Add_AyahNotInDataset(t *testing.T) {
	svc := NewService(testDB(t), testQuranStore(t))

	if err := svc.Add("99:99", "60:9", ""); err == nil {
		t.Error("expected error when ayah1 not in dataset")
	}
	if err := svc.Add("60:8", "99:99", ""); err == nil {
		t.Error("expected error when ayah2 not in dataset")
	}
}

func TestService_Add_DuplicateReturnsError(t *testing.T) {
	svc := NewService(testDB(t), testQuranStore(t))

	if err := svc.Add("60:8", "60:9", "note1"); err != nil {
		t.Fatalf("first Add: %v", err)
	}
	// Reversed order — normalised to same pair, should return duplicate error.
	if err := svc.Add("60:9", "60:8", "note2"); err == nil {
		t.Fatal("expected duplicate relation error")
	}
}

func TestService_RelatedAyahs(t *testing.T) {
	svc := NewService(testDB(t), testQuranStore(t))

	if err := svc.Add("60:8", "60:9", "mutashabihat"); err != nil {
		t.Fatal(err)
	}

	related, err := svc.RelatedAyahs(60, 8)
	if err != nil {
		t.Fatalf("RelatedAyahs: %v", err)
	}
	if len(related) != 1 {
		t.Fatalf("expected 1 related ayah, got %d", len(related))
	}
	if related[0].Surah != 60 || related[0].Ayah != 9 {
		t.Errorf("unexpected related ayah: %+v", related[0])
	}
}

func TestService_RelatedAyahs_Bidirectional(t *testing.T) {
	svc := NewService(testDB(t), testQuranStore(t))

	if err := svc.Add("60:8", "60:9", ""); err != nil {
		t.Fatal(err)
	}

	related, err := svc.RelatedAyahs(60, 9)
	if err != nil {
		t.Fatalf("RelatedAyahs: %v", err)
	}
	if len(related) != 1 {
		t.Fatalf("expected 1 related ayah, got %d", len(related))
	}
	if related[0].Surah != 60 || related[0].Ayah != 8 {
		t.Errorf("unexpected related ayah: %+v", related[0])
	}
}

func TestService_RelatedAyahs_Empty(t *testing.T) {
	svc := NewService(testDB(t), testQuranStore(t))

	related, err := svc.RelatedAyahs(1, 1)
	if err != nil {
		t.Fatalf("RelatedAyahs: %v", err)
	}
	if len(related) != 0 {
		t.Error("expected empty related list for ayah with no relations")
	}
}

func TestService_UpdateByID(t *testing.T) {
	svc := NewService(testDB(t), testQuranStore(t))

	if err := svc.Add("60:8", "60:9", "before"); err != nil {
		t.Fatal(err)
	}
	rows, err := svc.AllRelations()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}

	if err := svc.UpdateByID(rows[0].ID, "60:9", "60:8", "after", "LAFZI"); err != nil {
		t.Fatalf("UpdateByID: %v", err)
	}

	updated, err := svc.AllRelations()
	if err != nil {
		t.Fatal(err)
	}
	if updated[0].Note != "after" {
		t.Fatalf("expected updated note, got %q", updated[0].Note)
	}
	if updated[0].Category != "lafzi" {
		t.Fatalf("expected normalized category lafzi, got %q", updated[0].Category)
	}
}
