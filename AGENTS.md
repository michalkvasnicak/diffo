# Development guidelines

- Never run checks manually: formatting, lint, builds, and tests run automatically through the harness after your turn. Fix failures reported by the harness.
- Write human-readable, explicit Go: clear names, focused functions, and straightforward control flow.
- Keep `cmd/diffo` thin. Wire dependencies in `internal/app`; put behavior in focused service packages under `internal/`, separate from Bubble Tea models and rendering.
- Prefer concrete services and explicit dependency injection. Add interfaces at consuming boundaries only when needed; avoid generic frameworks and speculative abstractions.
- We control the code and callers. Backward compatibility is not a goal: change callers directly rather than adding compatibility layers.
- Handle real errors at system boundaries. Avoid redundant validation, defensive nil checks for required dependencies, and fallback paths for impossible states.
