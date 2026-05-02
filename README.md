# wow

A package manager for programs that follow the [go-toolchain autorelease](https://github.com/wow-look-at-my/go-toolchain) pattern — GitHub releases with assets named `<binary>_<os>_<arch>[.exe]`.

## Installation

```sh
curl -fsSL https://wow-look-at-my.github.io/wow-cli/install.sh | sh
```

This installs `wow` to `~/.local/bin/wow`. No PATH or shell config is modified.

Or download the latest binary manually from the [releases page](https://github.com/wow-look-at-my/wow-cli/releases/latest):

| Platform      | Binary |
|---------------|--------|
| Linux amd64   | `wow_linux_amd64` |
| Linux arm64   | `wow_linux_arm64` |
| macOS amd64   | `wow_darwin_amd64` |
| macOS arm64   | `wow_darwin_arm64` |
| Windows amd64 | `wow_windows_amd64.exe` |
| Windows arm64 | `wow_windows_arm64.exe` |

Place it somewhere on your `PATH` (e.g. `~/.local/bin/wow`).

Once installed, `wow update` keeps itself up to date automatically alongside your other packages.

## Usage

```
wow <command> [flags]
```

### Commands

#### `install <owner/repo>`

Install a binary from a GitHub release.

```sh
wow install wow-look-at-my/go-toolchain
```

Flags:
- `--name <name>` — override the binary name (default: repo name)
- `--path <path>` — override the install path (default: `~/.local/bin/<name>`)
- `--version <tag>` — install a specific release tag (default: latest)

When sources are configured (see below), `install` checks them first and falls back to the GitHub API only when no source has the package.

#### `update`

Update all installed packages to their latest releases. Also updates `wow` itself.

```sh
wow update
```

#### `list`

List all installed packages with their name, version, and install path.

```sh
wow list
```

#### `uninstall <name|owner/repo>...`

Remove one or more packages and delete their binaries.

```sh
wow uninstall go-toolchain
```

#### `which <name|owner/repo>`

Print the install path of a package.

```sh
wow which go-toolchain
```

#### `version`

Print the build version of the running `wow` binary along with the latest available release. Use `--bare` to print only the current version string. Dirty trees get a `+dirty` suffix; binaries built outside of a VCS checkout fall back to whatever `runtime/debug.ReadBuildInfo` reports (typically `(devel)`).

```sh
wow version
wow version --bare
wow --version
```

### Encrypted manifest sources

`wow` can install packages from an encrypted manifest hosted at any URL, with no GitHub API access required at install time. The manifest is JSON encrypted with [age](https://age-encryption.org), distributed along with a key that decrypts it.

#### `add-src <url> <key>`

Register an encrypted-manifest source. `key` is the age identity (private key, `AGE-SECRET-KEY-...`) that decrypts the manifest at `url`. The source is fetched and decrypted immediately to verify the key is valid.

```sh
wow add-src https://wow-look-at-my.github.io/wow-cli/manifest.json.age AGE-SECRET-KEY-...
```

#### `remove-src <url>`

Remove a configured source.

```sh
wow remove-src https://wow-look-at-my.github.io/wow-cli/manifest.json.age
```

#### `list-src`

List configured sources. The decryption key is shown truncated.

```sh
wow list-src
```

#### `keygen`

Generate a fresh age X25519 keypair for publishing a manifest. Prints both halves; the recipient is what your CI uses to encrypt, the identity is what one user passes to `add-src`. Run it once per user.

```sh
wow keygen
```

#### `build-manifest`

Walk a GitHub org's repos, gather their releases, and emit an age-encrypted manifest. Used by CI to refresh the published manifest. Recipients come from `recipients.json` (one entry per authorized user) and from any `--recipient` flags; both sources are merged.

```sh
wow build-manifest --org wow-look-at-my --output manifest.json.age
```

Flags:
- `--org <org>` — GitHub org to enumerate (default: `wow-look-at-my`)
- `--recipients-file <path>` — JSON file of recipients (default: `recipients.json`; pass `""` to skip)
- `--recipient <age1...>` — age recipient public key (repeatable, merged with the file)
- `--output <file>` — output file (`-` for stdout, default `-`)
- `--plain` — write plain JSON instead of encrypting (debugging)

### `recipients.json`

The list of who can decrypt the manifest lives in `recipients.json` at the repo root. It's checked into git so additions and revocations are auditable in the history and reviewable as PRs:

```json
{
  "recipients": [
    {"name": "alice",  "key": "age1...",  "note": "laptop, added 2026-05-01"},
    {"name": "bob",    "key": "age1..."},
    {"name": "ci-bot", "key": "age1...",  "note": "for nightly fleet provisioning"}
  ]
}
```

Accepted shapes: the object form above, a bare array of objects, or a bare array of strings. `name` and `note` are optional but recommended — that's the whole reason for keeping the list in git.

### Setting up a private package source

1. For each user who should be able to install from the manifest, run `wow keygen` once. Each invocation prints a unique recipient/identity pair.
2. PR each user's recipient (`age1...`) into `recipients.json` with a name and optional note. The included [pages workflow](.github/workflows/pages.yml) encrypts the manifest to everyone in that file and publishes `manifest.json.age` to GitHub Pages on every push to master.
3. Distribute each identity (`AGE-SECRET-KEY-...`) out-of-band to its corresponding user. They run `wow add-src <pages url>/manifest.json.age <their identity>` to start installing from your manifest without hitting the GitHub API.

To revoke a user, send a PR removing their entry from `recipients.json`. On the next deploy, the new manifest no longer encrypts to their key; their copy of the file becomes useless. Other users' identities keep working.

The published manifest is encrypted before it leaves the runner, so it's safe to host on a public URL — only holders of one of the listed identities can read it. If `recipients.json` is empty, the workflow skips manifest publishing and just deploys the rest of `pages/`.

## State

Package state is stored as JSON at:

1. `$WOW_STATE_DIR/packages.json` (if set)
2. `$XDG_DATA_HOME/wow/packages.json`
3. `~/.local/share/wow/packages.json` (default)

Source state lives next to it as `sources.json` (file mode 0600 since it holds decryption keys).

## Compatibility

Programs must publish GitHub releases with assets following the naming convention:

```
<binary>_<os>_<arch>
<binary>_<os>_<arch>.exe   # Windows
```

This matches the output of the [go-toolchain autorelease](https://github.com/wow-look-at-my/go-toolchain) action.
