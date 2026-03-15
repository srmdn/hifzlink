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

### Changed

- shared `head`, `topbar`, and `footer` extracted into `_partials.html` — eliminates duplication across all page templates
- homepage: remove raw API card, rename Quick Links to Examples, add compare and browse links
- compare page: back link now points to ayah1 instead of generic Home
- surah and juz pair lists: add Compare button per pair for direct navigation
- add `lang="ar"` attribute to Arabic text elements for screen reader accuracy
- invalid search input redirects back to home with an inline error message and pre-filled input
- updated all internal cross-references and `README.md` links to reflect new `docs/` paths

## [0.1.0] - 2026-03-15

### Added

- initial MVP server with ayah lookup, relations API, and server-rendered pages
- surah name support in dataset, API, and UI
