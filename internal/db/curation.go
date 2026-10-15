package db

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// CurationCandidate stores an automatically suggested pair separately from
// the public relations catalog until an administrator accepts it.
type CurationCandidate struct {
	ID                 int64
	Ayah1Surah         int
	Ayah1Ayah          int
	Ayah2Surah         int
	Ayah2Ayah          int
	SuggestedCategory  string
	CategoryConfidence string
	CategoryBasis      string
	ReviewPriority     string
	MatchType          string
	PhraseLen          int
	Jaccard            float64
	Status             string
	Category           string
	Note               string
	InternalNote       string
	RelationID         int64
	Highlights         string
	ManuallyReviewed   bool
	ManuallyReviewedAt string
	ManuallyReviewedBy string
	CreatedAt          string
	ReviewedAt         string
}

// ImportCurationCandidates adds newly generated suggestions without changing
// previous review decisions or importing pairs already in the public catalog.
func (s *Store) ImportCurationCandidates(candidates []CurationCandidate) (int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin curation candidate import: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
	INSERT OR IGNORE INTO curation_candidates (
		ayah1_surah, ayah1_ayah, ayah2_surah, ayah2_ayah,
		suggested_category, category_confidence, category_basis,
		review_priority, match_type, phrase_len, jaccard
	)
	SELECT ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
	WHERE NOT EXISTS (
		SELECT 1 FROM relations
		WHERE ayah1_surah = ? AND ayah1_ayah = ? AND ayah2_surah = ? AND ayah2_ayah = ?
	)
	`)
	if err != nil {
		return 0, fmt.Errorf("prepare curation candidate import: %w", err)
	}
	defer stmt.Close()

	added := 0
	for _, candidate := range candidates {
		result, err := stmt.Exec(
			candidate.Ayah1Surah, candidate.Ayah1Ayah, candidate.Ayah2Surah, candidate.Ayah2Ayah,
			candidate.SuggestedCategory, candidate.CategoryConfidence, candidate.CategoryBasis,
			candidate.ReviewPriority, candidate.MatchType, candidate.PhraseLen, candidate.Jaccard,
			candidate.Ayah1Surah, candidate.Ayah1Ayah, candidate.Ayah2Surah, candidate.Ayah2Ayah,
		)
		if err != nil {
			return 0, fmt.Errorf("import curation candidate %d:%d/%d:%d: %w", candidate.Ayah1Surah, candidate.Ayah1Ayah, candidate.Ayah2Surah, candidate.Ayah2Ayah, err)
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return 0, fmt.Errorf("count imported curation candidate: %w", err)
		}
		added += int(rows)
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit curation candidate import: %w", err)
	}
	return added, nil
}

func (s *Store) CurationCandidates(status string, limit, offset int) ([]CurationCandidate, error) {
	filter := "c.status = ?"
	var args []any
	switch status {
	case "auto":
		filter = "c.status = 'accepted' AND r.reviewed = 0"
	case "manual":
		filter = "c.status = 'accepted' AND r.reviewed = 1"
	default:
		args = append(args, status)
	}
	args = append(args, limit, offset)
	order := ""
	if status == "manual" {
		order = "c.manually_reviewed_at DESC, "
	}
	rows, err := s.db.Query(curationCandidateSelect+` WHERE `+filter+`
	ORDER BY `+order+`CASE c.review_priority WHEN 'high' THEN 0 WHEN 'medium' THEN 1 ELSE 2 END,
		c.phrase_len DESC, c.jaccard DESC, c.ayah1_surah, c.ayah1_ayah, c.ayah2_surah, c.ayah2_ayah
	LIMIT ? OFFSET ?
	`, args...)
	if err != nil {
		return nil, fmt.Errorf("query curation candidates: %w", err)
	}
	defer rows.Close()

	var candidates []CurationCandidate
	for rows.Next() {
		candidate, err := scanCurationCandidate(rows)
		if err != nil {
			return nil, fmt.Errorf("scan curation candidate: %w", err)
		}
		candidates = append(candidates, candidate)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate curation candidates: %w", err)
	}
	return candidates, nil
}

func (s *Store) CurationCandidateByID(id int64) (CurationCandidate, error) {
	candidate, err := scanCurationCandidate(s.db.QueryRow(curationCandidateSelect+` WHERE c.id = ?`, id))
	if err != nil {
		if err == sql.ErrNoRows {
			return CurationCandidate{}, fmt.Errorf("curation candidate not found")
		}
		return CurationCandidate{}, fmt.Errorf("load curation candidate: %w", err)
	}
	return candidate, nil
}

const curationCandidateSelect = `
	SELECT c.id, COALESCE(r.ayah1_surah, c.ayah1_surah), COALESCE(r.ayah1_ayah, c.ayah1_ayah),
	COALESCE(r.ayah2_surah, c.ayah2_surah), COALESCE(r.ayah2_ayah, c.ayah2_ayah),
	c.suggested_category, c.category_confidence, c.category_basis, c.review_priority,
	c.match_type, c.phrase_len, c.jaccard, c.status, COALESCE(r.category, c.category),
	COALESCE(r.note, c.note), c.internal_note, COALESCE(c.created_at, ''), COALESCE(c.reviewed_at, ''),
	COALESCE(r.id, 0), COALESCE(r.highlights, ''), COALESCE(r.reviewed, 0),
	c.manually_reviewed_at, c.manually_reviewed_by
	FROM curation_candidates c LEFT JOIN relations r ON r.id = c.relation_id
`

func scanCurationCandidate(row interface{ Scan(...any) error }) (CurationCandidate, error) {
	var c CurationCandidate
	err := row.Scan(&c.ID, &c.Ayah1Surah, &c.Ayah1Ayah, &c.Ayah2Surah, &c.Ayah2Ayah,
		&c.SuggestedCategory, &c.CategoryConfidence, &c.CategoryBasis, &c.ReviewPriority,
		&c.MatchType, &c.PhraseLen, &c.Jaccard, &c.Status, &c.Category, &c.Note, &c.InternalNote,
		&c.CreatedAt, &c.ReviewedAt, &c.RelationID, &c.Highlights, &c.ManuallyReviewed,
		&c.ManuallyReviewedAt, &c.ManuallyReviewedBy)
	return c, err
}

type CurationReviewCounts struct {
	Pending, Auto, Manual, Rejected int
}

func (s *Store) CurationReviewCounts() (CurationReviewCounts, error) {
	var counts CurationReviewCounts
	err := s.db.QueryRow(`
		SELECT COALESCE(SUM(c.status = 'pending'), 0),
		COALESCE(SUM(c.status = 'accepted' AND r.reviewed = 0), 0),
		COALESCE(SUM(c.status = 'accepted' AND r.reviewed = 1), 0),
		COALESCE(SUM(c.status = 'rejected'), 0)
		FROM curation_candidates c LEFT JOIN relations r ON r.id = c.relation_id
	`).Scan(&counts.Pending, &counts.Auto, &counts.Manual, &counts.Rejected)
	return counts, err
}

func (s *Store) CurationCandidateCounts() (pending, accepted, rejected int, err error) {
	err = s.db.QueryRow(`
	SELECT
		COALESCE(SUM(CASE WHEN status = 'pending' THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN status = 'accepted' THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN status = 'rejected' THEN 1 ELSE 0 END), 0)
	FROM curation_candidates
	`).Scan(&pending, &accepted, &rejected)
	if err != nil {
		err = fmt.Errorf("count curation candidates: %w", err)
	}
	return
}

// ReviewCurationCandidate records a decision. Accepted candidates become
// reviewed manual relations in the public catalog in the same transaction.
func (s *Store) ReviewCurationCandidate(id int64, accept bool, category, note, highlights string, reviewer ...string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin curation candidate review: %w", err)
	}
	defer tx.Rollback()

	var candidate CurationCandidate
	if err := tx.QueryRow(`
		SELECT ayah1_surah, ayah1_ayah, ayah2_surah, ayah2_ayah, status
		FROM curation_candidates WHERE id = ?
	`, id).Scan(&candidate.Ayah1Surah, &candidate.Ayah1Ayah, &candidate.Ayah2Surah, &candidate.Ayah2Ayah, &candidate.Status); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("curation candidate not found")
		}
		return fmt.Errorf("load curation candidate: %w", err)
	}
	if candidate.Status != "pending" {
		return fmt.Errorf("curation candidate already reviewed")
	}

	status := "rejected"
	var relationID int64
	if accept {
		status = "accepted"
		result, err := tx.Exec(`
			INSERT INTO relations (ayah1_surah, ayah1_ayah, ayah2_surah, ayah2_ayah, note, category, highlights, updated_at, source, reviewed)
			VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, 'manual', 1)
		`, candidate.Ayah1Surah, candidate.Ayah1Ayah, candidate.Ayah2Surah, candidate.Ayah2Ayah, note, category, highlights)
		if err != nil {
			return fmt.Errorf("accept curation candidate into relations: %w", err)
		}
		relationID, err = result.LastInsertId()
		if err != nil {
			return fmt.Errorf("read accepted relation ID: %w", err)
		}
	}
	if _, err := tx.Exec(`
		UPDATE curation_candidates
		SET status = ?, category = ?, note = ?, relation_id = ?, reviewed_at = CURRENT_TIMESTAMP
		WHERE id = ? AND status = 'pending'
	`, status, category, note, relationID, id); err != nil {
		return fmt.Errorf("save curation candidate review: %w", err)
	}
	if accept {
		if err := recordCurationManualReview(tx, relationID, true, reviewerName(reviewer)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func reviewerName(names []string) string {
	if len(names) > 0 && strings.TrimSpace(names[0]) != "" {
		return strings.TrimSpace(names[0])
	}
	return "admin"
}

// Keep the admin-only note and review metadata in the same transaction as the
// public relation change. Source stays unchanged when an automatic pair is reviewed.
func recordCurationManualReview(tx *sql.Tx, relationID int64, reviewed bool, reviewer string) error {
	var source string
	if err := tx.QueryRow(`SELECT source FROM relations WHERE id = ?`, relationID).Scan(&source); err != nil {
		return fmt.Errorf("read relation origin: %w", err)
	}
	origin := "Manually curated."
	if source == "auto" {
		origin = "Auto-curated by Codex."
	}
	now := time.Now().UTC().Format(time.RFC3339)
	note := fmt.Sprintf("%s Manually reviewed by admin (%s) on %s.", origin, reviewer, now)
	if !reviewed {
		note = fmt.Sprintf("%s Manual review pending; reopened by admin (%s) on %s.", origin, reviewer, now)
	}
	if _, err := tx.Exec(`
		UPDATE curation_candidates SET internal_note = ?,
		category = (SELECT category FROM relations WHERE id = ?),
		note = COALESCE((SELECT note FROM relations WHERE id = ?), ''),
		manually_reviewed_at = CASE WHEN ? THEN ? ELSE manually_reviewed_at END,
		manually_reviewed_by = CASE WHEN ? THEN ? ELSE manually_reviewed_by END
		WHERE status = 'accepted' AND relation_id = ?
	`, note, relationID, relationID, reviewed, now, reviewed, reviewer, relationID); err != nil {
		return fmt.Errorf("record manual curation review: %w", err)
	}
	return nil
}

type AutomaticCurationAcceptance struct {
	ID         int64
	Category   string
	Highlights string
}

// AcceptAllPendingCuration publishes generated relation data while preserving
// its Codex provenance in the admin-only candidate history.
func (s *Store) AcceptAllPendingCuration(items []AutomaticCurationAcceptance, internalNote string) (int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin bulk curation acceptance: %w", err)
	}
	defer tx.Rollback()

	insert, err := tx.Prepare(`
	INSERT INTO relations (ayah1_surah, ayah1_ayah, ayah2_surah, ayah2_ayah, note, category, highlights, updated_at, source, reviewed)
	SELECT c.ayah1_surah, c.ayah1_ayah, c.ayah2_surah, c.ayah2_ayah, '', ?, ?, CURRENT_TIMESTAMP, 'auto', 0
	FROM curation_candidates c WHERE c.id = ? AND c.status = 'pending'
	`)
	if err != nil {
		return 0, fmt.Errorf("prepare bulk curation relation insert: %w", err)
	}
	defer insert.Close()

	update, err := tx.Prepare(`
	UPDATE curation_candidates
	SET status = 'accepted', category = ?, note = '', internal_note = ?, reviewed_at = CURRENT_TIMESTAMP,
	relation_id = last_insert_rowid()
	WHERE id = ? AND status = 'pending'
	`)
	if err != nil {
		return 0, fmt.Errorf("prepare bulk curation candidate update: %w", err)
	}
	defer update.Close()

	accepted := 0
	for _, item := range items {
		if _, err := insert.Exec(item.Category, item.Highlights, item.ID); err != nil {
			return 0, fmt.Errorf("publish curation candidate %d: %w", item.ID, err)
		}
		result, err := update.Exec(item.Category, internalNote, item.ID)
		if err != nil {
			return 0, fmt.Errorf("record curation candidate %d: %w", item.ID, err)
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return 0, fmt.Errorf("count accepted curation candidate %d: %w", item.ID, err)
		}
		accepted += int(rows)
	}
	if accepted != len(items) {
		return 0, fmt.Errorf("accepted %d of %d pending candidates; transaction rolled back", accepted, len(items))
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit bulk curation acceptance: %w", err)
	}
	return accepted, nil
}
