# Feature Overview: User & Role Management

### Summary

The `users` domain answers two questions that the `auth` domain deliberately does not: **who exists** in the system, and **what each person is allowed to do**. Authentication only proves that a request belongs to a verified identity (a phone number via WhatsApp OTP, or an SSO subject). It is this feature that owns the account records those identities resolve to, and the roles attached to them. Accounts are created here — by seed migration (the initial owner) and, post-MVP, by invite — and are **never** minted automatically at login.

Concretely, this feature owns three tables — `users`, `roles`, and `user_roles` — and the logic that keeps them consistent. It is the single source of truth for account records and role-based access control (RBAC). Because the MVP runs as a single-user operational flow (the Factory Owner / Production Manager), most of this feature operates quietly in the background today, but the domain and data model are built so that adding Warehouse Operators post-MVP requires no schema migration.

### Where It Fits

This feature sits between authentication and the rest of the application. It never handles credentials or sessions itself; `auth` calls into it at login, and reads its role data from the token thereafter.

```text
  LOGIN (once per session)
  ┌────────┐  ResolveUserByPhone → user + roles   ┌───────┐
  │  auth  │ ───────────────────────────────────▶ │ users │
  │ issues │  RecordLogin → last_login_at written │       │
  │  JWT,  │ ───────────────────────────────────▶ └───────┘
  │ stamps │
  │ roles  │  PROTECTED REQUESTS (every call)
  └────────┘  RequireAuth / RequireRole read the roles from the
              JWT claims auth stamped — users is NOT queried again
```

Three boundaries are worth stating explicitly so the earlier confusion does not creep back in:

- **Not authentication.** OTP generation/validation and session/JWT issuance live in `auth`. This feature has no login screen and stores no OTPs or sessions.
- **Not enforcement.** `RequireAuth` and `RequireRole` live in `auth` and read the user's roles from the JWT claims stamped at login. This feature supplies that role data once, at login; it is not consulted on each protected request.
- **Not partner roles.** The `partner_roles` table (vendor / cutter / tailor / client) classifies external partners and lives in the `partners` domain. Despite the shared word "role," it is unrelated to user authorization.

### Responsibilities

- **Account provisioning.** Create accounts by seed migration (the initial owner) and, post-MVP, by invite. Accounts are **never** auto-created at login — an OTP request for an unknown number is a deliberate no-op in `auth`, so login can only ever resolve an account that already exists.
- **Login support.** Resolve an account by phone for `auth` (read-only) and record `last_login_at` when `auth` reports a successful verification.
- **Account lifecycle.** Maintain profile fields (`full_name`, `email`, `phone_number`) and the `is_active` flag used to deactivate an account without deleting its history.
- **Role catalogue.** Manage the set of available roles (`roles` table) — their names and descriptions.
- **Role assignment.** Attach and detach roles for a given user (`user_roles` join table).
- **Authorization data source.** Provide the user's roles to `auth` at login so they can be stamped into the access token. Enforcement (`RequireAuth` / `RequireRole`) lives in `auth` and reads those claims; this feature owns the role _data_, not the _enforcement point_.

### Data Model

This feature owns the following existing tables (no schema change required):

- **`users`** — `id`, `email` (unique), `phone_number` (unique), `full_name`, `is_active` (default `true`), `last_login_at`, `created_at`. Note there is no password column: the system is passwordless by design.
- **`roles`** — `id`, `name` (unique), `description`, `created_at`.
- **`user_roles`** — composite primary key (`user_id`, `role_id`), both foreign keys with `ON DELETE CASCADE`.

### Cross-Domain Contract

The key integration is a pair of calls that `auth` makes during login. Neither creates accounts, and `auth` never writes to these tables directly:

```text
ResolveUserByPhone(ctx, phone) -> (User, []Role, error)   // read-only
  • returns the active account matching the phone, plus its roles
  • returns "not found" for an unknown OR inactive number — auth turns
    both into the same generic success, so no existence is ever leaked
  • performs no writes; creates nothing

RecordLogin(ctx, user_id) -> error                        // single write
  • stamps last_login_at = now() for the resolved user
  • called by auth only after a successful OTP verification
```

`auth` holds a **read-only lease** on the `users` / `roles` tables for the resolve step and, for simplicity, may run those reads from its own repository (this matches the built `auth` feature, whose `repository.go` is explicitly read-only). Every _write_ to these tables — including `last_login_at` — goes through this feature. `auth` stamps the returned roles into the access-token claims, so authorization needs no further lookup. This keeps a clean separation: `auth` decides _whether a session is granted_, while `users` decides _what account and roles back it_.

### API Surface (proposed)

All routes sit behind `RequireAuth`. There is intentionally no current-user endpoint here — that is served by `auth`'s `GET /auth/me` from the token claims. The "MVP?" column marks what the single-user flow actually needs now versus what the data model already supports for later.

```text
| Method & Path                        | Purpose                       | MVP? |
| ------------------------------------ | ----------------------------- | ---- |
| `GET /api/roles`                     | List the role catalogue       | Yes  |
| `GET /api/users/{id}/roles`          | List a user's roles           | Yes  |
| `POST /api/users/{id}/roles`         | Assign a role (`{ role_id }`) | Yes  |
| `DELETE /api/users/{id}/roles/{rid}` | Revoke a role                 | Yes  |
| `GET /api/users`                     | List users                    | Post |
| `GET /api/users/{id}`                | Get a single user             | Post |
| `POST /api/users`                    | Invite / create a user        | Post |
| `PATCH /api/users/{id}`              | Edit profile fields             | Post |
| `POST /api/users/{id}/deactivate`    | Set `is_active = false`       | Post |
| `POST /api/users/{id}/activate`      | Set `is_active = true`        | Post |
| `POST /api/roles`                    | Create a role                 | Post |
| `PATCH /api/roles/{id}`              | Edit a role                   | Post |
```

Each route flows through the standard Handler → Service → Repository layering: handlers parse and validate HTTP, the service enforces business rules (e.g. you cannot revoke a user's last remaining role if that leaves them unable to act), and the repository runs the raw SQL against Postgres.

### Key Flows

**1. Login resolution (no auto-provisioning)**

1. `auth` verifies the OTP and obtains a trusted phone number.
2. `auth` calls `ResolveUserByPhone(phone)`.
3. If no active account matches, `users` returns not-found. `auth` has already returned a generic success to the caller, so the flow simply stops — **no row is created**.
4. If an active account matches, `users` returns it with its roles; `auth` calls `RecordLogin(user_id)`, stamps the roles into the JWT, and issues the token pair.

Accounts must already exist for step 4 to succeed: the owner via the seed migration, additional users via invite (post-MVP).

**2. Assigning a role**

1. An authorized request hits `POST /api/users/{id}/roles` with a `role_id`.
2. The service confirms the target user and role both exist, and that the assignment is not a duplicate.
3. The repository inserts a `user_roles` row inside a transaction.
4. The updated role set is returned.

### Scope: MVP vs Post-MVP

|Capability|MVP|Post-MVP|
|---|---|---|
|Provision the owner account (seed migration)|Yes|Invite-based user creation|
|Seed default roles (`OWNER`; `OPERATOR` held)|Yes|—|
|Resolve account + record login for `auth`|Yes|—|
|Assign / revoke roles|Yes (API-level)|UI + audit trail|
|User CRUD (invite, edit, deactivate)|Data model ready, no UI|Full management screens|
|Multiple operators (`OPERATOR` role)|Supported by schema|Activated with new roles|
|Fine-grained permissions beyond role names|No|Optional permissions table|

### Acceptance Criteria (MVP)

- The seeded owner account exists before the first login; logging in **resolves** it rather than creating it.
- Every successful verification updates `last_login_at` via `RecordLogin`; no login path ever inserts a `users` row.
- A default role (`OWNER`) is seeded and attached to the owner account.
- An OTP request for an unknown or inactive number never creates an account and is indistinguishable from the registered case (`ResolveUserByPhone` returns not-found for both).
- A user with `is_active = false` cannot obtain a session, even with a valid OTP.
- Assigning or revoking a role is reflected immediately in `user_roles` and `GET /api/users/{id}/roles`, and takes effect in that user's access token after their next login or refresh.

### Out of Scope

- Authentication mechanics — OTP generation/validation, session/JWT issuance (owned by `auth`).
- Current-user endpoint — served by `auth`'s `GET /auth/me`, which reads the JWT claims.
- Auth middleware — `RequireAuth` and `RequireRole` are owned by `auth`; this feature only supplies the role data they read.
- Partner classification via `partner_roles` (owned by `partners`).
- Identity-provider configuration for SSO.
- Per-action permission policies; the MVP authorizes on role name alone.

### Open Decisions

**Resolved** (carried over from the already-built `auth` feature):

- **Enforcement.** `RequireAuth` / `RequireRole` live in `auth` and read roles from the JWT claims — no per-request `users` lookup. A consequence: a role change takes effect on the user's **next login or token refresh**, not instantly. This is acceptable given the short access-token lifetime, and is documented here so it's a known behavior rather than a surprise.
- **Read seam.** `auth` reads `users` / `roles` directly under a read-only lease; all writes go through this feature. (The stricter alternative — a narrow read interface exposed by `users` — remains available later if the lease ever feels too loose.)

**Open:**

- **Default role seed.** Confirm the seeded set and naming. The `auth` doc uses `OWNER` / `ADMIN` for full access and `OPERATOR` (post-MVP) for warehouse movements; this doc now matches. Seeded via a migration SQL script, owned by this feature since it touches `users` / `roles` tables.
- **Role deletion semantics.** `user_roles.role_id` is `ON DELETE CASCADE`, so deleting a role silently strips it from every user who held it. Decide whether to block deletion of in-use roles or soft-delete instead.
- **Profile editing.** Whether users may edit their own profile, or whether profile changes are admin-only.
