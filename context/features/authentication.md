# Feature Overview: Authentication

_Domain: `internal/auth/` — the first feature of the Traceable Manufacturing & Logistics Engine._

## 1. Purpose & Scope

This feature answers one question: **is this request coming from a known, active user, and what roles do they hold?** It issues and validates the tokens that every other part of the app sits behind.

It is passwordless. A user proves identity by entering a 6-digit code sent to their WhatsApp number; on success they receive JWT access + refresh tokens.

Three jobs:

1. **Identity** — proving a person owns a known phone number via WhatsApp OTP.
2. **Tokens** — issuing access + refresh JWTs so a verified user stays logged in without re-verifying every request.
3. **Authorization primitives** — exposing the user's roles (via token claims + middleware) so other features can guard their own routes.

### In scope

- WhatsApp OTP request + verify flow, backed by **Redis** (codes, counters, lockouts).
- Resend OTP, request caps, verify-attempt caps, temporary lockout.
- **JWT access + refresh tokens** with refresh-token rotation and reuse detection.
- Refresh and logout (refresh-token revocation).
- `RequireAuth` / `RequireRole` middleware for the rest of the app to use.

## 2. Storage Split (Redis vs PostgreSQL)

|Data|Store|Why|
|---|---|---|
|Users, roles, role assignments|**PostgreSQL**|Permanent, relational, audited. Auth reads these; another feature writes them.|
|OTP codes, attempt/request counters, lockouts|**Redis**|Ephemeral; TTL auto-expires them — no cleanup jobs, no DB bloat.|
|Refresh tokens / sessions|**Redis**|Fast lookups on the refresh path; deleting a key = instant logout/revoke.|
|Access tokens|**Nowhere (stateless)**|Validated by JWT signature alone — no store, no lookup.|

No `otp_codes` or `sessions` tables in Postgres. Auth relies only on the existing `users`, `roles`, and `user_roles` tables — and reads them, never writes them.

## 3. Roles (as auth sees them)

Auth doesn't create or assign roles — it just reads whatever a user already has and stamps them into the access token so route guards need no DB lookup.

|Role (MVP)|Capability|
|---|---|
|`OWNER` / `ADMIN`|Full access.|
|`OPERATOR` _(post-MVP)_|Warehouse goods movements. Defined so RBAC is ready later.|

Authorization asks "does this user have a role that permits X," never "is this user ID == 1."

An OTP request for an unknown or inactive phone number returns a **generic success** — auth never reveals whether a number is registered.

## 4. Core Flows

### 4.1 Request OTP — `POST /auth/otp/request`

```
Input: { phone_number }

1. If otp:lockout:{phone} exists → reject (locked out).
2. INCR otp:requests:{phone}; on first set, EXPIRE to the request window (e.g. 15m).
   - If counter > MAX_REQUESTS (e.g. 3) → set otp:lockout:{phone} (e.g. 30m), reject.
3. Look up user by phone. If not found/inactive → return generic 200 (no leak), stop.
4. Generate 6-digit code. Store in Redis:
     HSET otp:code:{phone} code_hash <hash> attempts 0
     EXPIRE otp:code:{phone} 300            # 5-min validity
5. Send plaintext code via WhatsApp Business API.
6. Return generic 200.
```

**Resend** is the same endpoint — step 2's counter naturally caps how often a code can be (re)sent.

### 4.2 Verify OTP — `POST /auth/otp/verify`

```
Input: { phone_number, code }

1. If otp:lockout:{phone} exists → reject.
2. GET otp:code:{phone}. Missing/expired → 401 ("expired or invalid").
3. HINCRBY attempts. If attempts > MAX_ATTEMPTS (e.g. 5) →
     DEL otp:code:{phone}; set otp:lockout:{phone}; reject.
4. Constant-time compare code vs code_hash.
   - Mismatch → 401.
   - Match    → DEL otp:code:{phone}; clear counters; issue token pair (§5.3).
```

### 4.3 Issue Tokens (on successful verify)

```
access_token  = signed JWT, ~15 min, claims: sub=user_id, roles, jti, iat, exp
refresh_token = opaque random string (recommended) OR signed JWT, ~7–30 days

Store refresh in Redis:
  SET refresh:{token_id} {user_id, issued_at} EX <refresh_ttl>
  (optionally SADD user_sessions:{user_id} {token_id} for "logout everywhere")

Return both tokens to the client.
```

### 4.4 Access a Protected Route

Middleware verifies the access-token signature and `exp` — **no Redis/DB hit** — then loads `user_id` + roles from the claims into request context. Bad/expired → `401`.

### 4.5 Refresh — `POST /auth/refresh`

```
Input: refresh_token

1. Look up refresh:{token_id} in Redis. Missing → 401 (expired/revoked) → client logs out.
2. ROTATE: DEL the old refresh key, issue a NEW access + refresh pair (§5.3).
3. Reuse detection: if a token_id that was already rotated reappears →
   treat as theft → revoke all of that user's sessions.
```

### 4.6 Logout — `POST /auth/logout`

`DEL refresh:{token_id}` (and remove from `user_sessions:{user_id}`). The access token stays valid until it expires — accepted tradeoff, mitigated by its short lifetime.

## 5. Token Design Decisions

**Access token** — short-lived (~15 min), stateless, signed (HS256 is fine for a single backend; RS256 if you later split issuer/verifier). Carries `sub` + `roles` so authorization needs no lookup.

**Refresh token — two valid shapes:**

||Opaque random string _(recommended)_|Signed JWT|
|---|---|---|
|Validation|Redis lookup only|Signature + Redis lookup|
|Revocable|Yes (delete key)|Yes (track `jti` in Redis)|
|Complexity|Lower — no claims/signature to manage|Higher|

Since the refresh token's only job is to find a session in Redis, an **opaque token is simpler and equally secure**. A JWT refresh token works too but buys little here.

**Why store refresh tokens at all (vs pure stateless JWT):** without server-side state you cannot revoke — logout and theft response become impossible until natural expiry. Redis gives instant revocation and reuse detection cheaply.

## 6. Redis Key Reference

```
otp:code:{phone}          HASH  { code_hash, attempts }     TTL ~5m
otp:requests:{phone}      INT   request/resend counter       TTL ~15m
otp:lockout:{phone}       flag  presence = locked out        TTL ~30m
refresh:{token_id}        data  { user_id, issued_at }       TTL = refresh lifetime
user_sessions:{user_id}   SET   token_ids (optional)         — for "logout everywhere"
```

Tunable constants (put in `config.Env`): OTP length, OTP TTL, MAX_REQUESTS, request window, MAX_ATTEMPTS, lockout duration, access TTL, refresh TTL.

## 7. API Surface

All public (no token required) except where noted:

|Method|Path|Body|Result|
|---|---|---|---|
|POST|`/auth/otp/request`|`{ phone_number }`|Generic 200; OTP sent if user exists. Also handles resend.|
|POST|`/auth/otp/verify`|`{ phone_number, code }`|Returns `{ access_token, refresh_token }`|
|POST|`/auth/refresh`|`{ refresh_token }`|Returns a new token pair (rotated)|
|POST|`/auth/logout`|`{ refresh_token }`|Revokes refresh token|
|GET|`/auth/me`|_(access token)_|Current user + roles|

## 8. Layering (Handler → Service → Repository)

- **`handler.go`** — HTTP only: decode/validate JSON, call service, map errors to status codes.
- **`service.go`** — the rules: OTP generation/verification, attempt & lockout logic, token issue/rotate/revoke. Talks to the WhatsApp client, the Redis store, and the user repository.
- **`repository.go`** — Postgres via `database/sql` + `lib/pq`: **read-only** here — look up a user by phone, load their roles. (User writes live in the user-management feature.)
- **`redis_store.go`** _(behind an `OTPStore` / `TokenStore` interface)_ — all Redis access for OTP keys and refresh tokens, so the service stays testable.
- **`middleware.go`** — `RequireAuth` (valid access token) and `RequireRole(role)` for the rest of the app to mount on protected chi routers.

The WhatsApp sender sits behind a small `OTPSender` interface so the service doesn't depend on a concrete provider — testable and swappable.

## 9. Security Checklist

- **Hash OTP codes** in Redis; constant-time compare on verify.
- **Short OTP TTL** (~5 min); single use (delete on success).
- **Cap requests/resends** and **verify attempts**; **lockout** on exceed (Redis counters with TTL).
- **Rate limit** `/auth/otp/request` per phone and per IP (chi middleware) on top of the per-phone counter.
- **Generic responses** on OTP request — never reveal whether a number is registered.
- **Short access-token lifetime**; **rotate refresh tokens**; **reuse detection** revokes the session.
- **Strong JWT secret** in `config.Env`; never commit it. Validate `exp`/`iat`; set issuer/audience if useful.
- **Never log** plaintext OTP codes, full phone numbers, or tokens.
- **TLS everywhere** — tokens travel in the `Authorization: Bearer` header.

## 10. Open Decisions (confirm before building)

1. **Refresh token: opaque vs JWT** — recommend opaque. Agree?
2. **Token lifetimes** — proposed ~15 min access / ~7–30 day refresh. Your numbers?
3. **OTP provider** — which WhatsApp Business API vendor (Meta Cloud API, Twilio, etc.)? Shapes the `OTPSender` impl.
4. **"Logout everywhere"** — needed now, or skip the `user_sessions` set for the MVP?
5. **Seed user** — confirm the owner is created via a seed migration so there's someone to log in as before user management exists.

## 11. Suggested Build Order

1. Config + Redis and Postgres connections; seed owner + roles via migration.
2. Redis store layer (`OTPStore`, `TokenStore`) + a stub `OTPSender` that logs the code.
3. Service: request/verify OTP with counters & lockout.
4. Token issue / refresh / rotate / revoke logic.
5. Handlers + `RequireAuth` / `RequireRole` middleware; wire one protected test route.
6. Swap the stub sender for the real WhatsApp client.
