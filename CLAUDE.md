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
  version.go     # wow version (with --bare and DetectLatest age info) + buildVersion detection from debug.ReadBuildInfo
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

## Build version detection

`buildVersion` (in `cmd/root.go`) is populated at startup from `runtime/debug.ReadBuildInfo()` — specifically the `vcs.time` build setting, formatted as `v0.0.<unix-seconds>` to match the autorelease tag scheme.

This works because `go build` automatically embeds VCS info into binaries since Go 1.18. We do **not** rely on `-ldflags -X` injection: go-toolchain hardcodes its ldflags prefix to its own import path (`github.com/wow-look-at-my/go-toolchain/src/cmd.buildVersion`), so any `-X` it emits silently no-ops against wow-cli's variable. `runtime/debug.ReadBuildInfo()` sidesteps this entirely.

Dirty trees (`vcs.modified=true`) and non-VCS builds leave `buildVersion` empty, which disables the self-update branch in `wow update` and makes `wow version --bare` print an empty line.

`cmd/version.go` defines the `wow version` cobra command directly (with a `--bare` flag and a default mode that calls `up.DetectLatest` and prints current+latest+age). It does not delegate to `selfupdate.RegisterCommands` because that bundle would also register conflicting `install` and `update` commands. `rootCmd.Version = buildVersion` is set in the same `init()` after `populateBuildVersion()` so `wow --version` works on release builds.

## Testing

Tests use two helpers defined in `cmd/cmd_test.go`:

- `withTempState(t)` — sets `WOW_STATE_DIR` to an isolated temp dir
- `withMockUpdater(t, releases)` — injects a fake `go-selfupdate-mini` source with specified releases
- `execute(args...)` — runs a cobra command and returns stdout

Always use `withTempState` in command tests to avoid touching real state.

## CI

Uses `wow-look-at-my/go-toolchain@v1` with `autorelease: true`. On every push, CI builds cross-platform binaries and publishes a tagged release (`v0.0.<timestamp>`). The workflow requires `id-token: write` and `contents: write` permissions.

`pages.yml` deploys `pages/` to GitHub Pages on every push to `master`, serving the installer script at `https://wow-look-at-my.github.io/wow-cli/install.sh`.
