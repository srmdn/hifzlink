# Project Status

Last updated: 2026-03-16 (session 4)

## Current State

HifzLink is in MVP+ stage:

- full Quran Arabic dataset loaded locally (`6236` ayahs)
- local SQLite relation storage and APIs working
- server-rendered pages for home, ayah, compare, surah index, juz index
- EN/ID translation toggle implemented (`ar`, `en`, `id`)
- translation text renders below each ayah when enabled
- redesigned UI foundation applied (tokens, typography, top bar, language toggle)
- custom project favicon added
- docs reorganised into `docs/` folder
- unit and handler tests added (`go test ./...` passes)
- friendly not-found page for invalid/missing ayah references
- inline search validation message on home page (invalid format redirects back with error)
- shared template partials extracted (`_partials.html`: head, topbar, footer)
- `lang="ar"` added to Arabic text elements for screen reader accuracy
- footer added to all pages with Tanzil attribution
- homepage cleaned up: dead class removed, API card removed, Quick Links → Examples
- compare page back link points to ayah1 instead of generic Home
- surah/juz pair lists improved with Compare button per pair

## Implemented Features

- ayah lookup (`GET /api/ayah/{surah}/{ayah}`)
- related ayah lookup (`GET /api/ayah/{surah}/{ayah}/relations`)
- add relation (`POST /api/relations`)
- relations by surah (`GET /api/surah/{surah}/relations`)
- relations by juz (`GET /api/juz/{juz}/relations`)
- compare page with side-by-side ayahs
- language mode persistence via `?lang=` query parameter

## Data And Scripts

- Arabic import: `go run ./scripts/import`
- Translation import: `go run ./scripts/import_translations`
- Dataset validation: `go run ./scripts/validate`
- Relation seed: `go run ./scripts/seed_relations`

Local data files:

- `data/quran.json`
- `data/translations/en.json`
- `data/translations/id.json`
- `data/relations.seed.json`
- `data/relations.db` (generated locally)

## Known Gaps

- translation coverage checks not exposed as a report command (Milestone 1 item 2 — still pending)
- no admin UI for relation management; API-only create (Milestone 2)
- no pagination/filtering for large relation lists
- no production deployment docs yet

## Important Decisions

- local-first architecture (no runtime external API dependency)
- Quran text source: Tanzil
- translation source currently imported from Tanzil endpoints (`en.sahih`, `id.indonesian`)
- Arabic text is always primary; translations are secondary and shown beneath ayah text
- minimal dependencies; Go standard library + SQLite driver only

## Quick Verification

```bash
go test ./...
go run ./scripts/validate
go run ./cmd/server
```

Manual smoke URLs:

- `/ayah/60/8?lang=ar`
- `/ayah/60/8?lang=en`
- `/ayah/60/8?lang=id`
- `/compare?ayah1=60:8&ayah2=60:9&lang=id`

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
