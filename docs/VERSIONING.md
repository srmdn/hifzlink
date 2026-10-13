# Product Versioning

HifzLink uses Semantic Versioning for meaningful product releases. It is a
website, so routine deployments do not each need a product version or GitHub
Release.

Use `MAJOR.MINOR.PATCH`:

- `MAJOR`: a stable `1.0.0` release or a later breaking change.
- `MINOR`: a notable feature or substantial user-facing change. Before `1.0.0`,
  breaking changes may also be included in a minor release.
- `PATCH`: an important bug or security fix that does not add a notable
  capability.

Small visual polish, minor features, and internal maintenance do not need a
version bump or GitHub Release. Record user-visible changes under
`[Unreleased]` in `CHANGELOG.md`. Keep the exact source commit identifiable by
its Git SHA for every production deployment.

## Preparing a product release

When changes merit a product release:

1. Before promoting the verified changes from `staging` to `main`, finalize the
   matching `CHANGELOG.md` section with the version and release date.
2. After the changes reach `main`, create a Git tag matching the version, such
   as `v0.5.0`, on that verified source commit.
3. Publish a GitHub Release for that tag using the changelog section as its
   release notes.

For routine deployments, retain the `[Unreleased]` changelog entries and do not
create a semantic version tag or GitHub Release.
