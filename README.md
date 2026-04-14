# go-cli-template

A production-ready Go CLI boilerplate. Fork it, rename `mytool` to your tool name, and ship.

Batteries included: self-update with Ed25519 signing, cross-platform installers, GitHub Actions CI/release pipeline, Claude Code hook integration, Docker-based builds, and semantic versioning via git-cliff.

---

## What's Included

| Feature | Details |
|---------|---------|
| **Self-update** | Ed25519-signed binary updates with SHA256 verification |
| **Cross-platform** | Linux, macOS, Windows (amd64 + arm64) |
| **Installers** | `install.sh` (Unix) and `install.ps1` (Windows) |
| **CI/CD** | GitHub Actions: test on every push, build + sign + release on tags |
| **Changelog** | Auto-generated via [git-cliff](https://git-cliff.org) (Conventional Commits) |
| **Hook integration** | Claude Code PostToolUse hook scaffold |
| **Config** | Global `~/.config/mytool/config.yml` + project-level `.mytool.yml` override |
| **Docker builds** | Nothing installed on the host — Go runs in a container |

---

## Getting Started

### 1. Fork and rename

Replace every `mytool` reference with your tool name:

```sh
# macOS/Linux
find . -type f \( -name "*.go" -o -name "*.sh" -o -name "*.ps1" -o -name "*.yml" -o -name "*.toml" -o -name "Makefile" \) \
  | xargs sed -i 's/mytool/yourtool/g'

# Also update the Go module name in go.mod and all imports
find . -name "*.go" | xargs sed -i 's|github.com/agusrdz/mytool|github.com/yourorg/yourtool|g'
```

Then update the `repo` constant in `updater/updater.go` and the remote URLs in `install.sh`, `install.ps1`, `cliff.toml`, and `.github/workflows/release.yml`.

### 2. Set up signing keys

```sh
# Generate an Ed25519 key pair
openssl genpkey -algorithm ed25519 -out private.pem
openssl pkey -in private.pem -pubout -out public_key.pem

# Get the hex-encoded public key (32 bytes = 64 hex chars)
openssl pkey -in private.pem -noout -text 2>/dev/null | grep -A3 "pub:" | grep -v "pub:" | tr -d ' :\n'

# Base64-encode the private key for GitHub Actions
base64 -w 0 private.pem
```

1. Paste the hex public key into `updater/updater.go` → `const publicKey`
2. Add the base64 private key as `SIGNING_KEY` in your GitHub repository secrets
3. **Delete `private.pem`** — never commit it

### 3. Build

Everything runs inside Docker. No Go installation required on the host.

```sh
make build        # build for current platform → bin/mytool
make test         # run tests
make install      # build + copy to system PATH
make cross        # build all 5 platform binaries
```

---

## CLI Commands

```
mytool [command]
```

### Setup

| Command | Description |
|---------|-------------|
| `mytool init` | Install Claude Code PostToolUse hook |
| `mytool init --status` | Check hook installation status |
| `mytool init --uninstall` | Remove the hook |
| `mytool uninstall` | Remove hook, config, and cache |

### Maintenance

| Command | Description |
|---------|-------------|
| `mytool doctor` | Check hook, config, and binary health |
| `mytool enable` | Resume mytool globally |
| `mytool disable` | Bypass mytool globally |
| `mytool config show` | Show resolved config for current directory |

### Updates

| Command | Description |
|---------|-------------|
| `mytool update` | Check and apply latest version |
| `mytool auto-update` | Show auto-update status |
| `mytool auto-update on` | Enable background updates |
| `mytool auto-update off` | Disable background updates |

### Other

| Command | Description |
|---------|-------------|
| `mytool version` | Show version |
| `mytool help` | Show help |

---

## Installation Scripts

### Unix (macOS / Linux)

```sh
curl -fsSL https://raw.githubusercontent.com/agusrdz/mytool/main/install.sh | sh
```

Override defaults with environment variables:

```sh
MYTOOL_VERSION=v1.2.0 MYTOOL_INSTALL_DIR=/usr/local/bin \
  curl -fsSL .../install.sh | sh
```

The script:
- Detects OS and architecture automatically
- Installs to `~/.local/bin` by default
- Adds the install directory to `~/.zshrc` or `~/.bashrc` if not already in `PATH`

### Windows (PowerShell)

```powershell
irm https://raw.githubusercontent.com/agusrdz/mytool/main/install.ps1 | iex
```

Override with environment variables:

```powershell
$env:MYTOOL_VERSION = "v1.2.0"
$env:MYTOOL_INSTALL_DIR = "C:\Tools\mytool"
irm .../install.ps1 | iex
```

The script:
- Installs to `%LOCALAPPDATA%\Programs\mytool` by default
- Adds to the user `PATH` registry key
- Broadcasts `WM_SETTINGCHANGE` so new terminals pick up the PATH immediately (no restart required)

---

## Configuration

### Global config

`~/.config/mytool/config.yml` — applies to all projects.

### Project override

`.mytool.yml` at the project root (or any parent directory) — overlaid on top of the global config.

```yaml
enabled: true
timeout: 30s
skip_paths:
  - dist/
  - node_modules/
  - bin/
```

---

## Release Process

Releases use [Conventional Commits](https://www.conventionalcommits.org) to determine the version bump automatically.

```sh
make release        # auto-detect bump from commits (feat → minor, fix → patch, feat! → major)
make release-patch  # v1.0.0 → v1.0.1
make release-minor  # v1.0.0 → v1.1.0
make release-major  # v1.0.0 → v2.0.0
```

Requires [git-cliff](https://git-cliff.org/docs/installation) (`cargo install git-cliff` or via Homebrew).

Each release automatically:
1. Validates semver tag format
2. Runs the full test suite
3. Updates `CHANGELOG.md`
4. Cross-compiles 5 platform binaries
5. Generates SHA256 checksums
6. Signs checksums with Ed25519
7. Creates a GitHub Release with all artifacts
8. Attests build provenance (GitHub Artifact Attestations)
9. Updates Homebrew formula (if `HOMEBREW_TAP_TOKEN` secret is set)

### GitHub Secrets

| Secret | Required | Description |
|--------|----------|-------------|
| `SIGNING_KEY` | Yes | Base64-encoded Ed25519 private key |
| `HOMEBREW_TAP_TOKEN` | No | GitHub token for pushing to your Homebrew tap |

---

## Project Structure

```
.
├── .github/workflows/
│   ├── ci.yml              # Run tests on every push/PR
│   └── release.yml         # Build, sign, and release on version tags
├── check/
│   └── checker.go          # Checker plugin interface and registry
├── config/
│   └── config.go           # Global + project config loading
├── hooks/
│   └── hooks.go            # Claude Code hook install/uninstall
├── updater/
│   ├── updater.go          # Self-update with Ed25519 + SHA256 verification
│   ├── auto_update.go      # Background update checks and apply-on-next-run
│   └── auto_update_config.go # auto-update toggle and update-available hint
├── cliff.toml              # git-cliff changelog configuration
├── color.go                # ANSI color helpers
├── docker-compose.yml      # Dev build environment
├── Dockerfile              # golang:1.24-alpine build image
├── install.ps1             # Windows installer
├── install.sh              # Unix installer
├── main.go                 # CLI entry point and command router
├── Makefile                # Build, test, install, release targets
└── public_key.pem          # Ed25519 public key (replace before publishing)
```

---

## License

MIT
