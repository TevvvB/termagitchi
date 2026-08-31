# Releasing

First bump the plugin's version in both manifests, then tag:

```sh
# plugin/.claude-plugin/plugin.json and .claude-plugin/marketplace.json (two places in that one)
git tag -a vX.Y.Z -m "…" && git push origin vX.Y.Z
```

The release workflow builds every platform and publishes the archives and checksums.

**The plugin bump is not optional and the workflow enforces it.** Claude Code only offers
an update when `plugin.json`'s `version` changes, so leaving it behind pins every plugin
user on whatever they installed. It drifted three releases before anyone noticed, so the
release now fails on a mismatch rather than trusting the habit. If it fails, fix the
manifests, delete the tag, and tag again.

## The Homebrew cask and the Scoop manifest update themselves

[The tap](https://github.com/TevvvB/homebrew-tap) carries a workflow that polls
this repository's latest release every six hours, and can be run on demand:

```sh
gh workflow run update-casks.yml --repo TevvvB/homebrew-tap
gh workflow run update-manifests.yml --repo TevvvB/scoop-bucket
```

Each regenerates its manifest from the release's `checksums.txt` and pushes.

**No token is involved.** A workflow in this repository could not write to the
tap without a personal access token, but a workflow in the tap can write to the
tap with its own `GITHUB_TOKEN`. Pulling from the release rather than pushing to
the tap removes the secret entirely, which is why `.goreleaser.yaml` keeps
`skip_upload` set on both the cask and the scoop manifest: goreleaser must not
push to either, because each one pulls.

It also makes the checksum rule structural rather than a thing to remember: the
updater has no local build to take hashes from, only the published
`checksums.txt`. That matters because the build is not byte-reproducible, so
locally generated hashes match nothing anyone downloads and every `brew install`
would fail verification.

## Platforms

Homebrew casks are macOS only; the [Scoop
bucket](https://github.com/TevvvB/scoop-bucket) covers Windows. Linux users take a
release archive or `go install`. The Linux blocks GoReleaser writes into the cask are unreachable,
and harmless.

## Checks

The tap runs `brew audit --cask --strict` on a macOS runner on every push, then
installs the cask and runs the binary, so an audit cannot pass while the thing
it describes is unusable.
