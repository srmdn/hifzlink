# Agent Instructions

Keep repository-specific guidance here short. Follow the linked document for
details relevant to the task.

## Project boundaries

- Start application changes from a short-lived branch based on `staging`. Use
  pull requests to promote work to `staging`, then verified work to `main`.
  Never commit directly to `staging` or `main`, and do not recreate the
  archived `production` branch. See [the Git workflow](docs/GIT-WORKFLOW.md).
- Keep secrets, `.env` files, live databases, and local snapshots out of this
  public repository. Private runtime state belongs outside the source tree.
- Do not change canonical Quran Arabic text, diacritics, or attribution without
  explicit approval and the required validation. Only curated pairs belong in
  this repository's seed; do not change live relations based only on a seed
  difference.

## Task guidance

- Follow [the architecture](docs/ARCHITECTURE.md) for the stack, structure, and
  UI constraints.
- Follow [contributor guidance](CONTRIBUTING.md) for change scope, checks,
  commit messages, attribution, and changelog updates.
- Follow [the versioning policy](docs/VERSIONING.md) when preparing a product
  release. A production source commit must remain identifiable even when it is
  not a versioned product release.
- For Quran pair curation, work in the separate mutashabihat data repository
  and follow its local instructions.

If `AGENTS.local.md` exists, read it for machine-specific guidance. Keep that
file untracked.
