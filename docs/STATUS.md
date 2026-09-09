# Project status

Last updated: August 22, 2026

## Current state

HifzLink is in active post-v0.2.3 development. The public browsing,
mutashabihat comparison, account, collection, and administration workflows are
implemented.

- full Quran Arabic dataset loaded locally (`6236` ayahs)
- local SQLite relation storage working with migration system
- server-rendered pages: home, ayah, compare, surah index, juz index, search,
  collections, dashboard, and admin
- EN/ID translation toggle implemented (`ar`, `en`, `id`)
- landing page with product overview, live statistics, and browse actions
- Quran Foundation OAuth2 login with expiring local sessions
- Quran Foundation bookmark synchronization and audio integration
- user collections, saved pairs, mastery state, and dashboard
- CSS split into focused files: `base.css`, `topbar.css`, `components.css`, `admin.css`, `pages.css`
- responsive button system (`.btn`, `.btn-sm`, `.btn-outline`, `.btn-danger`)
  with mobile touch targets
- full mobile layout pass: hero centering, search row stacking, diff example
  collapse, and consistent top spacing
- search page at `/search` supports ayah ref, surah number, surah name, and category filter
- compare page shows related pairs (all pairs sharing either ayah) instead of sequential prev/next
- category taxonomy revised to confusion-pattern only: `lafzi`,
  `addition_omission`, `word_swap`, `ending_variation`, `order_change`,
  `pronoun_shift`, and `other`
- old thematic category values migrated to `other` on startup via DB migration
- admin auth auto-loaded from `.env` at startup (no shell export needed for local dev)
- em dashes removed from all visitor-facing templates; replaced with natural sentence structure
- unit and handler tests passing (`go test ./...`)
- GitHub Actions CI runs tests, dataset validation, translation validation, and
  the server build for `staging` and `main` changes
- production VPS migration from legacy `production` to `main` is complete

## Implemented Features

- ayah lookup (`GET /api/ayah/{surah}/{ayah}`)
- related ayah lookup (`GET /api/ayah/{surah}/{ayah}/relations`)
- add relation (`POST /api/relations`)
- relations by surah (`GET /api/surah/{surah}/relations`)
- relations by juz (`GET /api/juz/{juz}/relations`)
- compare page with side-by-side ayahs and word-level diff highlighting
- language mode persistence via `?lang=` query parameter
- search page (`GET /search`) with ayah ref, surah number, surah name, category filter
- collections: create, save ayah/pair, remove item, browse
- dashboard: quick resume links, recent collections, recent saved items
- admin relation management: add, edit, delete, category filter, word picker for highlights
- admin protected by a separate cookie-based session initialized from
  `HIFZLINK_ADMIN_USER` and `HIFZLINK_ADMIN_PASS`
- tafsir display on ayah pages: collapsible section for `lang=en` (Ibn
  Kathir) and `lang=id` (Kemenag RI)
- SEO metadata, canonical URLs, structured data, sitemap, `robots.txt`, and
  `llms.txt`
- private and administration pages excluded from indexing

## Branch model

- Feature branches target `staging` through pull requests.
- `staging` is the integration branch and local verification target.
- `main` is the production branch and release-source branch.
- the legacy `production` branch is archived under
  `archive/production-2026-08-22` and is no longer an active branch
- `qf-api` is historical and is not part of the active workflow.

Develop and test locally on feature branches based on `staging`. Promote the
verified `staging` commit to `main` through a pull request. The local
`production/` worktree tracks `main`.

See [`docs/GIT-WORKFLOW.md`](./GIT-WORKFLOW.md) for the release procedure and
migration record.

## Data and scripts

- Arabic import: `go run ./scripts/import`
- Translation import: `go run ./scripts/import_translations`
- Translation validation: `go run ./scripts/validate_translations` (use
  `-report` for per-language coverage)
- Dataset validation: `go run ./scripts/validate`
- Relation seed: `go run ./scripts/seed_relations`

Local data files:

- `data/quran.json`
- `data/translations/en.json`
- `data/translations/id.json`
- `data/tafsir/id.kemenag.json`
- `data/tafsir/en.ibn-kathir.json`
- `data/relations.seed.json`
- `data/relations.db` (generated locally)

The seed contains 283 unique curated mutashabihat pairs. The separate
`quran-mutashabihat` repository tracks curated, pending, and dropped dataset
entries. Only curated pairs belong in the application seed.

## Known gaps

- Additional search facets remain deferred until the curated relation set
  requires them.
- The external dataset still contains pending pairs that need human review.

## Important decisions

- Quran text, translations, tafsir, and relations use local-first data.
- Account login, bookmark synchronization, audio, and selected content features
  use Quran Foundation APIs when configured.
- Quran text source: Tanzil
- translation sources:
  - English: Quran.com default verse-route translation (Clear Quran / Dr.
    Mustafa Khattab)
  - Indonesian: `rioastamal/quran-json` (Kemenag-based source)
- Arabic text is always primary; translations are secondary and shown beneath.
- Minimal dependencies: Go standard library and a SQLite driver.
- Each relation uses one confusion-pattern category. Multi-tag support remains
  deferred.

## Quick verification

```bash
go test ./...
go run ./scripts/validate
go run ./cmd/server
```

Manual smoke URLs:

- `/`: landing page
- `/search?q=60:8`: search by ayah ref
- `/search?q=60`: search by surah number
- `/search?q=mumtahanah`: search by surah name
- `/ayah/60/8?lang=en`
- `/compare?ayah1=60:8&ayah2=60:9&lang=id`
- `/surah/60?lang=ar`
- `/juz/28?lang=ar`
- `/admin/relations?lang=ar` (requires Basic Auth)
- `/collections?lang=ar`
- `/dashboard?lang=ar`

## Handoff notes for other agents

Start with these files in order:

1. `docs/PROJECT.md`
2. `docs/ARCHITECTURE.md`
3. `docs/DESIGN.md`
4. `docs/TRANSLATIONS.md`
5. `docs/ROADMAP.md`

Then run:

```bash
go run ./scripts/validate
go run ./cmd/server
```
