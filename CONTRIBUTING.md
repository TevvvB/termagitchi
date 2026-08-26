# Contributing

Two things are easy to contribute and neither needs much Go.

## Adding a creature

Creatures live in one slice in `internal/identity/identity.go`. Adding one is a
single line:

```go
{"newt", "~[", "]~", Uncommon},
```

That is a name, the two fragments that bracket its face, and its rarity band.
`~[` and `]~` around the face `•ᴗ•` render as `~[•ᴗ•]~`.

Rules, all enforced by `go test ./internal/identity/`:

- The name must be unique.
- Neither fragment may be longer than three characters, or rows stop lining up.
- The band must be one of `Common`, `Uncommon`, `Rare`, `Legendary`, `Mythic`.
- The creature must be original. No third-party characters, however loosely.

Rarity odds come from a weight table, not from how many creatures sit in a band,
so adding ten legendaries does not make legendaries common. It only makes each
one rarer to draw.

Please check your creature reads at a glance in a terminal, not just in a diff.
Faces range from `•ᴗ•` down to `x_x`, so try it at both ends.

### The line is small and the blast radius is not

Read this before opening the PR, because the diff looks free and is not.

A creature is picked with `pool[Hash(key) % len(pool)]`, so **growing a band
changes which creature every existing key in that band draws.** Adding one
Common is enough. Measured, from adding a single new Common:

```
For("main")   mouse -> toad
For("master") vole  -> toad
For("trunk")  wren  -> moth
For("a")      crab  -> carp
```

Three consequences, in rising order of how much they matter:

1. Two tests fail, in two packages: the golden pin in `internal/identity` and a
   party test in `internal/render`. That is the pin doing its job, not a mistake
   you made. Repin it in the same PR.
2. Everyone's pet changes when they upgrade. The mouse someone has had in `main`
   for a month becomes a toad.
3. `den.json` keeps the **old** species names, and it is the only precious state
   here. So `collection.Has()` stops matching what is on screen: the den quietly
   re-earns creatures it already holds, and the entries it held before become
   unreachable, because no key summons them any more.

None of that is a reason never to add creatures. It is the reason roster growth
is batched into a release and called out in the notes, rather than merged one
cute animal at a time. If you have a creature you like, open it anyway and say
so - it may just wait for company.

Whether that stays the policy is [an open
question](https://github.com/TevvvB/termagitchi/discussions/46), and
the answer partly depends on how many creatures are waiting. Post yours there
too.

## Adding a signal

A signal is any executable that prints `key=value` lines on stdout. It receives
the worktree path as its first argument. Drop it in `~/.config/pets/signals/`.

```sh
#!/bin/sh
# ~/.config/pets/signals/cargo.sh
echo "clippy=$(cargo clippy --message-format=short 2>&1 | grep -c '^warning')"
```

By default that only adds a row to `pets card`. Give it a bound and a cost under
`[signals.external.penalties.<name>]` and it costs hearts like the built-in signals,
and the card shows it in amber. `over` for things that are bad when they rise,
`under` for things that are bad when they fall.

Signals are user configuration rather than part of the tool, so there is nothing
to submit unless you want one documented in the README as an example.

## Hook fixtures

`cmd/pets/testdata/` holds payloads captured from a real session, not written by
hand. That distinction is the point: a hand-written fixture encodes what its
author believed the harness sends, so a test built on one agrees with the
author's mistake. This project shipped that exact bug once, and every test passed
while the code read escaped JSON source instead of text.

To refresh one, point the hook or status line at a script that tees stdin before
calling `pets`, then **sanitise it before committing**. A real payload carries
your username in several absolute paths, your repository owner, the session name
and `cost.total_cost_usd`. Replace them with the `example/demo` and
`/home/user/...` placeholders already used in the existing fixtures, and keep
the worktree path exactly as it is: the tests rewrite that one field and nothing
else, so the shape stays as the harness sent it.

Assert what the hook *wrote*, not what it returned. Its product is a cache entry
with an expiry that a later render reads, so a function can decode perfectly and
still record the wrong verdict.

## Working on the code

```sh
go test ./...
go build ./cmd/pets
```

The binary reads and writes three locations, all redirectable, so you can
exercise a build without touching your own collection or agent config:

```sh
export HOME=/tmp/pets-sandbox
export XDG_CONFIG_HOME=/tmp/pets-sandbox/config
export XDG_STATE_HOME=/tmp/pets-sandbox/state
```

`HOME` is what `pets install` writes into, and the two XDG paths hold config and
the collection. Nothing else on disk is touched.

Two things worth knowing before changing them:

**Identity must stay stable.** A key is supposed to summon the same creature on
any machine, for as long as the roster holds still. `internal/identity` has a
golden test pinning specific keys to specific creatures, and it is there to make
an accidental reshuffle loud. If you change identity deliberately, repin it and
say so. Adding a creature is one of the ways to change it: see [the blast radius
above](#the-line-is-small-and-the-blast-radius-is-not).

**The render path never runs git and never touches the network.** `pets render`
executes once a second in every open session, so it reads two small files and
nothing else. The expensive work happens in `pets probe`, backgrounded and
throttled. Update checks run only from `card`, `party` and `den`, which a person
types.

## Conduct

By taking part you agree to the [Code of Conduct](CODE_OF_CONDUCT.md). It is the
Contributor Covenant, unmodified.
