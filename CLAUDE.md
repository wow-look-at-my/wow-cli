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

**`store.Repo`** — one configured manifest URL + decryption identity:
- `URL` — encrypted-manifest endpoint
- `Identity` — age X25519 private key (`AGE-SECRET-KEY-...`); kept out of `String()` output
- `AddedAt` — timestamp

**`store.RepoList`** — the registry of repos, backed by `repos.json` (mode 0600).

**`manifest.Manifest`** — the decrypted catalog: schema version, generated-at, and `Packages map[slug]*Package`. Each Package has `Latest`, `Description`, and `Releases` (each release has a `Tag` and a list of `Asset{Name, URL, Size, SHA256}`).

State directory resolution order: `$WOW_STATE_DIR` → `$XDG_DATA_HOME/wow` → `~/.local/share/wow`.

## Install / update flow

`runInstall` and `updateOne` first call `repoCache.find(ctx, slug, binary, tag)`. The cache:

1. Loads `repos.json` (once per command run).
2. For each configured repo, fetches & decrypts the manifest (memoized by URL).
3. Returns the first repo that has the slug AND a matching platform-specific asset (`<binary>_<os>_<arch>[.exe]`). Repos that don't match are skipped silently.

If the cache returns nil (no repo matched), the existing GitHub-API path runs (`detectLatest` / `DetectVersion` via go-selfupdate-mini).

`update` reuses one `repoCache` across all installed packages so the same
manifest URL isn't re-fetched per package.

## Encrypted manifest format

age X25519, multi-recipient. The list of recipients lives in `recipients.jsonc`
at the repo root (checked into git for auditability). `manifest.LoadRecipients`
parses JSONC (line + block comments stripped via `stripJSONCComments`) and
accepts the object form (`{"recipients": [...]}`), a bare array of objects, or
a bare array of strings. The optional top-level `$schema` key is silently
ignored — `recipients.schema.json` at the repo root provides editor validation. `manifest.Encrypt` takes `[]string` and emits a
single ciphertext that any of the corresponding identities can decrypt —
that's what makes per-user keys + revocation work. Each user holds their own
`AGE-SECRET-KEY-...` identity and passes it to `wow repo add`.

Both halves are kept secret in this design (the manifest itself is meant to
be private), so the "public/private" naming is loose: functionally it's a
shared-recipient-set / per-user-identity scheme.

`wow repo keygen` generates one keypair per invocation; run it N times to onboard
N users. Revoke a user by removing their entry from `recipients.jsonc` and
republishing. The pages workflow passes `--skip-if-empty` to `repo build`
so initial setup (with zero recipients) doesn't break the pages deploy.

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
- `withMockGitHub(t, items, releases)` — fake GitHub server covering both `/search/repositories` and `/repos/.../releases` (used by `repo build` tests)
- `newTestKeyPair(t)` — generates a fresh age X25519 keypair
- `startManifestServer(t, m, recipient)` — encrypts m and serves it over HTTP
- `startBinaryServer(t, body)` — serves arbitrary bytes (a fake release asset)
- `resetBuildManifestFlags(t)` — restores cobra-bound flag vars between `repo build` tests
- `execute(args...)` — runs a cobra command and returns stdout+stderr

Always use `withTempState` in command tests to avoid touching real state.

Cobra binds flags to package-level vars; tests that set flags must restore them in `t.Cleanup` (see `resetBuildManifestFlags`, the `installVersion` cleanup in `update_test.go`).

## CI

`ci.yml` has two jobs. `ci` runs on every push: build, test, autorelease via `go-toolchain@v1`. `pages` runs only on master after `ci`, calling the reusable `deploy-manifest.yml` workflow with `deploy-pages: true`.

`deploy-manifest.yml` is a reusable workflow (`workflow_call`) designed for any repo that needs an encrypted manifest. Inputs: `org` (required), `deploy-pages` (boolean, default false). It downloads the latest wow binary via `download-release-binary`, runs `repo build --org <org> --skip-if-empty`, uploads the manifest as a GHA artifact, and optionally deploys `pages/` to GitHub Pages. The calling repo must have a `pages/` directory and a `recipients.jsonc`.

The installer script is served at `https://wow-look-at-my.github.io/wow-cli/install.sh`; the encrypted manifest is at `https://wow-look-at-my.github.io/wow-cli/manifest.json.age` once `recipients.jsonc` has at least one entry.
