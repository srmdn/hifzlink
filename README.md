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

## Project Structure

- `cmd/server/main.go` HTTP server + routes
- `internal/search` Quran dataset loader and ayah lookup
- `internal/db` SQLite storage for verse relations
- `internal/relations` relation service and ayah parser
- `web/templates` server-rendered pages
- `web/static` CSS
- `data/quran.json` local Quran dataset
