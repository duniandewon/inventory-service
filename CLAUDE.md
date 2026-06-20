# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

Textile Factory MVP: a Go backend tracking the lifecycle of raw materials through a textile manufacturing supply chain (Location/Custody, Transformation, and Lineage Traceability), as materials move Warehouse → Cutter/Tailor → Customer. See `context/porject-overview.md` for the full product spec and database schema, and `README.md` for setup/infra details.

## Commands

```bash
# Run infra + API via Docker (Postgres, Redis, pgAdmin, RedisInsight, API)
docker compose up --build -d

# Build
go build ./...

# Static analysis (required before committing)
go vet ./...
golangci-lint run

# Run a single package's tests / a single test
go test ./internal/inventory/...
go test ./internal/inventory/ -run TestName -v
```

There is no migration CLI/ORM. Schema changes are raw, sequentially numbered SQL files under `db/migrations/` (e.g. `001_initial_schema.sql`), applied manually via `psql` or pgAdmin before running new code that depends on them.

Local services once `docker compose` is up:

- Go API: `http://localhost:8080/health`
- pgAdmin: `http://localhost:5050` (connect to host `postgres`, user/pass from `.env`)
- RedisInsight: `http://localhost:8001` (connect to host `redis`, port `6379`)

## Architecture

Standard Go layout, organized **by business domain**, not by technical layer:

- `cmd/server/main.go` — entry point only: loads `config.Env`, opens the DB connection pool, wires the chi router, starts/gracefully shuts down the HTTP server. No business logic here.
- `internal/config/` — `Env` struct populated from environment variables (via `godotenv` + `os.Getenv`); fails fast (`log.Fatalf`) if a required var is missing.
- `internal/auth/` — Users, OTP (passwordless WhatsApp OTP / SSO), Sessions.
- `internal/inventory/` — Products (blueprints) and physical stock (the `inventory` table).
- `internal/logistics/` — the workflow engine: Work Orders and Delivery Notes. This is the core business loop.
- `internal/partners/` — Vendors, Cutters, Tailors, Customers.

Each domain package internally follows **Handler → Service → Repository** layering:

- `handler.go` — HTTP routing, request parsing/validation, JSON response formatting. No business logic.
- `service.go` — business rules (e.g. checking inventory stock is sufficient before a work order proceeds).
- `repository.go` (or `store.go` for logistics, where complex multi-table SQL transactions live) — raw SQL queries, manual `rows.Scan()` mapping, transaction management.

### The core domain loop

The system doesn't track static items — it tracks the *lifecycle* of inventory through state transitions:

1. **Receive Goods** → new `inventory` row, status `AVAILABLE`.
2. **Create Work Order** (select a `process_type` + `partner`) → **Assign Inputs**: marks selected inventory `CONSUMED`/`IN_PROGRESS` (`work_order_line_items` direction `INPUT`).
3. **Receive Outputs**: logs new `inventory` rows linked to that work order (direction `OUTPUT`), enabling backward traceability from a finished good to its originating raw material batch.
4. **Create Delivery Note**: selects finished `inventory`, generates a delivery note, flips inventory status to `SHIPPED`.
5. Dashboard queries `inventory` (status `AVAILABLE`) for current stock and `work_orders` (status `PROCESSING`) for WIP/what vendors currently hold.

Key enums: `item_type` (`RAW`/`SEMI_FINISHED`/`FINISHED`), `inventory_status` (`AVAILABLE`/`IN_PROGRESS`/`CONSUMED`/`SHIPPED`), `order_status` (`PENDING`/`PROCESSING`/`COMPLETED`), `io_direction` (`INPUT`/`OUTPUT`).

## Database & SQL conventions

- Standard library `database/sql` + `github.com/lib/pq` only — **no ORM, no `pgx`, no `sqlc`**.
- Raw SQL lives in the repository layer as constants/string literals. Always use parameterized queries (`$1, $2`); never concatenate query strings.
- Manually scan rows into structs with `rows.Scan()`; use `sql.NullString`/`sql.NullInt64`/etc. for nullable columns.
- Multi-table writes (e.g. work order input/output processing, shipping) must use an explicit `*sql.Tx` (`BEGIN`/`COMMIT`/`ROLLBACK`) passed down with context, managed in the repository layer.

## Go conventions

- Files: `snake_case.go`. Packages: short, single-word, lowercase.
- Receiver variables: 1-2 letter type abbreviation (`func (s *Service) Create(...)`), not full words.
- Define interfaces where consumed, not where implemented; keep them small (1-3 methods); end action interfaces in "-er". Accept interfaces, return structs.
- No `panic()` for control flow — always return `error` as the last return value and wrap with `fmt.Errorf("...: %w", err)` for traceability.
- Never expose raw DB/internal errors to HTTP clients — map to standardized JSON error responses with appropriate status codes.
- Validate DTOs at the handler layer (struct tags, e.g. `go-playground/validator`); the service layer does a second pass for domain-specific validation (e.g. sufficient stock).
- Keep functions short; if a function exceeds ~50 lines, look for a way to split it.
- Format with `gofmt`/`goimports` before committing.

## Workflow expectations (from context/ai-interaction.md)

This repo follows a fixed feature workflow — follow it rather than improvising:

1. Document the feature/fix in `context/current-feature.md` first.
2. Create a branch: `feature/[name]` or `fix/[name]`.
3. Implement exactly what's documented in `context/current-feature.md` — no extra "nice to have" features, no unrelated refactors.
4. Test: verify endpoints manually (API client or raw SQL), then `go build ./...` and `go vet`/`golangci-lint run`.
5. Iterate as needed.
6. Commit only after the build passes and behavior is verified — **never commit without explicit user permission**.
7. Rebase the feature branch onto main.
8. Delete the branch once rebased (ask first).
9. Mark the feature completed in `context/current-feature.md` and append it to that file's History section.

Additional standing rules:

- Ask before large refactors or architectural changes; never delete files without clarification.
- If something isn't working after 2-3 attempts, stop and explain rather than continuing to try random fixes.
- Make minimal changes scoped to the task; preserve existing patterns rather than introducing new ones.
