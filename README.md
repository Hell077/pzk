# Portguard

Know what owns your ports before you stop it.

A native Go terminal app for Linux, macOS, and Windows. Inspect TCP listeners and bound UDP sockets with process names, parent processes, service-manager context, and Docker / Podman published ports. Browse a live terminal UI, use focused commands, or pipe JSON into your own tools.

[Introduction](https://hell077.github.io/portguard/) · [Download binaries](https://github.com/Hell077/portguard/releases/latest) · [CI](https://github.com/Hell077/portguard/actions/workflows/ci.yml)

## Install

With **Go 1.27.3 or newer**:

```sh
go install github.com/Hell077/portguard@latest
```

Ensure your Go binary directory (`go env GOBIN`, or `$(go env GOPATH)/bin` when GOBIN is empty) is on `PATH`.

Without Go, download a [release binary](https://github.com/Hell077/portguard/releases/latest) for your operating system and architecture, or use an installer:

```sh
# Linux / macOS — verifies the release SHA-256 checksum
curl -fsSL https://raw.githubusercontent.com/Hell077/portguard/main/scripts/install.sh | sh
```

```powershell
# Windows PowerShell — verifies the release SHA-256 checksum
iwr https://raw.githubusercontent.com/Hell077/portguard/main/scripts/install.ps1 -UseBasicParsing | iex
```

The Unix installer writes to `~/.local/bin`; Windows uses `%LOCALAPPDATA%\Portguard`. Add that directory to `PATH` if needed. Override the destination with `PORTGUARD_INSTALL_DIR`. No administrator privileges are required for installation.

Release targets: Linux, macOS, Windows × amd64, arm64. macOS requires version 13+. Linux termination requires kernel 5.3+ with pidfd support; inspection also works on older kernels. Other Go targets may build from source, but are outside the tested MVP support matrix.

## Commands

```sh
# All visible listening TCP and bound UDP ports
portguard list
portguard list --range 3000-9000
portguard list --protocol tcp --filter postgres

# Details for every owner of a port, across protocols and addresses
portguard inspect 8080

# Graceful shutdown of every visible owner, after confirmation
portguard kill 8080

# Explicit force / confirmation controls
portguard kill 8080 --force
portguard kill 8080 --yes
portguard kill 8080 --force --yes

# Refresh every two seconds; Ctrl+C stops monitoring
portguard watch 8080
portguard watch 8080 --interval 1s

# Structured output; watch emits newline-delimited JSON
portguard list --json
portguard inspect 8080 --json
portguard watch 8080 --json

portguard --help
portguard kill --help
portguard --version

# Live overview; / filters, ↑↓ or j/k navigate, r refreshes, c clears, q quits
portguard
```

`list` returns an empty result successfully when nothing matches. `inspect` and `kill` return an error when no owner is visible. Errors go to stderr and return exit code 1; successful commands and help return 0. JSON uses the same snapshot schema for list, inspect, and watch: `listeners`, `warnings`, `scanned_at`, `duration_ns`. Unknown PIDs are `0`; unknown metadata is empty rather than guessed.

Without an interactive terminal, the overview prints one plain snapshot and watch prints successive snapshots. `--no-containers` skips optional engine queries for list, inspect, watch, and the overview. `--once` prints a single overview. Ranges are inclusive and must be within 1–65535. Filtering is case-insensitive and includes parents, services, and container names.

## Termination safety

Inspection and monitoring are read-only. Only `kill` signals processes. Portguard never changes firewalls, exposes ports, restarts a service, or automatically requests elevated privileges.

- A port can have multiple owners, addresses, or TCP/UDP bindings. `kill` deduplicates PIDs and shows the complete visible target set. Terminating a process affects **every port it owns**.
- Confirmation requires the word `yes`; EOF or any other response cancels. `--yes` skips only the prompt, preserving all checks.
- Every owner must have a resolved PID, name, and creation time. All process handles are opened before signaling anyone. Owners and creation times are checked again after confirmation; any mismatch aborts.
- System PID 1, Portguard itself, its ancestors, container publications, and recognized container runtime proxies are protected. Stop containers through Docker or Podman instead.
- Linux uses pidfd-bound signals, refusing an unsafe numeric-PID fallback. Windows keeps a native process handle. These handles prevent a recycled PID from redirecting a termination request.
- macOS watches process exit with kqueue and checks the precise process start time immediately before signaling. **macOS has no public atomic PID-bound signal API; a small check-to-signal race remains.** Use the application's own shutdown mechanism if that residual risk is unacceptable.
- Graceful means SIGTERM on Linux/macOS and WM_CLOSE on Windows applications with windows. Windows headless processes return an explicit error: use their own shutdown interface, or choose `--force`. Graceful requests can be ignored; there is no universal graceful shutdown mechanism for arbitrary Windows processes.
- `--force` means SIGKILL on Linux/macOS and TerminateProcess on Windows. It may lose unsaved work. Portguard waits up to five seconds, verifies exit, and checks the port again. It never silently escalates after a timeout. A process that is supervised may immediately restart.
- Multiple-process shutdown is not atomic. If a later signal fails, the error reports how many earlier requests were sent. Review the current state before retrying.

## Discovery limits

Results reflect the current user and network namespace. OS permissions can hide sockets or metadata; a missing result does **not** prove a port is available or reachable. Ownership may change immediately after a snapshot. Exposure describes the bind address, not firewall policy or internet reachability. TCP client connections are excluded; UDP sockets are shown because UDP has no LISTEN state.

On macOS, socket discovery needs `lsof` (normally included by the OS). Native services come from Linux cgroups, the current macOS launchd domain, or Windows CIM. Missing service metadata stays unresolved. Portguard does not infer services from familiar port numbers and does not collect full command lines or environment variables.

Container enrichment uses installed Docker / Podman CLIs and running daemons. It displays published ports and Compose labels; it does not discover Kubernetes services, unpublished container ports, or every host-network container. Docker remote contexts and explicit remote Podman connections are skipped. A default Podman machine can represent local VM port forwarding; use `--no-containers` when working with a remotely configured engine. Published mappings without a host process have PID 0 and remain inspectable. Failed optional lookups produce warnings.

## Development

Feature-based architecture keeps ownership and OS integration separate:

```text
main.go                         executable and build version
internal/features/inspect/      port/process model, filtering, generic projection
internal/features/scan/         socket discovery and bounded process enrichment
internal/features/containers/   published-port and Compose metadata
internal/features/services/     native service-manager evidence
internal/features/terminate/    confirmed shutdown and native identity guards
internal/features/terminal/     commands, JSON, text, live UI
internal/platform/command/      bounded commands without a shell
internal/platform/search/       standard and experimental SIMD matching
docs/                          GitHub Pages introduction
```

```sh
go test ./...
go test -race ./...
go vet ./...
go build -trimpath -ldflags='-s -w' .

# Opt-in Go 1.27 SIMD experiment, with a standard-build fallback
GOEXPERIMENT=simd go test ./...
GOEXPERIMENT=simd go install github.com/Hell077/portguard@latest
```

Go 1.27 generic methods power typed projections of listener data. The opt-in SIMD build uses vector byte comparisons for filtering, with a scalar tail. SIMD is experimental and is **not** required by the default installation or prebuilt binaries. Its performance depends on query length and hardware; no universal speedup is claimed. Discovery resolves metadata only for owners, bounds concurrent work to eight processes, and refreshes without overlapping scans. The binary has no Node runtime or background daemon.

The terminal design is inspired by [Ink](https://github.com/vadimdemedes/ink). Ink is a React renderer; Portguard uses [Bubble Tea](https://github.com/charmbracelet/bubbletea) and Lip Gloss to keep the application native Go.

Make small, buildable commits and push incremental changes. Use [Conventional Commits](https://www.conventionalcommits.org/), for example `feat(inspect): show parent processes`, `fix(terminate): reject reused PIDs`, or `docs: clarify installation`. CI validates new commit subjects, runs native Linux/macOS/Windows tests, checks the SIMD build, and runs the Linux race detector. Version tags trigger six checksummed release binaries after native tests pass.

MIT licensed. See [LICENSE](LICENSE).
