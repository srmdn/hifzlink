# Project Status

Last updated: 2026-03-24 (session 6)

## Current State

HifzLink is past MVP, Milestone 2 complete. The public-facing site is fully functional and styled for open source use.

- full Quran Arabic dataset loaded locally (`6236` ayahs)
- local SQLite relation storage working with migration system
- server-rendered pages: home, ayah, compare, surah index, juz index, search, collections, dashboard, admin
- EN/ID translation toggle implemented (`ar`, `en`, `id`)
- landing page redesigned as SaaS-style public page (hero, story, diff example, features, how-it-works, browse CTA)
- topbar restructured: Search links to `/search`; Dashboard hidden until auth is implemented
- CSS split into focused files: `base.css`, `topbar.css`, `components.css`, `admin.css`, `pages.css`
- responsive button system (`.btn`, `.btn-sm`, `.btn-outline`, `.btn-danger`) with mobile touch targets
- full mobile layout pass: hero centering, search row stacking, diff example collapse, consistent top spacing
- search page at `/search` supports ayah ref, surah number, surah name, and category filter
- compare page shows related pairs (all pairs sharing either ayah) instead of sequential prev/next
- category taxonomy revised to confusion-pattern only: `lafzi`, `addition_omission`, `word_swap`, `ending_variation`, `order_change`, `pronoun_shift`, `other`
- old thematic category values migrated to `other` on startup via DB migration
- admin auth auto-loaded from `.env` at startup (no shell export needed for local dev)
- em dashes removed from all visitor-facing templates; replaced with natural sentence structure
- unit and handler tests passing (`go test ./...`)

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
- admin protected by HTTP Basic Auth (`HIFZLINK_ADMIN_USER` / `HIFZLINK_ADMIN_PASS`)

## Data And Scripts

- Arabic import: `go run ./scripts/import`
- Translation import: `go run ./scripts/import_translations`
- Translation validation: `go run ./scripts/validate_translations` (use `-report` for per-language coverage)
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

## Known Gaps

- `relations.seed.json` is minimal — curating a real starter set is M3 work
- no production deployment docs yet
- faceted filters beyond category (surah, juz, has_note) deferred
- account/auth system deferred to post-MVP

## Important Decisions

- local-first architecture (no runtime external API dependency)
- Quran text source: Tanzil
- translation sources:
  - English: Quran.com default verse-route translation (Clear Quran / Dr. Mustafa Khattab)
  - Indonesian: `rioastamal/quran-json` (Kemenag-based source)
- Arabic text is always primary; translations are secondary and shown beneath
- minimal dependencies: Go standard library + SQLite driver only
- single confusion-pattern category field per relation (multi-tag deferred)

## Quick Verification

```bash
go test ./...
go run ./scripts/validate
go run ./cmd/server
```

Manual smoke URLs:

- `/` — landing page
- `/search?q=60:8` — search by ayah ref
- `/search?q=60` — search by surah number
- `/search?q=mumtahanah` — search by surah name
- `/ayah/60/8?lang=en`
- `/compare?ayah1=60:8&ayah2=60:9&lang=id`
- `/surah/60?lang=ar`
- `/juz/28?lang=ar`
- `/admin/relations?lang=ar` (requires Basic Auth)
- `/collections?lang=ar`
- `/dashboard?lang=ar`

## Handoff Notes For Other Agents

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
