# Translation Architecture

This document defines how English/Indonesian translations are added while preserving Arabic-first reading.

## Scope

Supported languages:

- `en` (English)
- `id` (Indonesian)

Arabic remains mandatory in all modes.

## UX Rule

Translation is always rendered under the corresponding Arabic ayah.

Never replace Arabic with translation.

## Data Model

Arabic source (existing):

- `data/quran.json`

Translation sources (new):

- `data/translations/en.json`
- `data/translations/id.json`

Current import source IDs:

- Quran.com verse-route Next data (`/_next/data/{build}/{surah}:{ayah}.json`) default English translation
  (currently shown as Dr. Mustafa Khattab, The Clear Quran)
- `rioastamal/quran-json` surah files for Indonesian translation text

Tafsir-ready output:

- `data/tafsir/id.kemenag.json` (ayah-keyed Indonesian tafsir text, imported from `rioastamal/quran-json`)
- `data/tafsir/en.ibn-kathir.json` (ayah-keyed English tafsir text, imported from Quran.com API tafsir resource `169`)

Recommended translation record format:

```json
{
  "key": "60:8",
  "text": "Allah does not forbid you..."
}
```

Alternative map format is also acceptable:

```json
{
  "60:8": "Allah does not forbid you..."
}
```

## Runtime Loading

At server startup:

1. load `data/quran.json` into memory
2. load `en.json` and `id.json` into memory maps keyed by `surah:ayah`
3. fail fast on invalid translation file format

Import command:

```bash
go run ./scripts/import_translations
```

Missing translation entries should not crash the app.
Fallback behavior:

- show Arabic only
- optionally show small `Translation unavailable` note

## API Behavior

Existing endpoints stay compatible.

Add optional query parameter:

- `?lang=en`
- `?lang=id`

When `lang` is set and translation exists, include:

- `translation_lang`
- `translation_text`

Example response shape:

```json
{
  "surah": 60,
  "surah_name": "Al-Mumtahanah",
  "ayah": 8,
  "text": "...Arabic...",
  "translation_lang": "id",
  "translation_text": "Allah tidak melarang kamu...",
  "juz": 28
}
```

For related ayah responses, include translation per related entry when requested.

## UI State

Language mode should be stored in one of:

- query param (`lang=en|id|ar`)
- cookie
- both (query param takes precedence)

Suggested default:

- `ar` (Arabic only)

## Rendering Contract

For each ayah component:

1. render Arabic text
2. if mode is `AR + EN` or `AR + ID`, render translation directly below Arabic
3. keep reference/actions below text block

## Sources And Licensing

Before importing translations:

1. select translation source with clear reuse terms
2. add attribution and license details in `NOTICE.md`
3. document source/version/date in PR description

## Validation Checklist

For each translation file:

- valid JSON
- keys in `surah:ayah` format
- no duplicate keys
- no empty translation strings
- optional coverage report by percent of `6236`

## Rollout Plan

1. add translation loader + in-memory maps
2. add `lang` support in API and page handlers
3. add UI language toggle
4. render translation under ayah blocks
5. add tests for loader, API, and UI handler behavior
