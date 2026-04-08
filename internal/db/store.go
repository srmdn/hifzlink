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
	Highlights string // JSON: {"ayah1":[0,2],"ayah2":[1,3]}
	UpdatedAt  string
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

type RecentCollectionItem struct {
	CollectionItem
	CollectionName string
}

// Session represents an authenticated user session created via QF OAuth2.
type Session struct {
	ID           string
	UserID       string
	Email        string
	Name         string
	AccessToken  string
	RefreshToken string
	ExpiresAt    int64
	CreatedAt    string
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

	CREATE TABLE IF NOT EXISTS sessions (
		id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		email TEXT NOT NULL DEFAULT '',
		name TEXT NOT NULL DEFAULT '',
		access_token TEXT NOT NULL,
		refresh_token TEXT NOT NULL DEFAULT '',
		expires_at INTEGER NOT NULL,
		created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions (user_id);
	`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate relations table: %w", err)
	}

	if err := ensureColumn(db, "relations", "category", "ALTER TABLE relations ADD COLUMN category TEXT NOT NULL DEFAULT ''"); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := ensureColumn(db, "relations", "highlights", "ALTER TABLE relations ADD COLUMN highlights TEXT NOT NULL DEFAULT ''"); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := ensureColumn(db, "relations", "updated_at", "ALTER TABLE relations ADD COLUMN updated_at TEXT NOT NULL DEFAULT ''"); err != nil {
		_ = db.Close()
		return nil, err
	}
	if _, err := db.Exec(`UPDATE relations SET updated_at = CURRENT_TIMESTAMP WHERE COALESCE(updated_at, '') = ''`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("backfill relations.updated_at: %w", err)
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

	// Migrate old thematic category values to 'other' — these are topics, not confusion patterns.
	if _, err := db.Exec(`UPDATE relations SET category = 'other' WHERE category IN ('maana', 'siyam', 'aqidah', 'adab')`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate legacy category values: %w", err)
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) Add(rel Relation) (bool, error) {
	res, err := s.db.Exec(`
	INSERT OR IGNORE INTO relations
		(ayah1_surah, ayah1_ayah, ayah2_surah, ayah2_ayah, note, category, highlights, updated_at)
	VALUES
		(?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	`, rel.Ayah1Surah, rel.Ayah1Ayah, rel.Ayah2Surah, rel.Ayah2Ayah, rel.Note, rel.Category, rel.Highlights)
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
	SELECT id, ayah1_surah, ayah1_ayah, ayah2_surah, ayah2_ayah, COALESCE(note, ''), COALESCE(category, ''), COALESCE(highlights, ''), COALESCE(updated_at, '')
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
	SELECT id, ayah1_surah, ayah1_ayah, ayah2_surah, ayah2_ayah, COALESCE(note, ''), COALESCE(category, ''), COALESCE(highlights, ''), COALESCE(updated_at, '')
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
	SELECT id, ayah1_surah, ayah1_ayah, ayah2_surah, ayah2_ayah, COALESCE(note, ''), COALESCE(category, ''), COALESCE(highlights, ''), COALESCE(updated_at, '')
	FROM relations
	ORDER BY ayah1_surah, ayah1_ayah, ayah2_surah, ayah2_ayah
	`)
	if err != nil {
		return nil, fmt.Errorf("query all relations: %w", err)
	}
	defer rows.Close()

	return scanRelations(rows)
}

func (s *Store) CountRelations() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM relations`).Scan(&n)
	return n, err
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
	SET ayah1_surah = ?, ayah1_ayah = ?, ayah2_surah = ?, ayah2_ayah = ?, note = ?, category = ?, highlights = ?, updated_at = CURRENT_TIMESTAMP
	WHERE id = ?
	`, rel.Ayah1Surah, rel.Ayah1Ayah, rel.Ayah2Surah, rel.Ayah2Ayah, rel.Note, rel.Category, rel.Highlights, rel.ID)
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

func (s *Store) RecentCollections(limit int) ([]Collection, error) {
	if limit <= 0 {
		limit = 5
	}
	rows, err := s.db.Query(`
	SELECT c.id, c.name, COALESCE(c.description, ''), COALESCE(c.created_at, ''), COUNT(i.id)
	FROM collections AS c
	LEFT JOIN collection_items i ON i.collection_id = c.id
	GROUP BY c.id, c.name, c.description, c.created_at
	ORDER BY datetime(c.created_at) DESC, c.id DESC
	LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("query recent collections: %w", err)
	}
	defer rows.Close()

	out := make([]Collection, 0)
	for rows.Next() {
		var c Collection
		if err := rows.Scan(&c.ID, &c.Name, &c.Description, &c.CreatedAt, &c.ItemCount); err != nil {
			return nil, fmt.Errorf("scan recent collection: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recent collections: %w", err)
	}
	return out, nil
}

func (s *Store) RecentCollectionItems(limit int) ([]RecentCollectionItem, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := s.db.Query(`
	SELECT
		ci.id,
		ci.collection_id,
		ci.item_type,
		ci.ayah1_surah,
		ci.ayah1_ayah,
		ci.ayah2_surah,
		ci.ayah2_ayah,
		COALESCE(ci.note, ''),
		COALESCE(ci.created_at, ''),
		c.name
	FROM collection_items ci
	INNER JOIN collections c ON c.id = ci.collection_id
	ORDER BY datetime(ci.created_at) DESC, ci.id DESC
	LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("query recent collection items: %w", err)
	}
	defer rows.Close()

	out := make([]RecentCollectionItem, 0)
	for rows.Next() {
		var item RecentCollectionItem
		if err := rows.Scan(
			&item.ID,
			&item.Collection,
			&item.ItemType,
			&item.Ayah1Surah,
			&item.Ayah1Ayah,
			&item.Ayah2Surah,
			&item.Ayah2Ayah,
			&item.Note,
			&item.CreatedAt,
			&item.CollectionName,
		); err != nil {
			return nil, fmt.Errorf("scan recent collection item: %w", err)
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recent collection items: %w", err)
	}
	return out, nil
}

// RelationCountBySurah returns a map of surah number → distinct relation count.
// A relation is counted for a surah if either ayah belongs to it.
func (s *Store) RelationCountBySurah() (map[int]int, error) {
	rows, err := s.db.Query(`
	SELECT surah, COUNT(DISTINCT id) FROM (
		SELECT id, ayah1_surah AS surah FROM relations
		UNION ALL
		SELECT id, ayah2_surah AS surah FROM relations
	) t GROUP BY surah
	`)
	if err != nil {
		return nil, fmt.Errorf("relation count by surah: %w", err)
	}
	defer rows.Close()
	out := make(map[int]int)
	for rows.Next() {
		var surah, count int
		if err := rows.Scan(&surah, &count); err != nil {
			return nil, fmt.Errorf("scan surah count: %w", err)
		}
		out[surah] = count
	}
	return out, rows.Err()
}

func (s *Store) ByID(id int64) (Relation, bool, error) {
	var rel Relation
	err := s.db.QueryRow(`
	SELECT id, ayah1_surah, ayah1_ayah, ayah2_surah, ayah2_ayah, COALESCE(note, ''), COALESCE(category, ''), COALESCE(highlights, ''), COALESCE(updated_at, '')
	FROM relations WHERE id = ?
	`, id).Scan(
		&rel.ID, &rel.Ayah1Surah, &rel.Ayah1Ayah, &rel.Ayah2Surah, &rel.Ayah2Ayah,
		&rel.Note, &rel.Category, &rel.Highlights, &rel.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return Relation{}, false, nil
	}
	if err != nil {
		return Relation{}, false, fmt.Errorf("query relation by id: %w", err)
	}
	return rel, true, nil
}

func (s *Store) ByPair(s1, y1, s2, y2 int) (Relation, bool, error) {
	var rel Relation
	err := s.db.QueryRow(`
	SELECT id, ayah1_surah, ayah1_ayah, ayah2_surah, ayah2_ayah, COALESCE(note, ''), COALESCE(category, ''), COALESCE(highlights, ''), COALESCE(updated_at, '')
	FROM relations
	WHERE (ayah1_surah = ? AND ayah1_ayah = ? AND ayah2_surah = ? AND ayah2_ayah = ?)
	   OR (ayah1_surah = ? AND ayah1_ayah = ? AND ayah2_surah = ? AND ayah2_ayah = ?)
	LIMIT 1
	`, s1, y1, s2, y2, s2, y2, s1, y1).Scan(
		&rel.ID, &rel.Ayah1Surah, &rel.Ayah1Ayah, &rel.Ayah2Surah, &rel.Ayah2Ayah,
		&rel.Note, &rel.Category, &rel.Highlights, &rel.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return Relation{}, false, nil
	}
	if err != nil {
		return Relation{}, false, fmt.Errorf("query relation by pair: %w", err)
	}
	return rel, true, nil
}

func scanRelations(rows *sql.Rows) ([]Relation, error) {
	out := make([]Relation, 0)
	for rows.Next() {
		var rel Relation
		if err := rows.Scan(&rel.ID, &rel.Ayah1Surah, &rel.Ayah1Ayah, &rel.Ayah2Surah, &rel.Ayah2Ayah, &rel.Note, &rel.Category, &rel.Highlights, &rel.UpdatedAt); err != nil {
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

func (s *Store) CreateSession(sess Session) error {
	_, err := s.db.Exec(`
	INSERT OR REPLACE INTO sessions (id, user_id, email, name, access_token, refresh_token, expires_at, created_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	`, sess.ID, sess.UserID, sess.Email, sess.Name, sess.AccessToken, sess.RefreshToken, sess.ExpiresAt)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

func (s *Store) SessionByID(id string) (Session, error) {
	var sess Session
	err := s.db.QueryRow(`
	SELECT id, user_id, email, name, access_token, refresh_token, expires_at, COALESCE(created_at, '')
	FROM sessions WHERE id = ?
	`, id).Scan(&sess.ID, &sess.UserID, &sess.Email, &sess.Name, &sess.AccessToken, &sess.RefreshToken, &sess.ExpiresAt, &sess.CreatedAt)
	if err != nil {
		return Session{}, fmt.Errorf("session by id: %w", err)
	}
	return sess, nil
}

func (s *Store) DeleteSession(id string) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

func (s *Store) DeleteExpiredSessions() error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE expires_at < strftime('%s', 'now')`)
	if err != nil {
		return fmt.Errorf("delete expired sessions: %w", err)
	}
	return nil
}
