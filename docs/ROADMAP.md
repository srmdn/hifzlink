# Roadmap

## Goal

Deliver a stable, contributor-friendly open source Quran murojaah tool focused on mutashabihat review.

## Milestone 1: Reliability — [done]

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

## Milestone 2: Memorization UX — [done]

1. [done] relation management UI:
- [done] simple add relation form page (server-rendered)
- [done] optional delete relation action (safe confirmation)
- [done] edit relation action

2. [done] compare enhancements:
- [done] visual diff emphasis for repeated/changed words (non-destructive highlighting)
- [done] related pairs section on compare page — shows all pairs sharing either ayah, replaces next/prev sequential navigation

3. [done] browsing improvements:
- [done] surah index page with relation counts
- [done] juz index page with relation counts
- [done] ayah-relation search page (search-first, not full dump) — supports ayah ref, surah number, surah name
- [done] category filter on search page
- server-side pagination — **deferred**: dataset is manually curated and unlikely to grow large enough to need it; revisit if relation count exceeds ~500
- faceted filters beyond category (`surah`, `juz`, `has_note`, date range) — **deferred**: search page covers the core use case
- surah-range/juz filter on relation lists — **deferred**
- recent-first ordering on relation lists — **deferred**

4. [done] personal workflow:
- [done] bookmark/save ayah and relation pairs
- [done] saved collections (custom groups) for murojaah sessions
- [done] user dashboard page (quick resume, recent collections, recent saved items)

5. [done] public-facing UI:
- [done] landing page redesigned as SaaS-style public page (hero, story, diff example, features, how-it-works, browse CTA)
- [done] topbar restructured: Dashboard hidden until auth is implemented; Search links to /search
- [done] style.css split into focused files (base, topbar, components, admin, pages)
- [done] responsive button system (.btn, .btn-sm, .btn-outline, .btn-danger) with touch targets
- [done] full mobile layout pass: hero centering, search row stacking, diff example collapse

6. account support — **deferred to post-MVP**:
- login/auth for user-specific saved data
- local-first fallback mode when auth is disabled
- session and access control hardening

## Milestone 3: Data Quality And Curation

1. [done] expand `relations.seed.json` into a curated starter set of well-known mutashabihat pairs (67 pairs across all categories)
2. [done] define relation taxonomy for mutashabihat curation:
- confusion-pattern categories (single field, admin-selectable):
  - `lafzi` — near-identical wording, almost word-for-word
  - `word_swap` — one or more words differ between otherwise identical verses
  - `addition_omission` — one verse has an extra word or phrase the other lacks
  - `order_change` — same words in different sequence
  - `ending_variation` — verses identical except for the final word or phrase
  - `pronoun_shift` — differs only in pronoun (هو/هم, كم/كن, etc.)
  - `other` — does not fit cleanly into any above pattern
- multi-tag support and secondary thematic tags — **deferred**: single confusion-pattern field is sufficient for current dataset size
3. [done] document curation workflow for contributors (`docs/CURATION.md`)

## Milestone 4: Open Source Maturity

1. CI pipeline — **deferred**:
- run `go test ./...`
- run dataset/translation validation scripts

2. release flow: [done]
- tag-based releases
- changelog discipline for user-visible changes

3. contributor onboarding: [done]
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
