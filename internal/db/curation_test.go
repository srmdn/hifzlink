package db

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCurationCandidatesStaySeparateUntilAccepted(t *testing.T) {
	store, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	candidate := CurationCandidate{
		Ayah1Surah: 2, Ayah1Ayah: 255, Ayah2Surah: 20, Ayah2Ayah: 111,
		SuggestedCategory: "lafzi", CategoryConfidence: "strong", CategoryBasis: "shared_exact_phrase",
		ReviewPriority: "high", MatchType: "verbatim", PhraseLen: 9,
	}
	added, err := store.ImportCurationCandidates([]CurationCandidate{candidate})
	if err != nil || added != 1 {
		t.Fatalf("first import = %d, %v; want 1, nil", added, err)
	}
	added, err = store.ImportCurationCandidates([]CurationCandidate{candidate})
	if err != nil || added != 0 {
		t.Fatalf("repeat import = %d, %v; want 0, nil", added, err)
	}

	publicRelations, err := store.All()
	if err != nil {
		t.Fatal(err)
	}
	if len(publicRelations) != 0 {
		t.Fatalf("candidate leaked into public relations before review: %d rows", len(publicRelations))
	}
	pending, accepted, rejected, err := store.CurationCandidateCounts()
	if err != nil || pending != 1 || accepted != 0 || rejected != 0 {
		t.Fatalf("candidate counts = %d/%d/%d, %v", pending, accepted, rejected, err)
	}

	items, err := store.CurationCandidates("pending", 10, 0)
	if err != nil || len(items) != 1 || items[0].SuggestedCategory != "lafzi" {
		t.Fatalf("pending candidates = %#v, %v", items, err)
	}
	if err := store.ReviewCurationCandidate(items[0].ID, true, "word_swap", "Manual category correction", `{"ayah1":[0,2],"ayah2":[1]}`); err != nil {
		t.Fatal(err)
	}
	publicRelations, err = store.All()
	if err != nil || len(publicRelations) != 1 {
		t.Fatalf("accepted relation rows = %d, %v; want 1", len(publicRelations), err)
	}
	if publicRelations[0].Category != "word_swap" || publicRelations[0].Source != "manual" || !publicRelations[0].Reviewed || publicRelations[0].Highlights != `{"ayah1":[0,2],"ayah2":[1]}` {
		t.Fatalf("accepted relation not saved as reviewed manual relation: %#v", publicRelations[0])
	}
	pending, accepted, rejected, err = store.CurationCandidateCounts()
	if err != nil || pending != 0 || accepted != 1 || rejected != 0 {
		t.Fatalf("reviewed candidate counts = %d/%d/%d, %v", pending, accepted, rejected, err)
	}
	if err := store.ReviewCurationCandidate(items[0].ID, false, "", "", ""); err == nil {
		t.Fatal("second decision should fail for an already reviewed candidate")
	}
}

func TestRejectCurationCandidateDoesNotCreateRelation(t *testing.T) {
	store, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	candidate := CurationCandidate{Ayah1Surah: 2, Ayah1Ayah: 255, Ayah2Surah: 20, Ayah2Ayah: 111, SuggestedCategory: "lafzi"}
	if _, err := store.ImportCurationCandidates([]CurationCandidate{candidate}); err != nil {
		t.Fatal(err)
	}
	items, err := store.CurationCandidates("pending", 10, 0)
	if err != nil || len(items) != 1 {
		t.Fatalf("load candidate = %d, %v", len(items), err)
	}
	if err := store.ReviewCurationCandidate(items[0].ID, false, "", "Not similar enough", ""); err != nil {
		t.Fatal(err)
	}
	relations, err := store.All()
	if err != nil || len(relations) != 0 {
		t.Fatalf("rejected candidate created %d relations, %v", len(relations), err)
	}
	if _, _, rejected, err := store.CurationCandidateCounts(); err != nil || rejected != 1 {
		t.Fatalf("rejected count = %d, %v", rejected, err)
	}
}

func TestAutomaticCurationManualReviewUpdatesNoteAndGroups(t *testing.T) {
	path := filepath.Join(t.TempDir(), "curation.db")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if store != nil {
			store.Close()
		}
	}()
	if _, err := store.ImportCurationCandidates([]CurationCandidate{{
		Ayah1Surah: 2, Ayah1Ayah: 255, Ayah2Surah: 20, Ayah2Ayah: 111,
		SuggestedCategory: "lafzi",
	}}); err != nil {
		t.Fatal(err)
	}
	items, err := store.CurationCandidates("pending", 10, 0)
	if err != nil || len(items) != 1 {
		t.Fatalf("pending items = %#v, %v", items, err)
	}
	const highlights = `{"ayah1":[0,2],"ayah2":[1]}`
	if _, err := store.AcceptAllPendingCuration([]AutomaticCurationAcceptance{{
		ID: items[0].ID, Category: "lafzi", Highlights: highlights,
	}}, "Auto-curated by Codex; manual review pending."); err != nil {
		t.Fatal(err)
	}
	counts, err := store.CurationReviewCounts()
	if err != nil || counts.Auto != 1 || counts.Manual != 0 {
		t.Fatalf("automatic review counts = %#v, %v", counts, err)
	}
	before, err := store.CurationCandidateByID(items[0].ID)
	if err != nil || before.RelationID == 0 {
		t.Fatalf("accepted candidate = %#v, %v", before, err)
	}
	if err := store.SetReviewed(before.RelationID, true, "Boss Said"); err != nil {
		t.Fatal(err)
	}
	after, err := store.CurationCandidateByID(before.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !after.ManuallyReviewed || after.ManuallyReviewedBy != "Boss Said" ||
		!strings.Contains(after.InternalNote, "Auto-curated by Codex") ||
		!strings.Contains(after.InternalNote, "Manually reviewed by admin (Boss Said)") ||
		strings.Contains(after.InternalNote, "pending") {
		t.Fatalf("private note did not transition to manual review: %#v", after)
	}
	if _, err := time.Parse(time.RFC3339, after.ManuallyReviewedAt); err != nil {
		t.Fatalf("manual review timestamp = %q: %v", after.ManuallyReviewedAt, err)
	}
	relation, found, err := store.ByID(before.RelationID)
	if err != nil || !found || !relation.Reviewed || relation.Source != "auto" ||
		relation.Category != "lafzi" || relation.Highlights != highlights || relation.Note != "" {
		t.Fatalf("quick review changed public pair data or provenance: %#v, %v", relation, err)
	}
	counts, err = store.CurationReviewCounts()
	if err != nil || counts.Auto != 0 || counts.Manual != 1 {
		t.Fatalf("manual review counts = %#v, %v", counts, err)
	}
	automatic, _ := store.CurationCandidates("auto", 10, 0)
	manual, _ := store.CurationCandidates("manual", 10, 0)
	if len(automatic) != 0 || len(manual) != 1 || manual[0].InternalNote != after.InternalNote {
		t.Fatalf("candidate was not moved between admin groups: automatic=%#v, manual=%#v", automatic, manual)
	}
	// A restart must not rewrite the human decision or its private note.
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := store.CurationCandidateByID(before.ID)
	if err != nil || !restored.ManuallyReviewed || restored.InternalNote != after.InternalNote {
		t.Fatalf("restart changed the manual review: %#v, %v", restored, err)
	}
	if err := store.SetReviewed(before.RelationID, false, "Boss Said"); err != nil {
		t.Fatal(err)
	}
	reopened, _ := store.CurationCandidateByID(before.ID)
	if reopened.ManuallyReviewed || !strings.Contains(reopened.InternalNote, "Manual review pending") ||
		!strings.Contains(reopened.InternalNote, "Auto-curated by Codex") {
		t.Fatalf("reopened review left a stale internal note: %#v", reopened)
	}
	// Editing the relation also reviews it. Its stable ID keeps the private
	// candidate history connected even when an admin corrects the pair itself.
	relation.Ayah1Ayah = 254
	relation.Category = "word_swap"
	relation.Note = "Public explanation"
	relation.Highlights = `{"ayah1":[1],"ayah2":[2]}`
	if err := store.UpdateByID(relation, "another-admin"); err != nil {
		t.Fatal(err)
	}
	edited, err := store.CurationCandidateByID(before.ID)
	if err != nil || edited.Ayah1Ayah != 254 || edited.Category != "word_swap" ||
		edited.Highlights != relation.Highlights || edited.Note != relation.Note ||
		edited.ManuallyReviewedBy != "another-admin" || strings.Contains(edited.InternalNote, "pending") {
		t.Fatalf("editing failed to synchronize review history: %#v, %v", edited, err)
	}
	if err := store.DeleteByID(relation.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Add(Relation{Ayah1Surah: 3, Ayah1Ayah: 1, Ayah2Surah: 4, Ayah2Ayah: 1, Source: "manual", Reviewed: true}); err != nil {
		t.Fatal(err)
	}
	detached, err := store.CurationCandidateByID(before.ID)
	if err != nil || detached.RelationID != 0 {
		t.Fatalf("deleted pair's private history attached to a reused ID: %#v, %v", detached, err)
	}
}
