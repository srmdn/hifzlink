# Git workflow

HifzLink uses a pull-request branch workflow. Agents and contributors must use
a short-lived feature branch and a pull request for every code or
documentation change.

## Branch roles

- `codex/*` and other feature branches contain one focused change.
- `staging` is the integration branch. Pull requests from feature branches
  target `staging`.
- `main` is the production branch. Pull requests from `staging` target
  `main` for release.
- `archive/production-2026-08-22` is the archive tag for the former legacy
  `production` branch. Do not recreate that branch.
- `qf-api` is historical and is not part of the active workflow.

`main` is the release-source branch. Operational deployment configuration is
intentionally managed outside this public repository.

## Start a change

Run these commands from the HifzLink repository. Preserve existing local
changes before switching branches.

```bash
git fetch origin --prune
git switch staging
git pull --ff-only origin staging
git switch -c codex/short-description
```

Never commit directly on `main`, `staging`, or `production`. Never push
directly to those branches.

## Verify a change

Run the relevant checks from the feature branch before opening a pull request:

```bash
gofmt -w ./cmd ./internal
go test ./...
go run ./scripts/validate
go run ./scripts/validate_translations
go build ./cmd/server
```

Run the local preview at `http://127.0.0.1:18088` for user-visible changes.

## Open and merge pull requests

Push the feature branch and open a pull request to `staging`:

```bash
git push -u origin codex/short-description
```

Merge only after review and required checks pass. After the change is
verified on `staging`, open a second pull request from `staging` to `main`.

## Release source from `main`

After staging verification:

1. Merge the verified `staging` pull request into `main`.
2. Tag the verified source release and update `CHANGELOG.md`.

## Historical branch migration record

This records the completed source-control transition from the legacy
`production` branch to `main`:

1. Merge the verified application state into `main` through a pull request.
2. Set `main` as the GitHub default branch.
3. Configure GitHub branch protection for `main` and `staging`.
4. Archive the legacy source under
   `archive/production-2026-08-22`, then delete the legacy `production` branch.
