# CLAUDE.md

## Project
hifzlink: open source Quran murojaah tool to find and compare mutashabihat verses.
Go standard library + SQLite + minimal HTML/CSS/JS frontend. Live at hifz.click.

## Stack
- Backend: Go, `net/http` (standard library only), SQLite
- Frontend: plain HTML, CSS, minimal JavaScript (no frameworks, no npm)
- Database: SQLite at `data/hifzlink.db`

## Repo visibility: PUBLIC (open source)
This repo is public. CLAUDE.md and all docs must contain no server IPs,
real domains (beyond the public hifz.click), internal paths, or personal
infrastructure references. Use placeholders in all examples.

## Environment: LOCAL DEV ONLY

## Conventions
- Zero external backend dependencies: Go standard library only
- No Gin, Echo, Fiber, or ORM libraries
- No React, Vue, Angular, Next.js, or npm in the frontend
- Secrets in `.env`: never committed
- `.env.example` committed with all variable names, no real values
- Keep commits small: one logical change per commit

## Quran data rules
- Quran text loaded from `data/quran.json`: never alter the text content
- Preserve UTF-8 Arabic text and diacritics
- Keep attribution and source notes in `NOTICE.md`
- Validate dataset integrity: exactly 6236 ayah records, unique (surah, ayah) pairs, non-empty text_ar

## Testing
Run before every commit: `go test ./...` (from repo root)
All tests must pass before committing.
Write tests for new code in the same commit.

## Writing Conventions
- No em dashes (`—`) in commit messages, docs, README, or any written output.
- Use a colon, semicolon, or rewrite the sentence instead.

## Security Rules
- No hardcoded credentials or tokens in source code
- SQL queries must use parameterized statements
- Review AI-generated code for security issues before committing

## Do not modify without confirming
- Quran dataset files (`data/quran.json`, `NOTICE.md`)
- Database migration files
