# Security Policy

## Supported versions

Only the [latest release](https://github.com/TevvvB/termagitchi/releases/latest) of TermaGITchi (`pets`) is supported for security fixes. Older tags are not patched.

## Reporting a vulnerability

Please report vulnerabilities through [GitHub Security Advisories](https://github.com/TevvvB/termagitchi/security/advisories/new) for **TevvvB/termagitchi**.

Do **not** open a public issue for security reports.

We aim to acknowledge reports within a few business days and will coordinate a fix and disclosure timeline with you.

## Scope

**In scope**

- The Go CLI: `cmd/pets` and `internal/`
- `install.sh`
- Claude Code and Codex plugin skills and hooks shipped by this repository

**Out of scope**

- Claude Code itself
- Codex CLI itself
- Third-party harnesses, shells, or package managers that merely consume `pets`

## Safe harbor

Good-faith research that stays within this scope and avoids privacy harm, service disruption, or data destruction is welcome. If you are unsure whether something is in scope, ask via a private advisory first.
