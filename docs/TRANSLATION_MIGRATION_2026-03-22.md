# Translation/Tafsir Migration Report (2026-03-22)

## Summary

This report documents the migration of translation sources in `scripts/import_translations` and the operational issues encountered during implementation.

Final state:

- English translation source: Quran.com verse-route Next data
- Indonesian translation source: `rioastamal/quran-json`
- English tafsir source: Quran API (`resource_id=169`, Ibn Kathir abridged)
- Indonesian tafsir source: `rioastamal/quran-json`

Generated outputs:

- `data/translations/en.json` (`6236`)
- `data/translations/id.json` (`6236`)
- `data/tafsir/en.ibn-kathir.json` (`6236`)
- `data/tafsir/id.kemenag.json` (`6236`)

## Requested Goal

1. Stop using stiff Indonesian translation source and migrate Indonesian translation to `https://github.com/rioastamal/quran-json`.
2. Use English translation aligned with Quran.com default display.
3. Prepare tafsir imports for future use.
4. Keep only one English translation version (Quran.com-based).

## What Failed During Implementation

### Failure 1: DNS/network failures in sandbox

Observed failures:

- `lookup tanzil.net: no such host`
- `lookup quran.com: no such host`

Cause:

- sandbox network/DNS restrictions.

Resolution:

- rerun importer verification with escalated network access.

### Failure 2: Quran.com chapter Next-data only returned preview subset

Attempted path:

- `https://quran.com/_next/data/{build}/{chapter}.json`

Observed behavior:

- Surah 2 response contained only 5 verses (`2:1` to `2:5`) with `totalRecords=5` for that payload context.
- Import aborted with count mismatch (`expected 6236`, got `1200`).

Cause:

- chapter route payload is not a full chapter export endpoint.

Resolution:

- switched to verse-route Next-data per ayah:
  `https://quran.com/_next/data/{build}/{surah}:{ayah}.json`

### Failure 3: intermittent upstream 5xx during bulk English fetch

Observed behavior:

- transient `504 Gateway Timeout` while fetching some ayahs.

Resolution:

- added retry/backoff in HTTP fetch path (5 attempts, incremental delay).

### Failure 4: mid-run build ID rollover causing 404 on `_next/data`

Observed behavior:

- some ayah JSON fetches returned `404` even though verse pages existed.

Cause:

- Next.js build hash changed while importer was running.

Resolution:

- added shared build state and automatic build-id refresh on `404`, then retry.

### Failure 5: English tafsir entries with empty text

Observed behavior:

- importer failed on surah 2 due to empty tafsir text for some ayahs.

Cause:

- upstream tafsir dataset includes some empty values.

Resolution:

- changed tafsir parser validation to accept empty text values while enforcing full key coverage and unique keys.

## Code Changes Made

Main implementation file:

- `scripts/import_translations/main.go`

Key changes:

1. Replaced English source from Tanzil with Quran.com verse-route importer.
2. Kept Indonesian source from `rioastamal/quran-json` for translation + tafsir extraction.
3. Added English tafsir import from Quran API by chapter (`resource_id=169`).
4. Added concurrent worker pipeline for English verse fetch (`12` workers).
5. Added robust fetch retry/backoff for transient upstream failures.
6. Added build-id refresh logic for Next.js `404` recovery.
7. Kept strict output count checks (`6236`) for all generated files.

## Verification Commands And Results

Commands executed:

```bash
go test ./...
go run ./scripts/import_translations
jq 'length' data/translations/en.json
jq 'length' data/translations/id.json
jq 'length' data/tafsir/en.ibn-kathir.json
jq 'length' data/tafsir/id.kemenag.json
```

Results:

- tests passed
- importer completed successfully
- all four datasets returned `6236`

## Notes On Quran.com Source Discovery

What was confirmed:

- Quran.com page payload currently shows default English label:
  `Dr. Mustafa Khattab, The Clear Quran`
- Quran API tafsir resources include English tafsir entries; `169` is available and usable.

What was not used as final import path:

- public `resources/translations` output did not expose a clear `Mustafa Khattab` entry in current responses.
- chapter-route Next-data payload did not provide full-chapter verse lists.

## Final Decision

For reliability and to satisfy the product request, the importer now uses:

- Quran.com verse-route Next-data as the single English translation source
- `rioastamal/quran-json` as the Indonesian translation + tafsir source
- Quran API tafsir-by-chapter (`169`) for English tafsir

This keeps one English translation version in the project and preserves deterministic local outputs.
