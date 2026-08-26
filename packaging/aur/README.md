# AUR package

`termagitchi-bin` — installs the prebuilt Linux binary from the
GitHub release, so no Go toolchain is needed on the user's machine.

## First-time publish

Needs an AUR account and the SSH key registered against it, which is why this
is not automated.

1. Create an account at https://aur.archlinux.org/register and add your public
   key under My Account → SSH Public Key.
2. Clone the (empty) package repo and copy these two files in:

   ```sh
   git clone ssh://aur@aur.archlinux.org/termagitchi-bin.git
   cd termagitchi-bin
   cp /path/to/claude-buddy/packaging/aur/{PKGBUILD,.SRCINFO} .
   git add PKGBUILD .SRCINFO
   git commit -m "Initial import: termagitchi-bin 0.2.3"
   git push
   ```

`.SRCINFO` must be committed alongside `PKGBUILD` — the AUR rejects pushes
without it, and it must agree with the PKGBUILD or the package shows wrong
metadata.

## On every release

Bump `pkgver`, reset `pkgrel=1`, and replace both checksums with the ones from
that release's `checksums.txt`:

```sh
curl -sL https://github.com/TevvvB/termagitchi/releases/download/vX.Y.Z/checksums.txt | grep linux
```

Take the hashes from the published `checksums.txt` rather than hashing a local
build. The build is not byte-reproducible, so a locally generated hash matches
nothing anyone downloads and every install fails validation.

On an Arch machine, regenerate `.SRCINFO` properly rather than editing it:

```sh
makepkg --printsrcinfo > .SRCINFO
```

## Checked

- Archive layout confirmed: `LICENSE`, `README.md`, `pets` at the archive root,
  so no `_srcdir` subdirectory handling is needed.
- `options=('!strip')` because the Go binary is already stripped by GoReleaser
  and stripping again is wasted work.
- `provides`/`conflicts` cover `pets` so this cannot be co-installed with a
  future source-built package of the same tool.
