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
  search.go      # wow search
  version.go     # selfupdate.EmbeddedVersion = autoreleaseVersion(); selfupdate.RegisterCommands(...)
  cmd_test.go    # most command tests (mock selfupdate source)
  version_test.go # version helper + shouldSelfUpdateWow tests
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

## Build version detection

`cmd/version.go`'s `init()` sets `selfupdate.EmbeddedVersion` from `autoreleaseVersion()`, which reads the binary's embedded VCS info (`runtime/debug.ReadBuildInfo`) and formats `vcs.time` as `v0.0.<unix-seconds>` to match the autorelease tag scheme. Dirty trees (`vcs.modified=true`) get `+dirty` appended; non-VCS builds leave it empty so `selfupdate.CurrentVersion()` falls through to its short-revision/`(devel)` fallback.

We populate `EmbeddedVersion` (rather than relying on the library's auto-detection alone) because go-selfupdate-mini's auto-detection produces a 12-char SHA — useful for display but never equal to autorelease tags like `v0.0.1777283542`, which would break wow's self-update equality check. We also do **not** rely on `-ldflags -X` injection: go-toolchain hardcodes its ldflags prefix to its own import path (`github.com/wow-look-at-my/go-toolchain/src/cmd.buildVersion`), so any `-X` it emits silently no-ops against any other module's variable. `runtime/debug.ReadBuildInfo()` sidesteps that entirely.

After setting `EmbeddedVersion`, `init()` calls `selfupdate.RegisterCommands(rootCmd, slug)`. That registers the library's `version`, `install`, and `update` commands and sets `rootCmd.Version`. The library's `install` and `update` are dropped from `rootCmd` immediately because we have package-aware versions of those; the library's `version` is what serves `wow version` and `wow --version`.

`shouldSelfUpdateWow(v string) bool` in `cmd/update.go` gates the self-update step on a real autorelease tag — it skips empty, `(devel)`, and any `+dirty`-suffixed string so dev builds are never silently overwritten.

## Testing

Tests use two helpers defined in `cmd/cmd_test.go`:

- `withTempState(t)` — sets `WOW_STATE_DIR` to an isolated temp dir
- `withMockUpdater(t, releases)` — injects a fake `go-selfupdate-mini` source with specified releases
- `execute(args...)` — runs a cobra command and returns stdout

Always use `withTempState` in command tests to avoid touching real state.

## CI

Uses `wow-look-at-my/go-toolchain@v1` with `autorelease: true`. On every push, CI builds cross-platform binaries and publishes a tagged release (`v0.0.<timestamp>`). The workflow requires `id-token: write` and `contents: write` permissions.

`pages.yml` deploys `pages/` to GitHub Pages on every push to `master`, serving the installer script at `https://wow-look-at-my.github.io/wow-cli/install.sh`.
