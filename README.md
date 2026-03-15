# HifzLink (Quran Murojaah)

Simple Go web app to help memorizers review similar Quran verses (mutashabihat).

## Stack

- Go (`net/http`)
- SQLite (relations only)
- server-rendered HTML templates
- local JSON Quran dataset (`data/quran.json`)

## Run

```bash
go run ./cmd/server
```

Server listens on `http://localhost:8080`.

## Quran Dataset Workflow

Import full dataset from Tanzil into `data/quran.json`:

```bash
go run ./scripts/import
```

Validate dataset integrity:

```bash
go run ./scripts/validate
```

Import English and Indonesian translations:

```bash
go run ./scripts/import_translations
```

Seed starter relation pairs:

```bash
go run ./scripts/seed_relations
```

## API

- `GET /api/ayah/{surah}/{ayah}`
- `GET /api/ayah/{surah}/{ayah}/relations`
- `POST /api/relations`
- `GET /api/surah/{surah}/relations`
- `GET /api/juz/{juz}/relations`

Add relation example:

```bash
curl -X POST http://localhost:8080/api/relations \
  -H 'Content-Type: application/json' \
  -d '{"ayah1":"60:8","ayah2":"60:9","note":"mutashabihat"}'
```

## Open Source Docs

- [CONTRIBUTING.md](./CONTRIBUTING.md)
- [VERSIONING.md](./VERSIONING.md)
- [CHANGELOG.md](./CHANGELOG.md)
- [LICENSE](./LICENSE)
- [NOTICE.md](./NOTICE.md)
- [DESIGN.md](./DESIGN.md)
- [TRANSLATIONS.md](./TRANSLATIONS.md)

## Project Structure

- `cmd/server/main.go` HTTP server + routes
- `internal/search` Quran dataset loader and ayah lookup
- `internal/db` SQLite storage for verse relations
- `internal/relations` relation service and ayah parser
- `web/templates` server-rendered pages
- `web/static` CSS
- `data/quran.json` local Quran dataset
- `data/relations.seed.json` starter relation pairs
- `scripts/import` imports full Quran text + metadata from Tanzil
- `scripts/import_translations` imports full `en/id` translations from Tanzil
- `scripts/validate` validates dataset contract
- `scripts/seed_relations` seeds initial mutashabihat relation examples
