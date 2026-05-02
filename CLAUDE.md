# wow-cli

Package manager for go-toolchain autorelease pattern binaries. Supports both
direct GitHub-API installs and encrypted-manifest sources for hosting a
private package catalog on public infrastructure.

## Build & test

ALWAYS use `go-toolchain` (no arguments) — never bare `go build` or `go test`.

```sh
go-toolchain
```

This runs mod tidy, tests, coverage, and build.

## Structure

Top-level packages: `cmd/` (cobra commands, one per file), `store/` (on-disk JSON state for installed packages and sources), `manifest/` (encrypted catalog format + HTTP helpers), `pages/` (GitHub Pages content), `.github/workflows/` (CI + Pages deploy).

Each command file registers itself via `init()` — do not add registration calls to `root.go` or `main.go`.

## Key types

**`store.Package`** — one installed binary:
- `Slug` — GitHub `owner/repo`
- `Name` — binary name on disk
- `Path` — full install path
- `Version` — release tag

**`store.Store`** — the registry, backed by `packages.json`. Lookup works by either slug or binary name.

**`store.Source`** — one configured manifest URL + decryption identity:
- `URL` — encrypted-manifest endpoint
- `Identity` — age X25519 private key (`AGE-SECRET-KEY-...`); kept out of `String()` output
- `AddedAt` — timestamp

**`store.SourceStore`** — the registry of sources, backed by `sources.json` (mode 0600).

**`manifest.Manifest`** — the decrypted catalog: schema version, generated-at, and `Packages map[slug]*Package`. Each Package has `Latest`, `Description`, and `Releases` (each release has a `Tag` and a list of `Asset{Name, URL, Size, SHA256}`).

State directory resolution order: `$WOW_STATE_DIR` → `$XDG_DATA_HOME/wow` → `~/.local/share/wow`.

## Install / update flow

`runInstall` and `updateOne` first call `sourceCache.find(ctx, slug, binary, tag)`. The cache:

1. Loads `sources.json` (once per command run).
2. For each configured source, fetches & decrypts the manifest (memoized by URL).
3. Returns the first source that has the slug AND a matching platform-specific asset (`<binary>_<os>_<arch>[.exe]`). Sources that don't match are skipped silently.

If the cache returns nil (no source matched), the existing GitHub-API path runs (`detectLatest` / `DetectVersion` via go-selfupdate-mini).

`update` reuses one `sourceCache` across all installed packages so the same
manifest URL isn't re-fetched per package.

## Encrypted manifest format

age X25519, multi-recipient. The CI-side `WOW_MANIFEST_RECIPIENT` secret holds
one or more recipients (`age1...`), separated by newlines or commas (per
`splitRecipients`). `manifest.Encrypt` takes `[]string` and emits a single
ciphertext that any of the corresponding identities can decrypt — that's what
makes per-user keys + revocation work. Each user holds their own
`AGE-SECRET-KEY-...` identity and passes it to `wow add-src`.

Both halves are kept secret in this design (the manifest itself is meant to
be private), so the "public/private" naming is loose: functionally it's a
shared-recipient-set / per-user-identity scheme.

`wow keygen` generates one keypair per invocation; run it N times to onboard
N users. Revoke a user by dropping their recipient from `WOW_MANIFEST_RECIPIENT`
and republishing.

## Build version detection and self-update

`cmd/version.go`'s `init()` calls `selfupdate.RegisterCommands(rootCmd, slug)` and then drops the library's `install` and `update` commands (we have package-aware versions of both). The library's `version` command stays and serves `wow version` and `wow --version`.

go-selfupdate-mini detects the running binary's version itself: `selfupdate.CurrentVersion()` reads `runtime/debug.ReadBuildInfo()` and, when `Main.Version` is missing/`(devel)`, formats `vcs.time` as `v0.0.<unix-seconds>` (matching the autorelease tag scheme), with `+dirty` appended for modified working trees. We do not need to populate `EmbeddedVersion` ourselves; the library's autorelease branch produces the right format directly from VCS info.

`cmd/update.go`'s `selfUpdateWow` short-circuits on empty / `(devel)` / `+dirty` versions so dev builds are never silently overwritten. The actual self-update goes through `up.UpdateCommand(ctx, exePath, current, slug)` so tests can inject `wowExePathOverride` instead of letting the library resolve `os.Executable()` to the test binary.

## Testing

Tests use these helpers in `cmd/cmd_test.go`, `cmd/sources_test.go`, and `cmd/build_manifest_test.go`:

- `withTempState(t)` — sets `WOW_STATE_DIR` to an isolated temp dir
- `withMockUpdater(t, binary, tag)` — injects a fake `go-selfupdate-mini` source with a single release
- `withMockUpdaterPerSlug(t, perSlug)` — fake source returning different releases per slug
- `withMockSearchServer(t, body)` — fake GitHub `/search/repositories` server (`ghSearchBaseURL`)
- `withMockGitHub(t, items, releases)` — fake GitHub server covering both `/search/repositories` and `/repos/.../releases` (used by build-manifest tests)
- `newTestKeyPair(t)` — generates a fresh age X25519 keypair
- `startManifestServer(t, m, recipient)` — encrypts m and serves it over HTTP
- `startBinaryServer(t, body)` — serves arbitrary bytes (a fake release asset)
- `resetBuildManifestFlags(t)` — restores cobra-bound flag vars between build-manifest tests
- `execute(args...)` — runs a cobra command and returns stdout+stderr

Always use `withTempState` in command tests to avoid touching real state.

Cobra binds flags to package-level vars; tests that set flags must restore them in `t.Cleanup` (see `resetBuildManifestFlags`, the `installVersion` cleanup in `update_test.go`).

## CI

`ci.yml` uses `wow-look-at-my/go-toolchain@v1` with `autorelease: true`. On every push, CI builds cross-platform binaries and publishes a tagged release (`v0.0.<timestamp>`). Requires `id-token: write` and `contents: write` permissions.

`pages.yml` (push to master only) builds a fresh wow binary, runs `wow build-manifest --output pages/manifest.json.age` if `WOW_MANIFEST_RECIPIENT` is set, and deploys `pages/` to GitHub Pages. The installer script is served at `https://wow-look-at-my.github.io/wow-cli/install.sh`; the encrypted manifest is at `https://wow-look-at-my.github.io/wow-cli/manifest.json.age` once the secret is configured.
