# Changelog

All notable changes to this project will be documented in this file.

The format is inspired by Keep a Changelog.

## [Unreleased]

### Added

- unit tests for `internal/search` (Store, TranslationStore) and `internal/relations` (ParseAyahRef, Service)
- handler tests for all API endpoints (`/api/ayah/`, `/api/relations`, `/api/surah/`, `/api/juz/`) covering `lang` modes, error paths, and duplicate handling
- `docs/` folder; moved `PROJECT.md`, `ARCHITECTURE.md`, `STATUS.md`, `ROADMAP.md`, `DESIGN.md`, `TRANSLATIONS.md`, `VERSIONING.md` into it

### Changed

- extract shared `head`, `topbar`, and `footer` into `_partials.html` — eliminates duplication across all page templates
- add footer to all pages with Tanzil attribution and open source link
- homepage: remove raw API card, rename Quick Links to Examples with a compare link added
- compare page: back link now points to ayah1 instead of generic Home
- surah and juz pair lists: add Compare button per pair for direct navigation
- add `lang="ar"` attribute to Arabic text elements for screen reader accuracy
- invalid search input now redirects back to home with an inline error message and pre-filled input instead of a plain error page
- missing or invalid ayah references now show a styled not-found page instead of a bare 404
- updated all internal cross-references and `README.md` links to reflect new `docs/` paths
- Open source governance docs (`CONTRIBUTING.md`, `VERSIONING.md`, `NOTICE.md`, `LICENSE`)
- documentation requirements for Quran dataset integrity and attribution
- full dataset pipeline scripts:
  - `scripts/import` (Tanzil text + metadata -> `data/quran.json`)
  - `scripts/validate` (`6236` count, uniqueness, field/range checks)
  - `scripts/seed_relations` (starter mutashabihat examples)

## [0.1.0] - 2026-03-15

### Added

- initial MVP server with ayah lookup, relations API, and server-rendered pages
- surah name support in dataset, API, and UI
