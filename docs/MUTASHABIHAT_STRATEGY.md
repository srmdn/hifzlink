# Mutashabihat Grouping Strategy

Last updated: 2026-03-23

## Why This Exists

This note captures practical findings for structuring mutashabihat data in a way that directly helps murojaah and memorization workflows.

## Findings

1. Quran.com product direction emphasizes workflow tools (pin/compare, collections, bookmark, study mode), not a strict fixed mutashabihat taxonomy.
2. Tahfiz-focused research highlights practical methods:
- identify similar verses explicitly
- annotate differences
- use meaning/tafsir when needed for distinction
- repeat with focused review on confusion points
3. A purely thematic category set (`aqidah`, `adab`, etc.) is useful for study context but weak as a primary memorization classifier.

## Recommended Model

Use two layers:

1. Primary tags (murojaah-first confusion patterns):
- `lafzi_near_identical`
- `word_swap`
- `addition_omission`
- `order_change`
- `ending_variation`
- `pronoun_shift`
- `same_surah`
- `cross_surah`

2. Secondary tags (optional thematic context):
- `aqidah`
- `ahkam`
- `adab`
- `qasas`
- `dua`
- `targhib_tarhib`
- `other`

## Product Direction For HifzLink

1. Keep relation-level tags for curator quality.
2. Add user-level collections for personalized grouping.
3. Add bookmarks + dashboard for daily review continuity.
4. Evolve from single category to multi-tag relation records.

## Suggested Implementation Order

1. Support multi-tag storage in relation schema.
2. Add admin tag manager (create/edit/archive tags).
3. Add collection and bookmark tables (user scope).
4. Add dashboard summaries:
- pending review
- recently mistaken pairs
- last session continuation

## Sources

- Quran.com product updates: https://quran.com/en/product-updates
- New Pin & Compare Verses: https://quran.com/product-updates/new-pin-and-compare-verses
- Save Verses to Collections: https://quran.com/product-updates/save-verses-to-collections-organize-your-quran-study
- Reading Bookmark: https://quran.com/product-updates/reading-bookmark-easily-track-your-quran-progress
- Tahfiz mutashabihat study (JIMK, 2021): https://journal.unisza.edu.my/jimk/index.php/jimk/article/view/525
