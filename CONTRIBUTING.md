# Contributing

Thank you for contributing to HifzLink.

## Development Setup

1. Clone the repo.
2. Run the app:

```bash
go run ./cmd/server
```

3. Run checks before opening a PR:

```bash
gofmt -w ./cmd ./internal
go test ./...
```

## Contribution Scope

Good first contributions:

- improve verse relation browsing
- improve readability/accessibility of Arabic text UI
- add tests for parser, relation logic, and handlers
- improve docs and examples

## Pull Request Rules

- keep changes focused and small
- explain user impact in PR description
- update docs when behavior changes
- update `CHANGELOG.md` for user-visible changes

## Quran Text Data Rules

When working with `data/quran.json`:

- do not alter Quran text content
- preserve UTF-8 Arabic text and diacritics
- include source attribution updates in `NOTICE.md`

Required validation before merge:

- `6236` ayah records
- unique `(surah, ayah)`
- non-empty `text_ar`

## Commit Message Guidance

Use concise imperative messages, for example:

- `Add surah names to API response`
- `Fix compare page ayah heading`
- `Document dataset attribution requirements`
