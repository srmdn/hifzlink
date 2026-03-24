# Mutashabihat Curation Workflow

This document explains how to add, verify, and submit mutashabihat pairs to `data/relations.seed.json`.

## What Is a Mutashabihat Pair?

A mutashabihat pair is two Quran verses that are nearly identical in wording, making them easy to confuse during recitation or memorization. The confusion is linguistic, not thematic — the verses must share enough wording that a memorizer could plausibly mix them up.

A pair is a good candidate if:
- A memorizer who knows one verse could accidentally recite the other
- The difference is meaningful (a changed word changes the ruling, the subject, or the meaning)
- The similarity is in Arabic wording, not just in topic or theme

## File Format

Each entry in `data/relations.seed.json` is a JSON object:

```json
{
  "ayah1": "60:8",
  "ayah2": "60:9",
  "category": "word_swap",
  "note": "Both begin identically but 60:8 says 'lam yuqatilukum' (did not fight you) while 60:9 says 'qatalukum' (fought you), changing the ruling entirely."
}
```

Fields:
- `ayah1` — first ayah reference in `surah:ayah` format (e.g. `2:255`)
- `ayah2` — second ayah reference in `surah:ayah` format
- `category` — one of the confusion-pattern categories below
- `note` — optional; explain what differs and why it matters for memorization

Ordering: put the lower surah number first (`ayah1`). If same surah, put the lower ayah number first.

## Confusion-Pattern Categories

Pick the single category that best describes the difference between the two verses:

| Category | Description | Example |
|---|---|---|
| `lafzi` | Near-identical wording, almost word-for-word | 94:5 and 94:6 (verbatim repeat) |
| `word_swap` | One or more words differ between otherwise identical verses | 60:8 and 60:9 (ruling changes with one verb) |
| `addition_omission` | One verse has an extra word or phrase the other lacks | 17:23 and 29:8 (wa by al-walidayn) |
| `ending_variation` | Verses identical except for the final word or phrase | Many Al-Rahman pairs end differently |
| `order_change` | Same words appear in different sequence | 4:135 and 5:8 (subject order swapped) |
| `pronoun_shift` | Differs only in pronoun (هو/هم, كم/كن, etc.) | 23:5 and 70:29 (pronoun change) |
| `other` | Does not fit cleanly into any above pattern | Use sparingly |

When the difference spans more than one category, pick the dominant one. If unsure, use `other` and explain in the note.

## Verification Checklist

Before submitting a pair:

1. **Verify both references** — open both ayahs in a Quran app or mushaf and confirm the surah:ayah numbers are correct.
2. **Read both ayahs in Arabic** — confirm they are genuinely similar in wording, not just in topic.
3. **Pick the right category** — the category should match the primary type of difference.
4. **Write a useful note** (optional but recommended) — describe what specifically differs and why a memorizer would mix them up. Keep it to 1-2 sentences.
5. **Check for duplicates** — search `relations.seed.json` for both ayah refs before adding.

## Adding Pairs via the Admin UI

If you are running hifzlink locally, you can also add pairs through the admin interface at `/admin/relations`. Pairs added through the UI go into `data/relations.db`, not `relations.seed.json`. To include them in the seed:

1. Add the pair via the admin UI to test it
2. Once verified, add the same pair to `data/relations.seed.json` manually
3. Delete it from the DB and re-run `go run ./scripts/seed_relations` to reload from the seed file

## Common Mistakes

- **Wrong ayah number** — off-by-one errors are common. Always verify in a mushaf.
- **Thematic similarity only** — two ayahs about the same topic are not mutashabihat unless the Arabic wording is also nearly identical.
- **Duplicate directions** — `60:8 ↔ 60:9` and `60:9 ↔ 60:8` are the same pair. Always put the lower surah first.
- **Single-surah sequential verses** — consecutive ayahs in the same surah are rarely confusing in isolation. Prefer cross-surah pairs or pairs separated by many verses.

## Submitting a Contribution

1. Fork the repo and create a branch: `data/add-mutashabihat-pairs`
2. Edit `data/relations.seed.json` following the format above
3. Run `go run ./scripts/validate` to check the dataset
4. Run `go run ./scripts/seed_relations` to load the seed into a local DB
5. Open the compare page for each pair you added to visually verify the diff highlighting
6. Open a pull request with a short description of the pairs added and your verification method

For questions about whether a pair qualifies, open an issue with both ayah refs and your reasoning.
