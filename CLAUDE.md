# wow-cli

Package manager for go-toolchain autorelease pattern binaries.

## Build & test

ALWAYS use `go-toolchain` (no arguments) — never bare `go build` or `go test`.

```sh
go-toolchain
```

This runs mod tidy, tests, coverage, and build.

## Structure

```
main.go          # delegates to cmd.Execute()
cmd/
  root.go        # rootCmd, shared helpers (install/update logic, platform detection)
  install.go     # wow install
  update.go      # wow update
  uninstall.go   # wow uninstall
  list.go        # wow list
  which.go       # wow which
  cmd_test.go    # all command tests (mock selfupdate source)
store/
  store.go       # JSON state at ~/.local/share/wow/packages.json
  store_test.go  # store CRUD and persistence tests
pages/
  install.sh     # curl-pipe installer, served via GitHub Pages
```

Each command file registers itself via `init()` — do not add registration calls to `root.go` or `main.go`.

## Key types

**`store.Package`** — one installed binary:
- `Slug` — GitHub `owner/repo`
- `Name` — binary name on disk
- `Path` — full install path
- `Version` — release tag

**`store.Store`** — the registry, backed by `packages.json`. Lookup works by either slug or binary name.

State directory resolution order: `$WOW_STATE_DIR` → `$XDG_DATA_HOME/wow` → `~/.local/share/wow`.

## Testing

Tests use two helpers defined in `cmd/cmd_test.go`:

- `withTempState(t)` — sets `WOW_STATE_DIR` to an isolated temp dir
- `withMockUpdater(t, releases)` — injects a fake `go-selfupdate-mini` source with specified releases
- `execute(args...)` — runs a cobra command and returns stdout

Always use `withTempState` in command tests to avoid touching real state.

## CI

Uses `wow-look-at-my/go-toolchain@v1` with `autorelease: true`. On every push, CI builds cross-platform binaries and publishes a tagged release (`v0.0.<timestamp>`). The workflow requires `id-token: write` and `contents: write` permissions.

`pages.yml` deploys `pages/` to GitHub Pages on every push to `master`, serving the installer script at `https://wow-look-at-my.github.io/wow-cli/install.sh`.
