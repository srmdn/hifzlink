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

type Collection struct {
	ID          int64
	Name        string
	Description string
	CreatedAt   string
	ItemCount   int
}

type CollectionItem struct {
	ID         int64
	Collection int64
	ItemType   string
	Ayah1Surah int
	Ayah1Ayah  int
	Ayah2Surah int
	Ayah2Ayah  int
	Note       string
	CreatedAt  string
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

	CREATE TABLE IF NOT EXISTS collections (
		id INTEGER PRIMARY KEY,
		name TEXT NOT NULL UNIQUE,
		description TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS collection_items (
		id INTEGER PRIMARY KEY,
		collection_id INTEGER NOT NULL,
		item_type TEXT NOT NULL,
		ayah1_surah INTEGER NOT NULL,
		ayah1_ayah INTEGER NOT NULL,
		ayah2_surah INTEGER NOT NULL DEFAULT 0,
		ayah2_ayah INTEGER NOT NULL DEFAULT 0,
		note TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(collection_id) REFERENCES collections(id) ON DELETE CASCADE,
		UNIQUE(collection_id, item_type, ayah1_surah, ayah1_ayah, ayah2_surah, ayah2_ayah)
	);
	CREATE INDEX IF NOT EXISTS idx_collection_items_collection ON collection_items (collection_id);
	`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate relations table: %w", err)
	}

	if err := ensureColumn(db, "relations", "category", "ALTER TABLE relations ADD COLUMN category TEXT NOT NULL DEFAULT ''"); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := ensureColumn(db, "collections", "created_at", "ALTER TABLE collections ADD COLUMN created_at TEXT NOT NULL DEFAULT ''"); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := ensureColumn(db, "collection_items", "created_at", "ALTER TABLE collection_items ADD COLUMN created_at TEXT NOT NULL DEFAULT ''"); err != nil {
		_ = db.Close()
		return nil, err
	}
	if _, err := db.Exec(`UPDATE collections SET created_at = CURRENT_TIMESTAMP WHERE COALESCE(created_at, '') = ''`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("backfill collections.created_at: %w", err)
	}
	if _, err := db.Exec(`UPDATE collection_items SET created_at = CURRENT_TIMESTAMP WHERE COALESCE(created_at, '') = ''`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("backfill collection_items.created_at: %w", err)
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) Add(rel Relation) (bool, error) {
	res, err := s.db.Exec(`
	INSERT OR IGNORE INTO relations
		(ayah1_surah, ayah1_ayah, ayah2_surah, ayah2_ayah, note, category)
	VALUES
		(?, ?, ?, ?, ?, ?)
	`, rel.Ayah1Surah, rel.Ayah1Ayah, rel.Ayah2Surah, rel.Ayah2Ayah, rel.Note, rel.Category)
	if err != nil {
		return false, fmt.Errorf("insert relation: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("insert relation rows affected: %w", err)
	}
	return affected > 0, nil
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

func (s *Store) CreateCollection(name, description string) (int64, error) {
	res, err := s.db.Exec(`
	INSERT INTO collections (name, description, created_at) VALUES (?, ?, CURRENT_TIMESTAMP)
	`, strings.TrimSpace(name), strings.TrimSpace(description))
	if err != nil {
		return 0, fmt.Errorf("create collection: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("create collection last insert id: %w", err)
	}
	return id, nil
}

func (s *Store) Collections() ([]Collection, error) {
	rows, err := s.db.Query(`
	SELECT c.id, c.name, COALESCE(c.description, ''), COALESCE(c.created_at, ''), COUNT(i.id)
	FROM collections
	AS c
	LEFT JOIN collection_items i ON i.collection_id = c.id
	GROUP BY c.id, c.name, c.description, c.created_at
	ORDER BY c.name
	`)
	if err != nil {
		return nil, fmt.Errorf("query collections: %w", err)
	}
	defer rows.Close()

	out := make([]Collection, 0)
	for rows.Next() {
		var c Collection
		if err := rows.Scan(&c.ID, &c.Name, &c.Description, &c.CreatedAt, &c.ItemCount); err != nil {
			return nil, fmt.Errorf("scan collection: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate collections: %w", err)
	}
	return out, nil
}

func (s *Store) CollectionByID(id int64) (Collection, error) {
	var c Collection
	err := s.db.QueryRow(`
	SELECT id, name, COALESCE(description, ''), COALESCE(created_at, '')
	FROM collections
	WHERE id = ?
	`, id).Scan(&c.ID, &c.Name, &c.Description, &c.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return Collection{}, sql.ErrNoRows
		}
		return Collection{}, fmt.Errorf("query collection by id: %w", err)
	}
	return c, nil
}

func (s *Store) AddCollectionItem(item CollectionItem) (bool, error) {
	res, err := s.db.Exec(`
	INSERT OR IGNORE INTO collection_items
		(collection_id, item_type, ayah1_surah, ayah1_ayah, ayah2_surah, ayah2_ayah, note, created_at)
	VALUES
		(?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	`, item.Collection, item.ItemType, item.Ayah1Surah, item.Ayah1Ayah, item.Ayah2Surah, item.Ayah2Ayah, strings.TrimSpace(item.Note))
	if err != nil {
		return false, fmt.Errorf("add collection item: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("add collection item rows affected: %w", err)
	}
	return affected > 0, nil
}

func (s *Store) CollectionItems(collectionID int64) ([]CollectionItem, error) {
	rows, err := s.db.Query(`
	SELECT id, collection_id, item_type, ayah1_surah, ayah1_ayah, ayah2_surah, ayah2_ayah, COALESCE(note, ''), COALESCE(created_at, '')
	FROM collection_items
	WHERE collection_id = ?
	ORDER BY id DESC
	`, collectionID)
	if err != nil {
		return nil, fmt.Errorf("query collection items: %w", err)
	}
	defer rows.Close()

	out := make([]CollectionItem, 0)
	for rows.Next() {
		var item CollectionItem
		if err := rows.Scan(&item.ID, &item.Collection, &item.ItemType, &item.Ayah1Surah, &item.Ayah1Ayah, &item.Ayah2Surah, &item.Ayah2Ayah, &item.Note, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan collection item: %w", err)
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate collection items: %w", err)
	}
	return out, nil
}

func (s *Store) DeleteCollectionItem(itemID int64) error {
	res, err := s.db.Exec(`DELETE FROM collection_items WHERE id = ?`, itemID)
	if err != nil {
		return fmt.Errorf("delete collection item: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete collection item rows affected: %w", err)
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
