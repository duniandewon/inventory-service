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
