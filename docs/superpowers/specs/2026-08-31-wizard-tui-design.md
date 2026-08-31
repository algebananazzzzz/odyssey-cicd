# Wizard TUI and headless CLI design

2026-08-31. Session 5 of the roadmap: the full `odyssey-cli new` flow, replacing the selection-only wizard PoC.

## Goal

`odyssey-cli new` runs the whole scaffold: pick the row, name the project, fill variables, review the plan, apply it. One Bubble Tea program with a huh form embedded per page (the huh `examples/bubbletea` pattern), a headless mode sharing the same runner, and a `find` subcommand for discovery.

## Decisions that amend the roadmap

| | |
|---|---|
| huh un-rejected | the objections were grouped layout and racy async options; a parent Bubble Tea model owns the layout, and options come synchronously from the in-memory manifest, so neither applies |
| wizard order | provider, architecture, stack, environments; environments moved last since it filters nothing, the filter chain is provider to architecture to stack |
| pages | architecture, project, variables, plan, apply; terraform-flavored names, no "selection" or "preview" |

## Flow

`new` validates the manifest and fragments exactly as today, then starts one `tea.Program`, inline rather than altscreen, so apply output and the closing checklist persist in scrollback.

Pages in order: architecture, project, variables, plan, apply.

`esc` goes back one page with answers kept; on the architecture page it aborts. `ctrl+c` aborts anywhere via `ErrAborted`. Nothing writes to disk before apply, so every abort leaves the filesystem untouched.

## Package layout

| | |
|---|---|
| `internal/wizard` | `wizard.go` parent model and page machine, `forms.go` one huh.Form builder per page, `theme.go` huh theme, lipgloss styles, status panel |
| `internal/render` | new; the engine seam below |
| `cmd/odyssey-cli` | main moves here from `cmd/main.go`, matching the settled binary name |

## Parent model

Fields: manifest, shapes, current page, current `*huh.Form`, `Answers`, the plan, apply state (spinner, steps), width and height.

Entering a page builds its form fresh from `Answers`, which is what lets later pages pre-fill from earlier answers. `Update` routes every message to the form; `huh.StateCompleted` harvests values and advances. Layout is `lipgloss.JoinHorizontal`: form pane left, bordered status panel right, rows for cloud, arch, stack, envs, proj filled live. Under 60 columns the panel hides and the form takes the full width.

## Pages

### architecture

One huh group, four selects in order: Provider, Architecture, Stack, Environments. Architecture options come from `OptionsFunc` bound to the provider value, Stack from one bound to architecture, so changing an earlier answer re-filters below it. Environments lists the deploy shape directories, unfiltered.

### project

Two `huh.Input`s: project code, validated `^[a-z][a-z0-9-]*$` since it lands in `worker_name` and the state key; target directory, placeholder `./<project code>`, empty meaning that default. Completing the page validates the directory: absent, or existing and empty.

### variables

Built on entry from top-level `inputs:` plus the selected architecture's. Per-env vars are those appearing in `{{ENV}}`-named files, the same scan validation performs. The environment list derives from the chosen shape (dual: preprod, prd; single: prd).

Screen one asks the env-agnostic vars plus the first environment's per-env values. Each later environment gets its own screen, pre-filled with the first environment's answers, so accepting the defaults is pressing enter through. Optional vars accept empty. GitHub variables and secrets are never asked; they belong to the checklist. The page is skipped entirely when the selection has nothing to ask.

### plan

The rendered plan replaces the form pane: a scrollable file tree grouped by top-level path with per-group counts, the status panel unchanged on the right, and a `huh.Confirm` at the bottom: "Write N files to `<dir>`?". No aborts with nothing written, terraform semantics. `esc` returns to variables.

### apply

The bubbletea `tui-daemon-combo` pattern: a spinner with the in-flight step, each completed step printed via `tea.Println` as a `✓` line so it survives in scrollback. Steps stream from `Plan.Apply`'s events channel, one per composition unit: workflows, makefile, infra, stack files, AGENTS.md. On the last event the program quits and the closing output prints.

## Engine seam

`Answers` carries the four axis choices, project code, target directory, and variable values per environment.

`render.Plan(templates string, a Answers) (Plan, error)`: pure, renders every file in memory, returns paths and contents plus the derived metadata: the GitHub variables and secrets owed, the bootstrap section.

`Plan.Apply(dir string, events chan<- Step) error`: writes the plan and streams one step per composition unit.

TUI and headless share exactly these two calls; neither has any other path to the filesystem.

## Closing output

Printed to plain stdout after the program exits, so it is copyable:

The GitHub checklist: each required variable and secret as a runnable command, `gh variable set NAME --body <value>` and `gh secret set NAME`, with a note on scope (org-shaped values set once, environment scope where prd differs).

The bootstrap section: `--bootstrap` runs it; the default prints it as the continuation prompt, per the roadmap.

## Headless

The TUI requires stdin and stdout to both be TTYs; otherwise the run is headless. A TTY with partial flags opens the TUI with completed pages skipped, so flags shorten the wizard rather than forking a second code path.

Flags mirror the pages: `--provider`, `--architecture`, `--stack`, `--environments`, `--project`, `--dir`, repeatable `--var NAME=VALUE` setting all environments, `--var env:NAME=VALUE` scoping one, and `--yes`.

`new` accepts every axis explicitly: the same stack can deploy to many architectures, so inference never replaces the flags. A given axis pins it; missing upstream axes derive only when unique (an architecture always fixes its provider; a stack fixes architecture and provider only while it has exactly one architecture). Ambiguity is reported as a remaining choice, never guessed.

An incomplete headless run prints what is answered, what is derived and why, and what remains with its valid values, optional vars marked, then exits 2:

```
$ odyssey-cli new --stack nextjs --project acme-web
stack         nextjs
architecture  cloudflare-worker   (derived)
provider      cloudflare          (derived)
project       acme-web
dir           ./acme-web          (default)

missing
  --environments        single | dual
optional (empty ok)
  --var PREPROD_URL=  --var PRD_URL=
  --var CUSTOM_DOMAIN=          per-env: --var prd:CUSTOM_DOMAIN=

add the missing flags; --yes applies without confirmation
```

A complete headless run without `--yes` prints the plan tree and writes nothing, exit 0. With `--yes` it prints the plan tree, applies with one log line per step, and prints the checklist. A no-flag headless run lists the flags and points at `find`.

## find

`odyssey-cli find [term|axis=value ...]`, read-only, identical for humans and agents. Rows are the denormalized (stack, architecture, provider) combinations. Bare terms substring-match any column, `axis=value` pins one axis, filters AND together.

Many matches render a table. Exactly one match renders a detail card: the full chain, environment shapes, every input with optionality and per-env scope, the GitHub variables and secrets that row requires, and a ready-to-edit `new` invocation as the last line. Zero matches prints `no rows match`, exit 1.

All of `find`'s output is computed from the manifest, so a new fragment appears with no code change, same as the rest of the engine.

## Errors and exit codes

| | |
|---|---|
| 0 | success, including a headless plan-only run |
| 1 | failure or abort |
| 2 | incomplete headless answers |

A `render.Plan` failure prints and exits 1; it is a fragment or engine bug, not user-fixable in the TUI. An apply step failure prints a `✗` line with the error and exits 1, leaving the partial directory with a note that it is incomplete. Page-level validation stays on the page as a huh validation error.

## Testing

Form builders are pure: assert filtered options per manifest, per-env screen generation, and the validators directly. The page machine is driven the way `wizard_test.go` drives the PoC: `tea.KeyMsg` sequences into `Update`, asserting page transitions, back navigation, abort, and the final `Answers`. `render.Plan` is compared against expected file lists per manifest combination, which the CI render-everything harness already proves; `Apply` runs into a temp dir asserting files and the event stream. Optionally one `teatest` golden run of the happy path as a smoke test.

## Dependencies

Add `github.com/charmbracelet/huh` and `github.com/charmbracelet/bubbles` (spinner). bubbletea and lipgloss are already in.
