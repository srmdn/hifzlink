# Architecture

This document describes the system architecture.

## System Overview

The application consists of three main parts:

1. Quran dataset
2. Go backend server
3. server-rendered frontend

## Quran Dataset

Primary file:

- `data/quran.json`

Loaded into memory at server startup.

Each record:

- `surah`
- `surah_name`
- `ayah`
- `juz`
- `text_ar`

### Data Ingestion Flow

1. source dataset is placed in `data/raw/` (optional workspace)
2. transform/normalize into project schema
3. write output to `data/quran.json`
4. run validation checks
5. commit generated dataset and attribution updates

### Validation Rules

- total records: `6236`
- unique key: `(surah, ayah)`
- non-empty Arabic text for `text_ar`
- valid surah range: `1..114`

## Backend

The backend is a Go HTTP server.

Responsibilities:

- load Quran dataset
- handle API requests
- store verse relations
- serve HTML pages

## Database

SQLite database.

Stores only verse relations.

Table: `relations`

Fields:

- `id`
- `ayah1_surah`
- `ayah1_ayah`
- `ayah2_surah`
- `ayah2_ayah`
- `note`

## Folder Structure

- `cmd/server/main.go`
- `internal/db`
- `internal/relations`
- `internal/search`
- `web/templates`
- `web/static`
- `data/quran.json`

## Request Flow

Example: user searches for verse.

Request:

- `GET /ayah/60/8`

Server flow:

1. find verse in in-memory dataset
2. find relations in SQLite
3. render HTML page

## Compare Mode

Compare view renders two verses side-by-side.

Used to reduce confusion for similar wording.

## Performance

The Quran dataset is small enough for full in-memory loading.

This keeps verse lookup fast and simple.

## Deployment

The server runs with one command:

```bash
go run ./cmd/server
```

No additional services required.

## Design Philosophy

Keep everything simple.

Avoid unnecessary dependencies.

Prioritize clarity, maintainability, and accurate Quran text handling.
