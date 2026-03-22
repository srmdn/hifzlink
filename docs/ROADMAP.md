# Roadmap

## Goal

Deliver a stable, contributor-friendly open source Quran murojaah tool focused on mutashabihat review.

## Milestone 1: Reliability (Near Term)

1. [done] add handler/unit tests for:
- ayah endpoints with `lang` modes
- relation endpoints and duplicate handling
- translation loader and fallback behavior

2. [done] add data checks:
- translation file key-format validation command
- optional translation coverage report per language

3. [done] improve error UX:
- friendly not-found page for invalid ayah references
- clear form validation messages in search flow

## Milestone 2: Memorization UX

1. [done] relation management UI:
- simple add relation form page (server-rendered)
- [done] optional delete relation action (safe confirmation)
- [done] edit relation action

2. compare enhancements:
- visual diff emphasis for repeated/changed words (non-destructive highlighting)
- quick next/previous related ayah navigation

3. browsing improvements:
- filter relation lists by surah range/juz
- searchable relation index

4. personal workflow:
- [done] bookmark/save ayah and relation pairs
- [done] saved collections (custom groups) for murojaah sessions
- user dashboard page (recently viewed, recently compared, quick resume)
- optional notes attached to saved ayah entries

5. account support:
- login/auth for user-specific saved data
- local-first fallback mode when auth is disabled
- session and access control hardening

## Milestone 3: Data Quality And Curation

1. expand `relations.seed.json` into curated starter set
2. [in progress] define relation taxonomy for mutashabihat curation:
- primary murojaah tags (confusion pattern):
  - `lafzi_near_identical`
  - `word_swap`
  - `addition_omission`
  - `order_change`
  - `ending_variation`
  - `pronoun_shift`
- secondary thematic tags (optional):
  - `aqidah`, `ahkam`, `adab`, `qasas`, `dua`, `targhib_tarhib`, `other`
- relation records should support multiple tags over time (not single category only)
- add category/tag manager page in admin UI
3. document curation workflow for contributors

## Milestone 4: Open Source Maturity

1. CI pipeline:
- run `go test ./...`
- run dataset/translation validation scripts

2. release flow:
- tag-based releases
- changelog discipline for user-visible changes

3. contributor onboarding:
- screenshots and architecture diagram in docs
- issue templates for bug/feature/data reports

## Milestone 5: Optional Future (After Stable MVP)

1. quiz mode for memorization practice
2. word-level similarity helper tooling
3. optional cloud sync/public relation dataset

## Priority Order

Work in this sequence:

1. Reliability tests and validation tooling
2. Relation management UI
3. Compare and browse UX improvements
4. CI/release automation
5. Advanced memorization features
