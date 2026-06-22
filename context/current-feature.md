# Current Feature

Authentication (`internal/auth/`)

## Status

In Progress

## Goals

Passwordless WhatsApp OTP authentication issuing JWT access/refresh tokens, plus authorization primitives for the rest of the app.

1. **Identity** — verify a user owns a known phone number via WhatsApp OTP.
2. **Tokens** — issue access + refresh JWTs so a verified user stays logged in without re-verifying every request.
3. **Authorization primitives** — expose the user's roles (via token claims + middleware) so other features can guard their own routes.

In scope:

- WhatsApp OTP request + verify flow, backed by Redis (codes, counters, lockouts).
- Resend OTP, request caps, verify-attempt caps, temporary lockout.
- JWT access + refresh tokens with refresh-token rotation and reuse detection.
- Refresh and logout (refresh-token revocation).
- `RequireAuth` / `RequireRole` middleware for the rest of the app to use.

API surface:

- `POST /auth/otp/request` — `{ phone_number }` → generic 200; OTP sent if user exists (also handles resend).
- `POST /auth/otp/verify` — `{ phone_number, code }` → `{ access_token, refresh_token }`.
- `POST /auth/refresh` — `{ refresh_token }` → new rotated token pair.
- `POST /auth/logout` — `{ refresh_token }` → revokes refresh token.
- `GET /auth/me` — current user + roles (requires access token).

See `context/features/authentication.md` for the full design (Redis key reference, layering, security checklist, open decisions).

## Notes

Open decisions, as resolved with the user:

1. Refresh token: **signed JWT** (not opaque) — carries `jti`/`sub`/`exp`/`typ:"refresh"`; Redis still tracks the `jti` for revocation/rotation.
2. Token lifetimes: 15 min access / 7 day refresh (defaults in `config.Env`, overridable via `ACCESS_TOKEN_TTL` / `REFRESH_TOKEN_TTL` env vars).
3. OTP provider: stubbed for now — `LoggingOTPSender` logs the code instead of calling WhatsApp. Swap in a real client later behind the existing `OTPSender` interface.
4. "Logout everywhere": built now via `user_sessions:{user_id}` Redis set; reuse-detection on `/auth/refresh` calls it automatically.
5. Seed user: **not** added as a migration — assumed the `users`/`roles`/`user_roles` schema is already applied elsewhere.

Implemented in `internal/auth/`: `types.go`, `redis_store.go` (OTPStore/TokenStore), `repository.go` (read-only Postgres), `otp_sender.go` (stub), `jwt.go`, `service.go`, `handler.go`, `middleware.go`, `routes.go`. Wired into `cmd/server/main.go` (Redis client + route mounting). Added `internal/config` tunables: `OTP_LENGTH`, `OTP_TTL`, `OTP_MAX_REQUESTS`, `OTP_REQUEST_WINDOW`, `OTP_MAX_ATTEMPTS`, `OTP_LOCKOUT_TTL`, `ACCESS_TOKEN_TTL`, `REFRESH_TOKEN_TTL` (all have defaults). New deps: `github.com/redis/go-redis/v9`, `github.com/golang-jwt/jwt/v5`, `github.com/go-playground/validator/v10`.

Manually verified end-to-end via `docker compose up --build -d` against a seeded test user: OTP request (generic 200 for known/unknown phone), wrong-code rejection, correct verification issuing tokens, `/auth/me` with/without token, refresh rotation, reuse-detection revocation (401 on reused refresh token), and logout. `go build ./...` and `go vet ./...` pass; `gofmt` clean. `golangci-lint` not installed locally so it wasn't run.

Not yet committed — awaiting explicit go-ahead per the workflow.

## History

<!-- Keep this updated. Earliest to latest -->

- 2026-06-21: Feature documented and started, based on `context/features/authentication.md`.
- 2026-06-22: Implemented on `feature/authentication` branch: OTP request/verify, JWT access+refresh issuance/rotation/reuse-detection, logout, `RequireAuth`/`RequireRole` middleware, `/auth/me`. Manually verified against dockerized Postgres/Redis.
