# AI Agent Instructions

This repository may be modified by AI coding tools.

Follow these rules strictly.

## Do Not Overengineer

Avoid unnecessary complexity.

Do not introduce frameworks unless explicitly required.

## Backend Rules

Backend must use:

- Go
- Go standard library
- `net/http`
- SQLite

Do not add:

- Gin
- Echo
- Fiber
- ORM libraries

## Frontend Rules

Frontend must remain simple.

Allowed:

- HTML
- CSS
- minimal JavaScript

Do not add:

- React
- Vue
- Angular
- Next.js
- Node.js
- npm

## Architecture Rules

Follow the project structure defined in `ARCHITECTURE.md`.

Do not change folder layout without good reason.

## Code Quality

Code must be:

- readable
- minimal
- well structured

Avoid large abstractions.

Prefer simple functions.

## Data Handling Rules

Quran text must be loaded from `data/quran.json`.

The database stores only verse relations.

For Quran dataset updates:

- do not alter Quran text content
- preserve UTF-8 Arabic text and diacritics
- keep attribution and source notes in `NOTICE.md`
- validate dataset integrity before committing

Required dataset checks:

- exactly `6236` ayah records
- unique `(surah, ayah)` pairs
- non-empty `text_ar`

## UI Rules

Always display full verses.

Never truncate Quran text.

Arabic text must be readable.

## Development Strategy

Implement features incrementally:

1. HTTP server
2. load Quran dataset
3. verse search
4. relations API
5. frontend pages
6. compare view

## Open Source Hygiene

When making user-visible changes:

- update relevant docs (`README.md`, `PROJECT.md`, `ARCHITECTURE.md`)
- follow `CONTRIBUTING.md`
- record user-visible changes in `CHANGELOG.md`

## Priority

The primary goal is usability for Quran memorization.

Keep the interface simple and fast.
