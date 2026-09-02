---
name: pets-setup
description: "Wire pets into this machine's Codex config, or say how to install the binary. Use when the user asks to set pets up for Codex or types /pets-setup."
---

# Pets setup (Codex)

Check for the binary, then wire Codex:

```sh
command -v pets
```

If `pets` is on `PATH`, prefer the Codex harness when the installer supports it:

```sh
pets install --harness=codex
```

If that flag is rejected, fall back to:

```sh
pets install
```

Relay the installer's own output either way. It backs up any config it touches, and
`pets uninstall` reverses it.

If `command -v pets` fails, the binary is missing. Do not install it yourself —
print these options and stop:

```sh
brew install TevvvB/tap/termagitchi
go install github.com/TevvvB/termagitchi/cmd/pets@latest
```

(or `curl -fsSL https://raw.githubusercontent.com/TevvvB/termagitchi/main/install.sh | sh`)

## Codex trust warning

**Codex must trust hooks before they run.** A fresh or changed hook is completely
silent until you accept it on the next session start. `codex doctor` does not
mention hooks. Warn the user about this explicitly after a successful install.

Tell the user to start a new Codex session so hooks and any shell snippet take effect.
