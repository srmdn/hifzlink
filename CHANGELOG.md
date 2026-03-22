# Changelog

All notable changes to this project will be documented in this file.

The format is inspired by Keep a Changelog.

## [Unreleased]

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

## [0.1.0] - 2026-03-15

### Added

- initial MVP server with ayah lookup, relations API, and server-rendered pages
- surah name support in dataset, API, and UI
