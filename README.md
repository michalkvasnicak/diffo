# diffo

A Git diff TUI and CLI written in Go, planned around Charm's **Bubble Tea** terminal UI framework.

The goal is to review Git changes interactively in the terminal. This repository currently contains only the CLI scaffold and development setup; Git integration and the TUI are not implemented, and Bubble Tea is not yet a dependency.

## Getting started

Requires Go 1.27 or newer.

```sh
go run ./cmd/diffo
```

To build or install the CLI:

```sh
go build -o bin/diffo ./cmd/diffo
go install ./cmd/diffo
```

The current executable only prints a placeholder message.

## Structure

- `cmd/diffo/`: executable entry point and process-level error handling.
- `internal/app/`: application bootstrap and dependency wiring.
- Add focused service packages under `internal/` as behavior is implemented; keep Git operations separate from Bubble Tea state and rendering.

The module is named `diffo` until a canonical repository URL is chosen.

## Automatic checks

`.pi/settings.json` loads `@michalkvasnicak/pi-hooks`, following the setup in `../piup`. Start Pi from the repository root and grant project trust. Reload Pi after initially adding the package.

Before the agent finishes, `.pi/pi-hooks.json` runs these checks in order:

1. Format Go source with `gofmt -w`.
2. Run Go's built-in static analysis with `go vet ./...`.
3. Run tests with `go test -race ./...` (requires a C compiler for the race detector).

Checks rerun when their inputs change, and failures return to the agent for correction. No tests are included yet.
