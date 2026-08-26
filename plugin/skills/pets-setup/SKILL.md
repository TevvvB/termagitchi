---
name: pets-setup
description: "Wire pets into this machine's Claude Code config, or say how to install the binary. Use when the user types /pets-setup or asks to set pets up."
allowed-tools: Bash(pets:*), Bash(command:*)
---

Run the installer and report what it wired:

```sh
command -v pets >/dev/null 2>&1 && pets install --harness=claude || echo "not-installed"
```

If that printed `not-installed`, the binary is missing. Do not try to install it
yourself - print these two options and stop:

```sh
brew install TevvvB/tap/termagitchi
go install github.com/TevvvB/termagitchi/cmd/pets@latest
```

Otherwise relay the installer's own output. It backs up any config it touches, and
`pets uninstall` reverses it. Tell the user to start a new session, because Claude
Code reads the status line and hooks at startup.
