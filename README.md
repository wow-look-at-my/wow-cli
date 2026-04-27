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

Print the build version of the running `wow` binary along with the latest available release. Use `--bare` to print only the current version string. Dev builds (dirty working tree or no VCS info) print an empty version.

```sh
wow version
wow version --bare
wow --version
```

## State

Package state is stored as JSON at:

1. `$WOW_STATE_DIR/packages.json` (if set)
2. `$XDG_DATA_HOME/wow/packages.json`
3. `~/.local/share/wow/packages.json` (default)

## Compatibility

Programs must publish GitHub releases with assets following the naming convention:

```
<binary>_<os>_<arch>
<binary>_<os>_<arch>.exe   # Windows
```

This matches the output of the [go-toolchain autorelease](https://github.com/wow-look-at-my/go-toolchain) action.
