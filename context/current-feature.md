# Current Feature

<!-- Feature Name -->

## Status

<!-- Not Started|In Progress|Completed -->

Not Started

## Goals

<!-- Goals & requirements -->

## Notes

<!-- Any extra notes -->

## History

<!-- Keep this updated. Earliest to latest -->

- 2026-06-21: Authentication feature documented and started, based on `context/features/authentication.md`.
- 2026-06-22: Authentication implemented on `feature/authentication` branch: OTP request/verify, JWT access+refresh issuance/rotation/reuse-detection, logout, `RequireAuth`/`RequireRole` middleware, `/auth/me`. Open decisions resolved: signed-JWT refresh tokens, 15min/7day TTLs, stubbed `LoggingOTPSender`, "logout everywhere" via `user_sessions` set built now, no seed migration added. Manually verified end-to-end against dockerized Postgres/Redis; `go build`/`go vet`/`gofmt` clean. Committed, rebased and fast-forwarded onto `main`, branch deleted. Status: Completed.
- 2026-06-23: User & Role Management feature documented and started, based on `context/features/users.md`.
- 2026-06-23: Implemented on `feature/user-role-management` branch: new `internal/users` domain (Handler → Service → Repository) owning `users`/`roles`/`user_roles` writes; `GET /api/roles`, `GET/POST /api/users/{id}/roles`, `DELETE /api/users/{id}/roles/{roleID}`, all behind `RequireAuth`. Added `db/migrations/001_initial_schema.sql` (full schema, previously only applied manually and never committed) and `002_seed_owner_and_roles.sql` (seeds `OWNER`/`OPERATOR` roles and a placeholder owner on a fresh DB; idempotently normalizes a pre-existing lowercase `owner` role to `OWNER` and skips the placeholder user when a real owner already exists). Wired `auth.Service` to call `users.Repository.RecordLogin` after successful OTP verification via a new `LoginRecorder` interface, closing a gap where `last_login_at` was never stamped. Manually verified end-to-end against dockerized Postgres/Redis: role list/assign/revoke, duplicate-assignment 409, missing-revoke 404, unauthenticated 401, and `last_login_at` stamping. `go build`/`go vet`/`gofmt` clean; `golangci-lint` not installed locally so skipped. Status: Completed.
- 2026-06-24: Units of Measure feature documented and started, based on `context/features/units-of-measure.md`.
- 2026-06-24: Implemented on `feature/units-of-measure` branch: new `internal/reference` domain (Handler → Service → Repository) owning `units_of_measure` writes; `GET /api/uom` (any authenticated user), `POST/PATCH/DELETE /api/uom[/{id}]` (OWNER only — no `ADMIN` role exists yet, so OWNER stands in for it). Added `db/migrations/003_uom_case_insensitive_uniqueness.sql`: resolves the case-sensitivity open decision with a `UNIQUE INDEX ... (LOWER(name))`, and seeds the starter units (Yards/Meters/Pieces/Rolls). The other two open decisions (no symbol column, hard delete over `is_active`) were already implicit in the existing `units_of_measure` schema from `001_initial_schema.sql`, so no further migration was needed for those. In-use deletion check covers `products.default_uom_id` and `inventory.uom_id`; `work_order_line_items.uom_id`, named in the feature doc's FK diagram, does not actually exist as a column in `001_initial_schema.sql`, so it was excluded from the check rather than queried against a nonexistent column. Manually verified end-to-end against dockerized Postgres/Redis: list, create, case-insensitive duplicate rejection (`409`), empty-name rejection (`400`), rename, rename-conflict (`409`), delete, delete-missing (`404`), delete-in-use (`409`, via a temporary `products` row), and unauthenticated (`401`). `go build`/`go vet`/`gofmt` clean; `golangci-lint` not installed locally so skipped. Status: Completed.
