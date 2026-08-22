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
- `production` is the legacy release branch. Keep it frozen and do not create
  new commits there.
- `qf-api` is historical and is not part of the active workflow.

The production VPS checkout uses `main`. The legacy `production` branch is not
a deployment source. The local `production/` worktree remains on the legacy
branch until its uncommitted changes are preserved and it can be switched
safely.

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

## Release from `main`

After staging verification:

1. Merge the verified `staging` pull request into `main`.
2. Confirm the production binary matches the `main` source commit.
3. Deploy the binary to the production VPS.
4. Verify `https://hifz.click`.
5. Tag the release and update `CHANGELOG.md`.

Do not edit application source directly on the VPS. Emergency changes require
explicit approval and must be synchronized back to Git immediately.

## Migration record and remaining cleanup

The VPS migration is complete:

1. Merge the verified application state into `main` through a pull request.
2. Set `main` as the GitHub default branch.
3. Switch the production VPS checkout from `production` to `main` through the
   normal deployment procedure.
4. Verify the binary, service, and `https://hifz.click`.

Remaining cleanup:

- Configure GitHub branch protection for `main` and `staging`. The project
  workflow requires pull requests even before those settings are configured.
- Preserve local changes, then update the local `production/` worktree to
  `main`.
- Archive or delete the legacy `production` branch only after explicit
  approval.
