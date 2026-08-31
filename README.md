# odyssey

Composable CI/CD scaffolding. One command generates GitHub Actions workflows that only ever call `make`, a flat assembled Makefile, and optional Terraform infra for your stack. Templates are embedded, so the binary is self-contained.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/algebananazzzzz/odyssey-cicd/main/install.sh | sh
```

Detects OS and architecture, verifies the sha256 against the release checksums, and installs to `~/.local/bin` (override with `ODYSSEY_INSTALL_DIR`). Linux and macOS, amd64 and arm64.

Alternatives:

```bash
go install github.com/algebananazzzzz/odyssey@latest
```

or download an archive from [Releases](https://github.com/algebananazzzzz/odyssey-cicd/releases).

Keep it current with `odyssey-cli update`. The CLI prints a one-line notice when a newer release exists.

## Use

```bash
odyssey-cli find                  # browse stacks, architectures, providers
odyssey-cli find nextjs           # one match prints a detail card
odyssey-cli new                   # interactive wizard in a terminal
odyssey-cli new --stack nextjs --environments dual --project my-web --yes
```

Without a TTY, `new` is headless and driven entirely by flags. Run it with no flags to get a report of what is missing.

## Agent skill

```bash
npx skills add algebananazzzzz/odyssey-cicd
```

Installs the odyssey skill into Claude Code, Codex, Cursor, Gemini CLI and other SKILL.md-compatible agents, teaching them to drive `odyssey-cli` end to end.

## How it works

Workflows never name a tool. Every step is `make check` / `make test` / `make deploy ENV=prd`, so the workflow files are identical across projects and all stack variation collapses into the Makefile, assembled from fragments at generate time.

Two pipeline shapes: `single` (push to main cuts `vX.Y.Z` and deploys) and `dual` (merge cuts `vX.Y.Z-beta`, deploys preprod, cuts `vX.Y.Z`; shipping prd is a deliberate dispatch against that tag). The bump level comes from the squash-merge PR title, Conventional Commits style: `feat!:` or `BREAKING CHANGE` means major, `feat:` means minor, anything else is patch.

Template development happens in this repo: `fragments/` plus `manifest.yml`. `odyssey-cli validate` checks a checkout, and `--templates <path>` on `new` or `find` generates from one instead of the embedded copy.
