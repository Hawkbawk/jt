# Releasing

Releases are published automatically when a version bump merges to `main`. You never tag by hand.

## Cut a release

1. Bump `version` in the `let` block of `flake.nix` (for example `"0.2.0"`). This is the only place the version lives.
2. Open a PR and get it approved. Merging it is the release decision.
3. After the merge, the [Release workflow](.github/workflows/release.yml) runs on `main`. It publishes the release only if `v<version>` doesn't already have one.

Merges that don't change the version are no-ops for the workflow.

## What the workflow does

- Runs `go test ./...`.
- Builds tarballs for linux-amd64, linux-arm64, and darwin-arm64, with the version baked in for `jt --version`, plus a `checksums.txt`.
- Creates a draft release with generated notes, attaches the files, then publishes it. Publishing creates the `v<version>` tag at the merged commit.
- Marks the release as a prerelease if the version has a suffix, such as `0.2.0-rc1`.

## Immutable releases

Immutable releases must be enabled in the repo settings (Settings → General → Releases). Once a release is published, its tag and assets can't change, and the tag name can't be reused even if the release is deleted. To fix a bad release, bump the version and ship a new one.

## Verify

- The release appears on the [releases page](https://github.com/Hawkbawk/jt/releases) with three tarballs and `checksums.txt`.
- `nix run github:Hawkbawk/jt/v<version> -- --version` prints the new version.
- Compare versions at `https://github.com/Hawkbawk/jt/compare/v<old>...v<new>`.
