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
  version.go     # selfupdate.RegisterCommands(rootCmd, slug) + dedupe library install/update
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

## Build version detection and self-update

`cmd/version.go`'s `init()` calls `selfupdate.RegisterCommands(rootCmd, slug)` and then drops the library's `install` and `update` commands (we have package-aware versions of both). The library's `version` command stays and serves `wow version` and `wow --version`.

go-selfupdate-mini detects the running binary's version itself: `selfupdate.CurrentVersion()` reads `runtime/debug.ReadBuildInfo()` and, when `Main.Version` is missing/`(devel)`, formats `vcs.time` as `v0.0.<unix-seconds>` (matching the autorelease tag scheme), with `+dirty` appended for modified working trees. We do not need to populate `EmbeddedVersion` ourselves; the library's autorelease branch produces the right format directly from VCS info.

`cmd/update.go`'s `selfUpdateWow` short-circuits on empty / `(devel)` / `+dirty` versions so dev builds are never silently overwritten. The actual self-update goes through `up.UpdateCommand(ctx, exePath, current, slug)` so tests can inject `wowExePathOverride` instead of letting the library resolve `os.Executable()` to the test binary.

## Testing

Tests use two helpers defined in `cmd/cmd_test.go`:

- `withTempState(t)` — sets `WOW_STATE_DIR` to an isolated temp dir
- `withMockUpdater(t, releases)` — injects a fake `go-selfupdate-mini` source with specified releases
- `execute(args...)` — runs a cobra command and returns stdout

Always use `withTempState` in command tests to avoid touching real state.

## CI

Uses `wow-look-at-my/go-toolchain@v1` with `autorelease: true`. On every push, CI builds cross-platform binaries and publishes a tagged release (`v0.0.<timestamp>`). The workflow requires `id-token: write` and `contents: write` permissions.

`pages.yml` deploys `pages/` to GitHub Pages on every push to `master`, serving the installer script at `https://wow-look-at-my.github.io/wow-cli/install.sh`.
