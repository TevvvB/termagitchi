# pets, as a Claude Code plugin

Two skills, and a marketplace entry so people can find this from inside Claude Code
rather than having to hear about it first.

```
/plugin marketplace add TevvvB/termagitchi
/plugin install pets@termagitchi
/pets-setup
```

| Skill | Does |
|---|---|
| `/pets` | Prints this worktree's card - the creature, its mood, and the signals behind it. |
| `/pets-setup` | Runs `pets install` for you, or tells you how to get the binary if it is missing. |

## What this plugin deliberately does not do

**It does not set the status line.** `statusLine` is not a field Claude Code reads
from `plugin.json` - `claude plugin validate` says so outright: *"Unknown field
'statusLine'. Claude Code ignores it at load time."* Several published plugins
declare it anyway and it silently does nothing for them.

**It does not install the hooks either**, even though a plugin can. `pets install`
already writes the status line and all three hooks, backs up whatever it touches,
and reverses cleanly with `pets uninstall`. A plugin that wired the same three hooks
would double-fire them for anyone who had run the installer, and leave a copy behind
when either half was removed. One owner for that configuration is worth more than
saving a step.

So the split is: the plugin is discovery and a way in, `pets install` owns the
wiring. `/pets-setup` is the seam between them.

## The binary is still a separate step

A plugin carries configuration, not executables:

```sh
brew install TevvvB/tap/termagitchi
# or: go install github.com/TevvvB/termagitchi/cmd/pets@latest
```

`/pets-setup` checks for it first and prints those two lines rather than failing at
you.
