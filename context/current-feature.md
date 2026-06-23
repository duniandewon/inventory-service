# Current Feature

<!-- Feature Name -->

User & Role Management

## Status

<!-- Not Started|In Progress|Completed -->

Completed

## Goals

<!-- Goals & requirements -->

Based on `context/features/users.md`:

- Own the `users`, `roles`, and `user_roles` tables — the single source of truth for account records and role-based access control (RBAC).
- Account provisioning: seed the initial owner account by migration; accounts are never auto-created at login. Invite-based creation is post-MVP.
- Login support for `auth`: `ResolveUserByPhone(ctx, phone) -> (User, []Role, error)` (read-only) and `RecordLogin(ctx, user_id) -> error` (single write to `last_login_at`).
- Role catalogue management (`roles` table) and role assignment/revocation (`user_roles` join table).
- MVP API surface (all behind `RequireAuth`): `GET /api/roles`, `GET /api/users/{id}/roles`, `POST /api/users/{id}/roles`, `DELETE /api/users/{id}/roles/{rid}`.
- Seed a default `OWNER` role and attach it to the seeded owner account.
- Out of scope: OTP/session/JWT mechanics, `RequireAuth`/`RequireRole` middleware, current-user endpoint, `partner_roles`, SSO config, per-action permissions — all owned by `auth` or other domains.

## Notes

<!-- Any extra notes -->

Open decisions to resolve during implementation (see `context/features/users.md` "Open Decisions"):

- Default role seed naming/set (`OWNER` vs `ADMIN`, `OPERATOR` post-MVP).
- Role deletion semantics — block deletion of in-use roles vs. soft-delete (note `user_roles.role_id` is `ON DELETE CASCADE`).
- Whether users may edit their own profile or it's admin-only.

## History

<!-- Keep this updated. Earliest to latest -->

- 2026-06-21: Authentication feature documented and started, based on `context/features/authentication.md`.
- 2026-06-22: Authentication implemented on `feature/authentication` branch: OTP request/verify, JWT access+refresh issuance/rotation/reuse-detection, logout, `RequireAuth`/`RequireRole` middleware, `/auth/me`. Open decisions resolved: signed-JWT refresh tokens, 15min/7day TTLs, stubbed `LoggingOTPSender`, "logout everywhere" via `user_sessions` set built now, no seed migration added. Manually verified end-to-end against dockerized Postgres/Redis; `go build`/`go vet`/`gofmt` clean. Committed, rebased and fast-forwarded onto `main`, branch deleted. Status: Completed.
- 2026-06-23: User & Role Management feature documented and started, based on `context/features/users.md`.
- 2026-06-23: Implemented on `feature/user-role-management` branch: new `internal/users` domain (Handler → Service → Repository) owning `users`/`roles`/`user_roles` writes; `GET /api/roles`, `GET/POST /api/users/{id}/roles`, `DELETE /api/users/{id}/roles/{roleID}`, all behind `RequireAuth`. Added `db/migrations/001_initial_schema.sql` (full schema, previously only applied manually and never committed) and `002_seed_owner_and_roles.sql` (seeds `OWNER`/`OPERATOR` roles and a placeholder owner on a fresh DB; idempotently normalizes a pre-existing lowercase `owner` role to `OWNER` and skips the placeholder user when a real owner already exists). Wired `auth.Service` to call `users.Repository.RecordLogin` after successful OTP verification via a new `LoginRecorder` interface, closing a gap where `last_login_at` was never stamped. Manually verified end-to-end against dockerized Postgres/Redis: role list/assign/revoke, duplicate-assignment 409, missing-revoke 404, unauthenticated 401, and `last_login_at` stamping. `go build`/`go vet`/`gofmt` clean; `golangci-lint` not installed locally so skipped. Status: Completed.
