# Mutashabihat Curation Workflow

This document explains how to add, verify, and submit mutashabihat pairs to `data/relations.seed.json`.

## What Is a Mutashabihat Pair?

A mutashabihat pair consists of two Quran verses with shared wording that may cause confusion during recitation or memorization. Similarity may cover the complete verses or a recurring phrase. The confusion is linguistic, not thematic — the verses must share enough wording that a memorizer could plausibly mix them up.

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

## Wording categories

Use one primary category. These labels describe observable wording patterns,
not a judgment about tafsir or the scholarly completeness of the catalog.

| Key | Meaning |
|---|---|
| `lafzi` | Identical or nearly identical **complete verses**, not just a shared phrase |
| `addition_omission` | One complete sequence retained with a small addition |
| `word_swap` | A few word substitutions in otherwise matching verses |
| `ending_variation` | Matching opening, differing final phrase |
| `order_change` | Same complete word multiset, different order |
| `pronoun_shift` | Pronoun change identified by a human reviewer |
| `shared_opening` | A common phrase starts both verses; other wording differs |
| `shared_phrase` | Common phrase in the middle or at different positions |
| `shared_ending` | Common closing phrase; other wording differs (2:114 ↔ 5:41) |
| `structural` | Sentence-pattern resemblance identified by a human reviewer |
| `other` | No clear pattern; assess whether the pair helps memorization |

### Automatic rules (`wording-v2`)

The single analyzer is `internal/curation`, also used by the dashboard and the
`scripts/analyze_curation` command. It reads the versioned Arabic without editing
it. It ignores stop signs and folds common alif/mark variants for word alignment.
The separate highlight comparison retains vocalization and spelling differences.

Apply these rules in order:

1. Full normalized word sequences match → `lafzi` (strong evidence only if the
   vocalization/spelling also matches after ignoring stop signs and tatweel).
2. One entire ordered sequence is contained in the other and covers at least 80%
   of **both** verses → `addition_omission`.
3. Complete word multisets match → `order_change`.
4. Equal lengths, at most two substitutions, at least 80% identical positions →
   `word_swap`.
5. At least 90% of each complete verse aligns in order → `lafzi`.
6. An identical prefix of at least four words covers 80% of each verse →
   `ending_variation`.
7. Longest contiguous shared phrase has at least four words → `shared_opening`
   if it starts both, `shared_ending` if it ends both, otherwise `shared_phrase`.
8. Otherwise → `other`. The analyzer does not infer grammatical structure or
   pronoun meaning from overlap alone.

The phrase search reads full verses; the detector's former nine-word cap does
not constrain classification. Coverage means the proportion of normalized word
**occurrences** aligned in sequence, not semantic similarity or a confidence
probability. Repeated formulaic phrases can still be poor memorization examples;
automatic labels remain provisional.

### Highlight editing

Maintain one set of selected differing words in one color. Matching wording is
unmarked. Waqaf/section signs alone are never automatically highlighted;
vocalization differences remain visible. All saved indices address the original
whitespace-separated words, including original stop-sign positions. Arabic text
and diacritics are never rewritten. An explicit empty selection means no highlights.

### Reclassifying existing automatic relations

The companion dataset repository provides `refine_hifzlink_curation.py`. Use
an explicitly scoped, consistent curation snapshot outside Git. Build the analyzer:

```sh
go build -o /tmp/hifzlink-analyze ./scripts/analyze_curation
```

Use the dataset script's `plan --help`, then inspect its private before/after plan.
The plan includes all eligible rows, category transitions, full-verse coverage,
human-decision conflicts, and original values. Apply it only to an explicit local
copy. `apply` first saves a SQLite backup, checks a fingerprint of every table,
and uses one transaction. A stale database fails closed.

Only accepted relations with automatic source, reviewed=false, and **no past human
review metadata** are eligible. Existing manual/seed relations and human-reviewed
or reopened candidates are preserved. Highlights differing from the previous
automatic algorithm are preserved as possible manual edits. Only public category,
automatic highlights, modification time, and private suggestion/automation notes
are changed. Public notes, review flags, IDs, origins, and account state are retained.

`export` regenerates versioned candidate suggestion JSON from the same analyzer,
without private notes or human review metadata. Startup imports only new candidate
pairs; changing that JSON does **not** reclassify existing runtime relations.
Publishing a later refinement requires a fresh production snapshot and a separately
approved, guarded data update. Never replace the live database with a preview copy.

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
3. Validate the seed separately. Preserve runtime relations and review history; a seed difference does not authorize deletion or replacement.

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
