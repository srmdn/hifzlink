# Changelog

All notable changes to this project will be documented in this file.

The format is inspired by Keep a Changelog.

## [Unreleased] — Milestone 3 in progress

### Added

- unit tests for `internal/search` (Store, TranslationStore) and `internal/relations` (ParseAyahRef, Service)
- handler tests for all API endpoints (`/api/ayah/`, `/api/relations`, `/api/surah/`, `/api/juz/`) covering `lang` modes, error paths, and duplicate handling
- `docs/` folder; moved `PROJECT.md`, `ARCHITECTURE.md`, `STATUS.md`, `ROADMAP.md`, `DESIGN.md`, `TRANSLATIONS.md`, `VERSIONING.md` into it
- open source governance docs (`CONTRIBUTING.md`, `VERSIONING.md`, `NOTICE.md`, `LICENSE`)
- full dataset pipeline scripts: `scripts/import`, `scripts/validate`, `scripts/seed_relations`
- friendly not-found page for invalid or missing ayah references
- footer on all pages with Tanzil attribution
- tafsir-ready import output at `data/tafsir/id.kemenag.json`
- tafsir-ready import output at `data/tafsir/en.ibn-kathir.json`
- `scripts/validate_translations` command for translation key-format validation and optional coverage reporting (`-report`)
- server-rendered admin relation management page at `/admin/relations` with add/list/delete actions
- admin relation editing added (update ayah refs, note, category) with custom delete dialog modal
- server-rendered collections MVP (`/collections`, `/collections/{id}`) with create/save/remove flows
- dashboard MVP (`/dashboard`) with quick resume links and recent activity cards

### Changed

- shared `head`, `topbar`, and `footer` extracted into `_partials.html` — eliminates duplication across all page templates
- homepage: remove raw API card, rename Quick Links to Examples, add compare and browse links
- compare page: back link now points to ayah1 instead of generic Home
- surah and juz pair lists: add Compare button per pair for direct navigation
- add `lang="ar"` attribute to Arabic text elements for screen reader accuracy
- invalid search input redirects back to home with an inline error message and pre-filled input
- updated all internal cross-references and `README.md` links to reflect new `docs/` paths
- `scripts/import_translations` now imports Indonesian translation from `rioastamal/quran-json`, English translation from Quran.com default chapter translation, and English tafsir from Quran API resource `169`
- `scripts/import_translations` now imports Indonesian translation from `rioastamal/quran-json`, English translation from Quran.com default verse-route data, and English tafsir from Quran API resource `169`
- added migration log for translation source switch and importer hardening in `docs/TRANSLATION_MIGRATION_2026-03-22.md`
- frontend responsive refinement: mobile topbar now uses compact `HifzLink` branding, translation toggle labels (`AR/EN/ID`), and a collapsible menu; page back links use compact `← Back` style
- topbar navigation now includes Admin entry and the mobile drawer supports admin access
- relation records now support optional `category` (`lafzi`, `maana`, `siyam`, `aqidah`, `adab`, `other`) with admin-side filtering
- ayah and compare pages now support saving items directly into collections
- top navigation now includes Dashboard entry for direct access
- compare page now shows a "Related pairs" section — all other saved pairs sharing either ayah in the current comparison, replacing the previous next/prev sequential navigation
- search page at `/search` (GET) — find mutashabihat pairs by ayah ref (e.g. `60:8`), surah number (`60`), or surah name (`Al-Mumtahanah`); topbar Search link updated to point here
- relation category taxonomy revised to confusion-pattern only: `lafzi`, `addition_omission`, `word_swap`, `ending_variation`, `order_change`, `pronoun_shift`, `other`; old thematic values (`maana`, `siyam`, `aqidah`, `adab`) migrated to `other` on first startup
- `style.css` split into focused files: `base.css`, `topbar.css`, `components.css`, `admin.css`, `pages.css` — each scoped to its concern for easier maintenance
- landing page redesigned as SaaS-style public page: hero, story section with inline diff example (60:8 ↔ 60:9), features grid, how-it-works steps, and browse CTA
- topbar: removed Dashboard link (auth deferred); Search now links to `/search` page
- responsive button system: `.btn` (primary), `.btn-sm` (inline), `.btn-outline` (secondary), `.btn-danger` (destructive) with tablet (≤768px) and mobile (≤480px) touch-target sizing; all templates updated to use new classes
- mobile layout improvements: hero content centers on small screens, search row stacks vertically, diff example collapses to single column, CTA groups adjust per breakpoint
- hero section: full viewport height (`min-height: 100dvh`) with flex vertical centering; buttons centered, equal-width, column-stacked on all breakpoints
- landing page styling audit: section labels/titles centered, story body centered, diff caption spacing, hero padding reduced to prevent flex centering conflict
- `main { padding-top }` fix: `.container { padding: 0 1rem }` shorthand was overriding vertical spacing via class specificity; switched to `padding-left`/`padding-right` only
- admin auth: server now auto-loads `.env` from project root at startup so `HIFZLINK_ADMIN_USER`/`HIFZLINK_ADMIN_PASS` work without shell export
- em dashes removed from all visitor-facing templates: replaced with periods, commas, conjunctions, or colons depending on context

## [0.2.0] - 2026-03-24 — Milestone 2 complete

### Added

- server-rendered admin relation management page at `/admin/relations` with add/list/edit/delete flow and word picker for diff highlights
- server-rendered collections (`/collections`, `/collections/{id}`) with create/save/remove flows
- dashboard (`/dashboard`) with quick resume links, recent collections, and recent saved items
- search page at `/search` supporting ayah ref, surah number, surah name, and category filter
- `SurahByName` lookup in `internal/search` for case-insensitive prefix match on surah names

### Changed

- compare page now shows related pairs section instead of sequential prev/next navigation
- relation category taxonomy revised to confusion-pattern only (`lafzi`, `addition_omission`, `word_swap`, `ending_variation`, `order_change`, `pronoun_shift`, `other`); old thematic values migrated to `other` on first startup
- style.css split into `base.css`, `topbar.css`, `components.css`, `admin.css`, `pages.css`
- landing page redesigned as SaaS-style public page with hero, story section, diff example (60:8 ↔ 60:9), features grid, how-it-works steps, and browse CTA
- topbar: removed Dashboard link until auth is implemented; Search links to `/search` page
- responsive button system (`.btn`, `.btn-sm`, `.btn-outline`, `.btn-danger`) replacing ad-hoc classes; all templates updated
- admin HTTP Basic Auth credentials now auto-loaded from `.env` at startup

## [0.1.0] - 2026-03-15

### Added

- initial MVP server with ayah lookup, relations API, and server-rendered pages
- surah name support in dataset, API, and UI
