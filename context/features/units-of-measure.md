# Feature Overview: Units of Measure

_Domain: `internal/reference/` — a shared lookup owned by no single domain._

## 1. Purpose & Scope

Units of Measure (UOM) is the canonical list of the units the business counts things in — yards, pieces, meters, rolls. It is a **flat catalogue**: a set of named units, nothing more. It lives in its own `reference` package because three different domains depend on it (`inventory`, `products`, and the workflow engine), so it belongs to none of them.

It deliberately does **not** know how units relate to one another. "1 roll = 100 yards" or "1 yard yields ~2 pieces" are _conversions_ — a separate, later concern (§8). This feature only answers **"what units exist?"**

### In scope

- CRUD over the unit list (create, list, rename, delete).
- Name uniqueness and basic normalization.
- Refusing to delete a unit that is in use.
- A seed set of starter units via migration.

## 2. Where It Fits

UOM is a leaf dependency: nothing it depends on, three things depend on it.

```text
        products.default_uom_id ──┐
        inventory.uom_id ─────────┼──▶  units_of_measure   (reference)
   work_order_line_items.uom_id ──┘
```

**Write / read split** (the same shape as how `auth` treats the `users` tables): every _write_ — creating, renaming, or deleting a unit — goes through the `reference` package. _Reads_ are open: consumer domains foreign-key straight to `units_of_measure` and join it in their own queries to show a unit's name. The foreign keys guarantee that any `uom_id` stored elsewhere is valid, so consumers never need to call into `reference` to validate — the database enforces it at write time.

So `reference` owns the management surface and the seed; every other domain just points at the table.

## 3. Data Model

One table, exactly as it stands in the schema:

```sql
units_of_measure (
  id    SERIAL PRIMARY KEY,
  name  VARCHAR(50) NOT NULL UNIQUE
)
```

That is the whole feature's state. No symbol/abbreviation column, no dimension or category, no conversion factors — each of those is a deliberate future decision (§8, §10), not part of the MVP.

The three inbound foreign keys (`products.default_uom_id`, `inventory.uom_id`, `work_order_line_items.uom_id`) carry no `ON DELETE` clause, so Postgres **blocks deletion** of any unit currently referenced. That database behavior is the backstop behind the in-use rule in §5.

## 4. API Surface

All routes sit behind `RequireAuth`; managing units is an `OWNER` / `ADMIN` activity.

| Method | Path | Body | Result |
|---|---|---|---|
| GET | `/api/uom` | — | List all units |
| POST | `/api/uom` | `{ name }` | Create a unit |
| PATCH | `/api/uom/{id}` | `{ name }` | Rename a unit |
| DELETE | `/api/uom/{id}` | — | Delete a unit (`409` if in use) |

`GET` is the one other features lean on — it backs the unit dropdowns on the Product and Receiving screens — though those screens may equally join the table server-side.

## 5. Key Behaviors & Validation

- **Uniqueness.** `name` is `UNIQUE` at the database level; the service catches the violation and returns a friendly "unit already exists" rather than a raw constraint error.
- **Normalization.** Trim surrounding whitespace and reject empty names. Settle case handling (§10) so "Yards" and "yards" don't both land in the list.
- **In-use deletion.** Before deleting, check whether the unit is referenced by any product, inventory row, or work-order line item. If it is, refuse with `409 Conflict` and a clear message. The FK is the hard backstop; the explicit check is what turns a database error into a clean API response.

### Seeding

A migration seeds the starter units the business already uses (e.g. yards, meters, pieces, rolls). This belongs in the **same migration set as the owner-account seed** from the auth work — both are "data that must exist before the app is usable." Seeding here unblocks Products and Receiving immediately.

## 6. Scope: MVP vs Post-MVP

| Capability | MVP | Post-MVP |
|---|---|---|
| CRUD + list | Yes | — |
| Seed starter units | Yes | — |
| Block deletion of in-use units | Yes | — |
| Short symbol/abbreviation (`yd`, `pc`, `m`) | No | Optional column |
| Unit categories / dimensions (length vs count vs weight) | No | Optional |
| **Conversions** between units | No | Separate feature + table |

## 7. Acceptance Criteria (MVP)

- The seeded units are present after migration and returned by `GET /api/uom`.
- Creating a unit with an existing name (modulo the agreed case rule) is rejected, not duplicated.
- A unit referenced by any product, inventory row, or work-order line item cannot be deleted; the API returns `409`, not a raw FK error.
- Renaming a unit updates it everywhere by reference — the `id` is the key, names are display only.

## 8. Out of Scope

- **Conversions.** Relationships between units ("1 roll = 100 yards," yield ratios) are a distinct future feature with its own table, likely referencing two UOMs per row. UOM stays a flat list until then.
- **Unit arithmetic.** Converting a quantity from one unit into another is conversion logic, not this feature.
- **Dimensional validation.** Enforcing that a length product can't be measured in a count unit needs a category/dimension model — deferred.

## 9. Layering (Handler → Service → Repository)

- **`handler.go`** — decode/validate JSON, map errors to status codes (the in-use case → `409`).
- **`service.go`** — normalization, uniqueness messaging, and the in-use check before a delete.
- **`repository.go`** — raw SQL via `database/sql` + `lib/pq`: list, insert, update, delete, plus a "is this unit referenced?" count query.

Given the feature's size, a service layer is borderline — but the in-use check and name normalization are real rules, so the thin service earns its place and keeps the pattern uniform with the rest of the codebase.

## 10. Open Decisions

1. **Case sensitivity of names.** Treat "Yards" / "yards" as the same unit (recommended — case-insensitive uniqueness) or as distinct? Postgres `UNIQUE` is case-sensitive by default, so this shapes both normalization and the index.
2. **Symbol / abbreviation now or later.** Add a short `symbol` column (`yd`, `pc`, `m`) for compact display in this pass, or defer? Cheap to add, but it's a schema change — so a conscious call, not a default.
3. **Delete vs deactivate.** Hard delete (blocked when in use) is proposed. If you'd rather retain historical units, an `is_active` flag would mirror how `users` handles deactivation — likely overkill for a unit list, but worth a moment's thought.
