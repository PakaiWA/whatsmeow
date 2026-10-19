# AGENTS.md — Workspace Guidelines for AI Agents

Welcome to the `github.com/PakaiWA/whatsmeow` repository. This file serves as the primary guidance and orientation for AI coding assistants working in this codebase.

## 1. Project Memory & Context Documentation

Comprehensive architecture, codebase mapping, architectural decisions, and project tracking documentation are maintained inside the [`.context/`](file:///.context/) directory:

- [`.context/PROJECT_CONTEXT.md`](file:///.context/PROJECT_CONTEXT.md) — System boundaries, domain terms, runtime constraints, coding conventions, and testing standards.
- [`.context/ARCHITECTURE.md`](file:///.context/ARCHITECTURE.md) — Technical architecture, component diagrams, request lifecycles, concurrency models, and mock socket test design.
- [`.context/CODEBASE_MAP.md`](file:///.context/CODEBASE_MAP.md) — Sub-package inventory, module responsibilities, entry points, and test framework maps.
- [`.context/DECISIONS.md`](file:///.context/DECISIONS.md) — Append-only Architecture Decision Records (ADRs).
- [`.context/TODO.md`](file:///.context/TODO.md) — Technical debt, pending tasks, and code annotations (`TODO`, `FIXME`).

> **Note**: Always keep `.context/` documentation up to date when introducing architectural changes, adding new modules, or deciding significant patterns.

---

## 2. Core Architectural & Code Conventions

1. **Module Namespace**:
   Always use `github.com/PakaiWA/whatsmeow` as the Go module path. Do not revert to upstream `go.mau.fi/whatsmeow`.
2. **Device Identity**:
   Maintain the custom platform name `PakaiWA` in `DeviceProps OS`.
3. **Unit Testing Standards**:
   - Every core module must have pure unit tests with statement coverage ≥ 80%.
   - Tests must run completely in-memory with zero network dependencies using the mock socket test harness (`mockTestSocket` / `testutils_test.go`).
   - All tests must pass race condition checks: `go test -v -race -coverprofile=coverage.txt -covermode=atomic ./...`.
4. **Non-Destructive & Safe Refactoring**:
   - Never remove existing docstrings or comments unrelated to your changes.
   - Preserve existing method signatures to avoid breaking consumer microservices.
5. **Git & Commit Standards**:
   - Follow Conventional Commits (`feat:`, `fix:`, `test:`, `docs:`, `refactor:`, `chore:`).
   - All pre-commit hooks (`pre-commit run`) must pass before pushing commits.
