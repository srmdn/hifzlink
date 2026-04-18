# Changelog

All notable changes to this project will be documented in this file.

The format is inspired by Keep a Changelog.

## [0.2.3] - 2026-04-18 — QF integration, practice tools, surah index redesign

### Added

- QF OAuth2 login flow with sessions; login/logout UI in topbar and dashboard
- QF Content API: verse audio playback on compare page
- QF User Bookmarks API: saved pairs from Quran.com shown on dashboard with richer display
- Practice Mode on compare page: hide one verse and reveal on demand
- Dashboard review prompt for saved pairs
- Surah/juz metadata display on index and detail pages
- SVG empty state icons on all no-content pages
- Mark as mastered on saved pairs
- Stats strip on homepage: total pair count, 114 surahs, 6236 ayahs
- Surah index badge: relation count shown per surah row
- Admin state indicator in topbar and mobile nav
- Proper admin login page
- Dedicated edit page for relations with live diff preview
- Search filter on admin relations list
- `updated_at` timestamp on compare page
- robots.txt, sitemap.xml, llms.txt; improved meta tags for SEO

### Changed

- Surah index: removed sidebar, filter moved above list, full-width single-column layout
- Surah index: filter input is now full-width with stats shown below
- Surah CSS extracted into dedicated `surah-index.css` and `surah-detail.css` files
- Surah detail: prev/next surah navigation
- Landing page redesigned with pair count, SVG icons, scroll cue, and browse CTA
- Mobile drawer restructured; account section pinned to bottom
- Hamburger moved to right side of topbar; mobile padding added
- QF bookmark items redesigned to distinguish two link destinations (Quran.com vs internal)
- Tafsir section auto-expands on load
- Collections scoped per user

### Fixed

- Surah index filter: rows were not hiding due to CSS specificity (`display: grid` overriding `[hidden]`); fixed via `style.display`
- Mobile menu account section cut off on short viewports
- Sticky footer and juz 8-column grid layout
- Footer links and Tanzil attribution removed
- QF OAuth2: nonce param, user scope, error logging
- QF bookmark API: correct scope, mushafId, and pagination params
- Timestamps displayed in WIB (UTC+7) with offset label
- Dark floating dropdown; topbar auth links hidden at tablet/mobile breakpoints
- Admin session cookie path set to `/` so topbar reflects admin state on all pages
- Login button hidden when admin session is active
- Logout uses POST; token exchange uses Basic Auth
- Cache-Control header added to robots.txt

### Security

- Session expiry added
- CSRF tokens on all state-changing forms
- Request body size limits
- Rate limiter memory pruning
- Force re-authentication on every login (`prompt=login`)

### Data

- Seed expanded to 150+ curated pairs covering Juz 28-30, Al-Rahman, Al-Mursalat

## [0.2.2] - 2026-03-24 — Tafsir display

### Added

- collapsible tafsir section on ayah pages — shown for `lang=en` (Ibn Kathir) and `lang=id` (Kemenag RI)
- `LoadTranslationFiles` in `internal/search` for loading stores from arbitrary lang→filepath mappings
- `tafsirFor` server helper: en tafsir rendered as trusted HTML, id tafsir converted from plain text to paragraph HTML

### Fixed

- `handlers_test.go`: missing `adminLimiter` in test server helper (caused nil panic on admin endpoint tests)
- `handlers_test.go`: stale redirect assertion for collection create (now goes to `/dashboard`)

## [0.2.1] - 2026-03-24 — Milestone 3 + open source maturity

### Added

- `docs/CURATION.md`: contributor workflow for adding and verifying mutashabihat pairs, including format reference, category guide, verification checklist, and common mistakes
- expanded `data/relations.seed.json` from 9 to 67 curated mutashabihat pairs covering surahs 2 through 114 across all confusion-pattern categories
- GitHub issue templates: bug report, feature request, data report (`.github/ISSUE_TEMPLATE/`)
- homepage screenshot in `docs/screenshots/homepage.webp`, referenced in README
- Mermaid architecture diagram and request flow sequence diagram in `docs/ARCHITECTURE.md`

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
