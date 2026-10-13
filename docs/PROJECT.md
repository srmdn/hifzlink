# Quran Murojaah

An open source tool to help Quran memorizers track and review similar verses (mutashabihat).

## Project Goal

Many Quran verses contain similar wording. During memorization (murojaah), memorizers often confuse one verse with another similar verse.

This project provides a simple tool to:

- practice sample verse continuations without an account
- browse memorization sections by surah or juz
- search a verse
- view related similar verses
- compare two verses side-by-side or compare several connections against one anchor ayah
- browse similar verses by surah or juz

The goal is to make Quran memorization easier and reduce confusion caused by similar verses.

## Target Users

- Quran memorizers (huffazh)
- students learning Quran memorization
- Quran teachers

## Core Idea

The core idea is verse relations.

Example:

Surah Al-Mumtahanah 8 <-> Surah Al-Mumtahanah 9

The system stores these relations and allows users to quickly explore them.

## Design Principles

This project must remain:

- simple
- lightweight
- easy to run locally
- minimal dependencies
- open source friendly

Avoid overengineering.

## Tech Stack

Backend:

- Go
- Go standard library only
- `net/http`
- SQLite database
- no ORM
- no backend frameworks

Frontend:

- HTML
- CSS
- minimal JavaScript
- server-rendered templates
- no Node.js
- no npm
- no frontend frameworks

## Quran Data Contract

Quran text is stored locally at:

- `data/quran.json`

Each record must include:

- `surah` (int)
- `surah_name` (string)
- `ayah` (int)
- `juz` (int)
- `text_ar` (string, full Arabic text, never truncated)

Example:

```json
{
  "surah": 60,
  "surah_name": "Al-Mumtahanah",
  "ayah": 8,
  "juz": 28,
  "text_ar": "..."
}
```

Dataset acceptance checks:

- total ayah count is `6236`
- `(surah, ayah)` is unique for every record
- all `text_ar` entries are non-empty

## Data Source And Attribution

Preferred source: Tanzil Quran Text.

Requirements when importing Quran text:

- preserve Quran text exactly (no content edits)
- keep source attribution in `NOTICE.md`
- include source and licensing notes in repository docs

## Database Schema

The database stores curated verse relations and user-created collection state.

Table: `relations`

- `id INTEGER PRIMARY KEY`
- `ayah1_surah INTEGER`
- `ayah1_ayah INTEGER`
- `ayah2_surah INTEGER`
- `ayah2_ayah INTEGER`
- `note TEXT`

Example relation:

`60:8 <-> 60:9`

Optional user accounts use Google sign-in. SQLite stores a Google subject ID,
verified email, display name, and expiring HifzLink sessions; OAuth tokens are
not retained. The admin overview reports account metadata and aggregate usage
counts without exposing saved notes or verse text.

## API Design

Get verse:

- `GET /api/ayah/{surah}/{ayah}`

Get related verses:

- `GET /api/ayah/{surah}/{ayah}/relations`

Add relation:

- `POST /api/relations`

Body:

```json
{
  "ayah1": "60:8",
  "ayah2": "60:9",
  "note": "mutashabihat"
}
```

List relations by surah:

- `GET /api/surah/{surah}/relations`

List relations by juz:

- `GET /api/juz/{juz}/relations`

## Frontend Requirements

The UI must prioritize readability for Quran memorization.

Rules:

- always display full verses (never truncated)
- Arabic text must be clear and large
- support side-by-side verse comparison

## Main Pages

Home:

- search for verse (example input: `60:8`)

Ayah Page:

- full verse
- related verses

Compare Page:

- compare two verses side-by-side, or select multiple connected verses from an ayah page
- show the anchor once and highlight wording differences for each connection
- keep translations collapsed by default in multi-verse comparisons

Surah Index:

- list relations inside a surah

Juz Index:

- list relations inside a juz

## MVP Scope

The first version must support:

- search verse
- view full verse
- view similar verses
- compare verses
- add verse relation
- list relations by surah
- list relations by juz

## Future Features

- word difference highlighting
- quiz mode for memorization practice
- automatic detection of similar verses
- public dataset of mutashabihat verses

## Open Source Expectations

This project should be easy to run and easy to contribute to.

- run locally with `go run ./cmd/server`
- follow `CONTRIBUTING.md` for workflow
- use Semantic Versioning as described in `docs/VERSIONING.md`
- keep `CHANGELOG.md` updated for user-visible changes
