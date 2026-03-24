# Changelog

All notable changes to this project will be documented in this file.

The format is inspired by Keep a Changelog.

## [Unreleased] — Milestone 3 in progress

## [0.2.0] - 2026-03-24 — Milestone 2 complete

### Added

- server-rendered admin relation management page at `/admin/relations` with add/list/edit/delete flow and word picker for diff highlights
- server-rendered collections (`/collections`, `/collections/{id}`) with create/save/remove flows
- dashboard (`/dashboard`) with quick resume links, recent collections, and recent saved items
- search page at `/search` supporting ayah ref, surah number, surah name, and category filter
- `SurahByName` lookup in `internal/search` for case-insensitive prefix match on surah names
- hero section: full viewport height (`min-height: 100dvh`) with flex vertical centering; buttons centered, equal-width, column-stacked on all breakpoints
- admin auth: server now auto-loads `.env` from project root at startup so `HIFZLINK_ADMIN_USER`/`HIFZLINK_ADMIN_PASS` work without shell export

### Changed

- compare page: related pairs section replaces sequential prev/next navigation
- relation category taxonomy revised to confusion-pattern only (`lafzi`, `addition_omission`, `word_swap`, `ending_variation`, `order_change`, `pronoun_shift`, `other`); old thematic values migrated to `other` on first startup
- `style.css` split into `base.css`, `topbar.css`, `components.css`, `admin.css`, `pages.css`
- landing page redesigned as SaaS-style public page with hero, story section, diff example (60:8 ↔ 60:9), features grid, how-it-works steps, and browse CTA
- topbar: removed Dashboard link until auth is implemented; Search links to `/search` page
- responsive button system (`.btn`, `.btn-sm`, `.btn-outline`, `.btn-danger`) replacing ad-hoc classes; all templates updated
- full mobile layout pass: hero centering, search row stacking, diff example collapse, consistent top spacing across all pages
- landing page styling audit: section labels/titles centered, story body centered, blockquote constrained, diff caption spaced
- `main { padding-top }` fix: `.container { padding: 0 1rem }` shorthand was overriding vertical spacing via class specificity; switched to explicit `padding-left`/`padding-right`
- em dashes removed from all visitor-facing templates; replaced with natural sentence structure (periods, commas, conjunctions, colons)

## [0.1.1] - 2026-03-22 — Milestone 1 complete

### Added

- unit tests for `internal/search` (Store, TranslationStore) and `internal/relations` (ParseAyahRef, Service)
- handler tests for all API endpoints (`/api/ayah/`, `/api/relations`, `/api/surah/`, `/api/juz/`) covering `lang` modes, error paths, and duplicate handling
- `docs/` folder; moved `PROJECT.md`, `ARCHITECTURE.md`, `STATUS.md`, `ROADMAP.md`, `DESIGN.md`, `TRANSLATIONS.md`, `VERSIONING.md` into it
- open source governance docs (`CONTRIBUTING.md`, `VERSIONING.md`, `NOTICE.md`, `LICENSE`)
- full dataset pipeline scripts: `scripts/import`, `scripts/validate`, `scripts/seed_relations`
- `scripts/validate_translations` command for translation key-format validation and optional coverage reporting (`-report`)
- friendly not-found page for invalid or missing ayah references
- footer on all pages with Tanzil attribution
- tafsir-ready import output at `data/tafsir/id.kemenag.json` and `data/tafsir/en.ibn-kathir.json`

### Changed

- shared `head`, `topbar`, and `footer` extracted into `_partials.html`; eliminates duplication across all page templates
- `scripts/import_translations` now imports Indonesian from `rioastamal/quran-json` and English from Quran.com default verse-route translation
- frontend responsive refinement: mobile topbar uses compact branding, `AR/EN/ID` language toggle, collapsible menu drawer
- add `lang="ar"` attribute to Arabic text elements for screen reader accuracy
- invalid search input redirects back with an inline error message and pre-filled input
- surah and juz pair lists: Compare button added per pair for direct navigation
- compare page: back link points to ayah1 instead of generic Home
- updated all internal cross-references and `README.md` links to reflect new `docs/` paths

## [0.1.0] - 2026-03-15

### Added

- initial MVP server with ayah lookup, relations API, and server-rendered pages
- surah name support in dataset, API, and UI
