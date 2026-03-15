package search

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

type Ayah struct {
	Surah  int    `json:"surah"`
	Ayah   int    `json:"ayah"`
	Juz    int    `json:"juz"`
	TextAR string `json:"text_ar"`
}

type Store struct {
	byKey   map[string]Ayah
	bySurah map[int][]Ayah
	byJuz   map[int][]Ayah
}

func Load(path string) (*Store, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open quran dataset: %w", err)
	}
	defer f.Close()

	var ayahs []Ayah
	if err := json.NewDecoder(f).Decode(&ayahs); err != nil {
		return nil, fmt.Errorf("decode quran dataset: %w", err)
	}

	s := &Store{
		byKey:   make(map[string]Ayah, len(ayahs)),
		bySurah: map[int][]Ayah{},
		byJuz:   map[int][]Ayah{},
	}

	for _, a := range ayahs {
		k := key(a.Surah, a.Ayah)
		s.byKey[k] = a
		s.bySurah[a.Surah] = append(s.bySurah[a.Surah], a)
		s.byJuz[a.Juz] = append(s.byJuz[a.Juz], a)
	}

	for surah := range s.bySurah {
		sort.Slice(s.bySurah[surah], func(i, j int) bool {
			return s.bySurah[surah][i].Ayah < s.bySurah[surah][j].Ayah
		})
	}

	for juz := range s.byJuz {
		sort.Slice(s.byJuz[juz], func(i, j int) bool {
			if s.byJuz[juz][i].Surah == s.byJuz[juz][j].Surah {
				return s.byJuz[juz][i].Ayah < s.byJuz[juz][j].Ayah
			}
			return s.byJuz[juz][i].Surah < s.byJuz[juz][j].Surah
		})
	}

	return s, nil
}

func (s *Store) Get(surah, ayah int) (Ayah, bool) {
	a, ok := s.byKey[key(surah, ayah)]
	return a, ok
}

func (s *Store) BySurah(surah int) []Ayah {
	out := s.bySurah[surah]
	cloned := make([]Ayah, len(out))
	copy(cloned, out)
	return cloned
}

func (s *Store) ByJuz(juz int) []Ayah {
	out := s.byJuz[juz]
	cloned := make([]Ayah, len(out))
	copy(cloned, out)
	return cloned
}

func key(surah, ayah int) string {
	return fmt.Sprintf("%d:%d", surah, ayah)
}
