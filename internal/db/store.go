package db

import (
	"database/sql"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

type Relation struct {
	ID         int64
	Ayah1Surah int
	Ayah1Ayah  int
	Ayah2Surah int
	Ayah2Ayah  int
	Note       string
	Category   string
}

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	if _, err := db.Exec(`
	CREATE TABLE IF NOT EXISTS relations (
		id INTEGER PRIMARY KEY,
		ayah1_surah INTEGER NOT NULL,
		ayah1_ayah INTEGER NOT NULL,
		ayah2_surah INTEGER NOT NULL,
		ayah2_ayah INTEGER NOT NULL,
		note TEXT,
		UNIQUE(ayah1_surah, ayah1_ayah, ayah2_surah, ayah2_ayah)
	);
	CREATE INDEX IF NOT EXISTS idx_relations_ayah1 ON relations (ayah1_surah, ayah1_ayah);
	CREATE INDEX IF NOT EXISTS idx_relations_ayah2 ON relations (ayah2_surah, ayah2_ayah);
	`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate relations table: %w", err)
	}

	if err := ensureColumn(db, "relations", "category", "ALTER TABLE relations ADD COLUMN category TEXT NOT NULL DEFAULT ''"); err != nil {
		_ = db.Close()
		return nil, err
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) Add(rel Relation) error {
	_, err := s.db.Exec(`
	INSERT OR IGNORE INTO relations
		(ayah1_surah, ayah1_ayah, ayah2_surah, ayah2_ayah, note, category)
	VALUES
		(?, ?, ?, ?, ?, ?)
	`, rel.Ayah1Surah, rel.Ayah1Ayah, rel.Ayah2Surah, rel.Ayah2Ayah, rel.Note, rel.Category)
	if err != nil {
		return fmt.Errorf("insert relation: %w", err)
	}
	return nil
}

func (s *Store) ByAyah(surah, ayah int) ([]Relation, error) {
	rows, err := s.db.Query(`
	SELECT id, ayah1_surah, ayah1_ayah, ayah2_surah, ayah2_ayah, COALESCE(note, ''), COALESCE(category, '')
	FROM relations
	WHERE (ayah1_surah = ? AND ayah1_ayah = ?)
	   OR (ayah2_surah = ? AND ayah2_ayah = ?)
	ORDER BY ayah1_surah, ayah1_ayah, ayah2_surah, ayah2_ayah
	`, surah, ayah, surah, ayah)
	if err != nil {
		return nil, fmt.Errorf("query relations by ayah: %w", err)
	}
	defer rows.Close()

	return scanRelations(rows)
}

func (s *Store) BySurah(surah int) ([]Relation, error) {
	rows, err := s.db.Query(`
	SELECT id, ayah1_surah, ayah1_ayah, ayah2_surah, ayah2_ayah, COALESCE(note, ''), COALESCE(category, '')
	FROM relations
	WHERE ayah1_surah = ? OR ayah2_surah = ?
	ORDER BY ayah1_ayah, ayah2_ayah
	`, surah, surah)
	if err != nil {
		return nil, fmt.Errorf("query relations by surah: %w", err)
	}
	defer rows.Close()

	return scanRelations(rows)
}

func (s *Store) All() ([]Relation, error) {
	rows, err := s.db.Query(`
	SELECT id, ayah1_surah, ayah1_ayah, ayah2_surah, ayah2_ayah, COALESCE(note, ''), COALESCE(category, '')
	FROM relations
	ORDER BY ayah1_surah, ayah1_ayah, ayah2_surah, ayah2_ayah
	`)
	if err != nil {
		return nil, fmt.Errorf("query all relations: %w", err)
	}
	defer rows.Close()

	return scanRelations(rows)
}

func (s *Store) DeleteByID(id int64) error {
	res, err := s.db.Exec(`DELETE FROM relations WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete relation: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete relation rows affected: %w", err)
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) UpdateByID(rel Relation) error {
	res, err := s.db.Exec(`
	UPDATE relations
	SET ayah1_surah = ?, ayah1_ayah = ?, ayah2_surah = ?, ayah2_ayah = ?, note = ?, category = ?
	WHERE id = ?
	`, rel.Ayah1Surah, rel.Ayah1Ayah, rel.Ayah2Surah, rel.Ayah2Ayah, rel.Note, rel.Category, rel.ID)
	if err != nil {
		return fmt.Errorf("update relation: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update relation rows affected: %w", err)
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func scanRelations(rows *sql.Rows) ([]Relation, error) {
	out := make([]Relation, 0)
	for rows.Next() {
		var rel Relation
		if err := rows.Scan(&rel.ID, &rel.Ayah1Surah, &rel.Ayah1Ayah, &rel.Ayah2Surah, &rel.Ayah2Ayah, &rel.Note, &rel.Category); err != nil {
			return nil, fmt.Errorf("scan relation: %w", err)
		}
		out = append(out, rel)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate relations: %w", err)
	}
	return out, nil
}

func ensureColumn(db *sql.DB, table, column, alterSQL string) error {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return fmt.Errorf("check schema for %s.%s: %w", table, column, err)
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, colType string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &colType, &notNull, &defaultValue, &pk); err != nil {
			return fmt.Errorf("scan schema for %s.%s: %w", table, column, err)
		}
		if strings.EqualFold(name, column) {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate schema for %s.%s: %w", table, column, err)
	}

	if _, err := db.Exec(alterSQL); err != nil {
		return fmt.Errorf("migrate %s.%s: %w", table, column, err)
	}
	return nil
}
