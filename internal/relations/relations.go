package relations

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/srmdn/hifzlink/internal/db"
	"github.com/srmdn/hifzlink/internal/search"
)

type AyahView struct {
	Surah       int    `json:"surah"`
	SurahName   string `json:"surah_name,omitempty"`
	Ayah        int    `json:"ayah"`
	Juz         int    `json:"juz,omitempty"`
	Text        string `json:"text"`
	Translation string `json:"translation_text,omitempty"`
}

type PairView struct {
	Ayah1     string `json:"ayah1"`
	Ayah1Name string `json:"ayah1_name,omitempty"`
	Ayah1URL  string `json:"-"`
	Ayah2     string `json:"ayah2"`
	Ayah2Name string `json:"ayah2_name,omitempty"`
	Ayah2URL  string `json:"-"`
	Note      string `json:"note,omitempty"`
}

type AdminRelationView struct {
	ID         int64
	Ayah1      string
	Ayah1Name  string
	Ayah2      string
	Ayah2Name  string
	Note       string
	Category   string
	Highlights string
	UpdatedAt  string
}

type Service struct {
	db    *db.Store
	quran *search.Store
}

func NewService(dbStore *db.Store, quran *search.Store) *Service {
	return &Service{db: dbStore, quran: quran}
}

func ParseAyahRef(v string) (int, int, error) {
	parts := strings.Split(strings.TrimSpace(v), ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("ayah must be in surah:ayah format")
	}

	surah, err := strconv.Atoi(parts[0])
	if err != nil || surah < 1 {
		return 0, 0, fmt.Errorf("invalid surah")
	}

	ayah, err := strconv.Atoi(parts[1])
	if err != nil || ayah < 1 {
		return 0, 0, fmt.Errorf("invalid ayah")
	}

	return surah, ayah, nil
}

func FormatAyahRef(surah, ayah int) string {
	return fmt.Sprintf("%d:%d", surah, ayah)
}

func (s *Service) Add(ayah1Ref, ayah2Ref, note string) error {
	return s.AddWithCategory(ayah1Ref, ayah2Ref, note, "")
}

func (s *Service) AddWithCategory(ayah1Ref, ayah2Ref, note, category string) error {
	s1, a1, err := ParseAyahRef(ayah1Ref)
	if err != nil {
		return fmt.Errorf("ayah1: %w", err)
	}

	s2, a2, err := ParseAyahRef(ayah2Ref)
	if err != nil {
		return fmt.Errorf("ayah2: %w", err)
	}

	if _, ok := s.quran.Get(s1, a1); !ok {
		return fmt.Errorf("ayah1 not found in dataset")
	}
	if _, ok := s.quran.Get(s2, a2); !ok {
		return fmt.Errorf("ayah2 not found in dataset")
	}

	// Keep a stable ordering so reverse duplicates are ignored by DB UNIQUE key.
	if comesAfter(s1, a1, s2, a2) {
		s1, s2 = s2, s1
		a1, a2 = a2, a1
	}

	inserted, err := s.db.Add(db.Relation{
		Ayah1Surah: s1,
		Ayah1Ayah:  a1,
		Ayah2Surah: s2,
		Ayah2Ayah:  a2,
		Note:       strings.TrimSpace(note),
		Category:   normalizeCategory(category),
	})
	if err != nil {
		return err
	}
	if !inserted {
		return fmt.Errorf("relation already exists")
	}
	return nil
}

func (s *Service) RelatedAyahs(surah, ayah int) ([]AyahView, error) {
	rels, err := s.db.ByAyah(surah, ayah)
	if err != nil {
		return nil, err
	}

	out := make([]AyahView, 0, len(rels))
	for _, rel := range rels {
		tSurah, tAyah := rel.Ayah2Surah, rel.Ayah2Ayah
		if rel.Ayah2Surah == surah && rel.Ayah2Ayah == ayah {
			tSurah, tAyah = rel.Ayah1Surah, rel.Ayah1Ayah
		}
		target, ok := s.quran.Get(tSurah, tAyah)
		if !ok {
			continue
		}
		out = append(out, AyahView{
			Surah:     target.Surah,
			SurahName: target.SurahName,
			Ayah:      target.Ayah,
			Juz:       target.Juz,
			Text:      target.TextAR,
		})
	}

	return out, nil
}

func comesAfter(s1, a1, s2, a2 int) bool {
	if s1 != s2 {
		return s1 > s2
	}
	return a1 > a2
}

func (s *Service) PairsBySurah(surah int) ([]PairView, error) {
	rels, err := s.db.BySurah(surah)
	if err != nil {
		return nil, err
	}

	out := make([]PairView, 0, len(rels))
	for _, rel := range rels {
		out = append(out, PairView{
			Ayah1:     FormatAyahRef(rel.Ayah1Surah, rel.Ayah1Ayah),
			Ayah1Name: s.quran.SurahName(rel.Ayah1Surah),
			Ayah1URL:  fmt.Sprintf("/ayah/%d/%d", rel.Ayah1Surah, rel.Ayah1Ayah),
			Ayah2:     FormatAyahRef(rel.Ayah2Surah, rel.Ayah2Ayah),
			Ayah2Name: s.quran.SurahName(rel.Ayah2Surah),
			Ayah2URL:  fmt.Sprintf("/ayah/%d/%d", rel.Ayah2Surah, rel.Ayah2Ayah),
			Note:      rel.Note,
		})
	}
	return out, nil
}

func (s *Service) PairsByJuz(juz int) ([]PairView, error) {
	rels, err := s.db.All()
	if err != nil {
		return nil, err
	}

	out := make([]PairView, 0)
	for _, rel := range rels {
		a1, ok1 := s.quran.Get(rel.Ayah1Surah, rel.Ayah1Ayah)
		a2, ok2 := s.quran.Get(rel.Ayah2Surah, rel.Ayah2Ayah)
		if !ok1 || !ok2 {
			continue
		}

		if a1.Juz != juz && a2.Juz != juz {
			continue
		}

		out = append(out, PairView{
			Ayah1:     FormatAyahRef(rel.Ayah1Surah, rel.Ayah1Ayah),
			Ayah1Name: s.quran.SurahName(rel.Ayah1Surah),
			Ayah1URL:  fmt.Sprintf("/ayah/%d/%d", rel.Ayah1Surah, rel.Ayah1Ayah),
			Ayah2:     FormatAyahRef(rel.Ayah2Surah, rel.Ayah2Ayah),
			Ayah2Name: s.quran.SurahName(rel.Ayah2Surah),
			Ayah2URL:  fmt.Sprintf("/ayah/%d/%d", rel.Ayah2Surah, rel.Ayah2Ayah),
			Note:      rel.Note,
		})
	}

	return out, nil
}

func (s *Service) AllRelations() ([]AdminRelationView, error) {
	rels, err := s.db.All()
	if err != nil {
		return nil, err
	}

	out := make([]AdminRelationView, 0, len(rels))
	for _, rel := range rels {
		out = append(out, AdminRelationView{
			ID:         rel.ID,
			Ayah1:      FormatAyahRef(rel.Ayah1Surah, rel.Ayah1Ayah),
			Ayah1Name:  s.quran.SurahName(rel.Ayah1Surah),
			Ayah2:      FormatAyahRef(rel.Ayah2Surah, rel.Ayah2Ayah),
			Ayah2Name:  s.quran.SurahName(rel.Ayah2Surah),
			Note:       rel.Note,
			Category:   rel.Category,
			Highlights: rel.Highlights,
			UpdatedAt:  rel.UpdatedAt,
		})
	}
	return out, nil
}

func (s *Service) RelationByID(id int64) (AdminRelationView, bool, error) {
	rel, ok, err := s.db.ByID(id)
	if err != nil || !ok {
		return AdminRelationView{}, ok, err
	}
	return AdminRelationView{
		ID:         rel.ID,
		Ayah1:      FormatAyahRef(rel.Ayah1Surah, rel.Ayah1Ayah),
		Ayah1Name:  s.quran.SurahName(rel.Ayah1Surah),
		Ayah2:      FormatAyahRef(rel.Ayah2Surah, rel.Ayah2Ayah),
		Ayah2Name:  s.quran.SurahName(rel.Ayah2Surah),
		Note:       rel.Note,
		Category:   rel.Category,
		Highlights: rel.Highlights,
		UpdatedAt:  rel.UpdatedAt,
	}, true, nil
}

func (s *Service) DeleteByID(id int64) error {
	if id <= 0 {
		return fmt.Errorf("invalid relation id")
	}
	if err := s.db.DeleteByID(id); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("relation not found")
		}
		return err
	}
	return nil
}

func (s *Service) UpdateByID(id int64, ayah1Ref, ayah2Ref, note, category, highlights string) error {
	if id <= 0 {
		return fmt.Errorf("invalid relation id")
	}

	s1, a1, err := ParseAyahRef(ayah1Ref)
	if err != nil {
		return fmt.Errorf("ayah1: %w", err)
	}
	s2, a2, err := ParseAyahRef(ayah2Ref)
	if err != nil {
		return fmt.Errorf("ayah2: %w", err)
	}

	if _, ok := s.quran.Get(s1, a1); !ok {
		return fmt.Errorf("ayah1 not found in dataset")
	}
	if _, ok := s.quran.Get(s2, a2); !ok {
		return fmt.Errorf("ayah2 not found in dataset")
	}

	if comesAfter(s1, a1, s2, a2) {
		s1, s2 = s2, s1
		a1, a2 = a2, a1
	}

	err = s.db.UpdateByID(db.Relation{
		ID:         id,
		Ayah1Surah: s1,
		Ayah1Ayah:  a1,
		Ayah2Surah: s2,
		Ayah2Ayah:  a2,
		Note:       strings.TrimSpace(note),
		Category:   normalizeCategory(category),
		Highlights: strings.TrimSpace(highlights),
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("relation not found")
		}
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return fmt.Errorf("relation already exists")
		}
		return err
	}
	return nil
}

func normalizeCategory(value string) string {
	c := strings.ToLower(strings.TrimSpace(value))
	switch c {
	case "lafzi", "maana", "siyam", "aqidah", "adab", "other":
		return c
	default:
		return ""
	}
}
