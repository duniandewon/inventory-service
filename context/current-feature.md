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
