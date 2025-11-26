# Repository Guidelines

## Project Structure & Module Organization
- `build/`: Dockerfiles per component.
- `cmd/`: Main binaries (gpuagent, migagent, gpupartitioner, metricsexporter).
- `config/`: Kubernetes manifests (CRDs, RBAC, webhook, per-component configs).
- `docs/`, `hack/`, `demos/`: Documentation, scripts, local dev helpers.
- `helm-charts/`: Helm chart definitions and docs.
- `internal/`: Controllers and internal packages.
- `pkg/`: Public packages (API types, scheduler lib, utilities, tests helpers).

## Build, Test, and Development Commands
- `make help`: List all available developer targets.
- `make build`: Build the main binary for local validation.
- `make test`: Run tests with integration tag and envtest; writes `cover.out`.
- `make lint`: Run `golangci-lint` with project rules.
- `make manifests`: Generate CRDs/RBAC/Webhooks for all components.
- `make cluster`: Create a local KIND cluster (`hack/kind/cluster.yaml`).
- `make docker-build-<component>`: Build Docker images (e.g., `make docker-build-operator`).

## Coding Style & Naming Conventions
- Language: Go. Format with `go fmt`; lint with `golangci-lint` (see `.golangci.yaml`).
- Indentation: tabs (Go default). Keep imports grouped; run `go fmt` before committing.
- Naming: follow standard Go conventions (lowercase package names, MixedCaps identifiers; avoid underscores in package names).
- Headers: ensure license headers are present; use `make license-check` / `make license-fix`.

## Testing Guidelines
- Framework: `go test` with controller-runtime envtest. Tests live alongside code as `*_test.go`.
- Conventions: prefer table-driven tests; keep tests deterministic; add coverage for edge cases.
- Run: `make test` (sets `KUBEBUILDER_ASSETS` automatically). Inspect coverage via `cover.out`.

## Commit & Pull Request Guidelines
- Commits: use Conventional Commits where possible (`feat:`, `fix:`, `chore:`, `docs:`). Keep them small and scoped.
- PRs: include a clear description, linked issues, and rationale. Note behavior changes and update docs/Helm values when relevant. Ensure `make test` and `make lint` pass.

## Security & Configuration Tips
- Do not commit secrets. Externalize configuration via Helm values or `config/` overlays.
- Images are published under `ghcr.io/walkai-org`. Use component specific Dockerfiles in `build/`.
