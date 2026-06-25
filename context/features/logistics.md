# Feature Overview: Logistics

_Domain: `internal/logistics/` — the workflow engine, outbound delivery, traceability, and process types. This is the heart of the system and the **only** place that runs multi-table, cross-domain transactions. Read it slowly; the rest of the codebase is lookups and CRUD by comparison._

## 1. Responsibilities & Boundaries

Logistics owns four things, of very different weights:

- **Work Orders** — the core transformation loop (`work_orders`, `work_order_line_items`). The hard part.
- **Delivery** — outbound shipping (`delivery_notes`, `delivery_note_items`).
- **Lineage** — the backward defect trace. A _read view_, not its own tables.
- **Process Types** — the engine's configurable vocabulary (`process_types`); a trivial lookup, folded in here.

**What it owns vs. touches.** Logistics owns the four table groups above. It **reads** master data by foreign key — `products`, `partners`, `units_of_measure` — and never writes them. The one exception, and the whole reason this domain is hard, is `inventory`: logistics must **write** the `inventory` table (owned by the `inventory` domain) as part of its transactions.

**The one-way dependency rule (hard).** `logistics` imports `inventory`; `inventory` must **never** import `logistics`. Inventory has no reason to know work orders exist. Keeping this one-directional is what prevents an import cycle and what keeps the cross-domain write pattern below safe. State it, enforce it in review.

## 2. The Keystone: the Inventory-Write Transaction Boundary

Every core logistics operation mutates tables in two domains and must be all-or-nothing. "Receive 200 cut pieces" has to, in one transaction: insert new `inventory` rows, flip the input roll's status, write line items for both sides, and advance the work order — or roll all of it back. Elsewhere, domains only _read_ each other's tables; here logistics must **write** another domain's table inside its own transaction, which is what makes this the hard part.

**The approach.** `inventory` exposes coarse, intention-named functions that each accept a transaction handle. Logistics opens the transaction, threads it through its own writes _and_ these inventory calls, and commits once. Inventory still owns every rule about its own rows; logistics owns the transaction boundary. This keeps inventory's write and status logic in one place — Inbound (Receiving) needs the same logic — without a separate orchestration layer, which a single-user MVP doesn't warrant.

**The threading pattern.** A minimal interface that both `*sql.DB` and `*sql.Tx` satisfy, so repository functions don't care which they get:

```go
type DBTX interface {
    ExecContext(ctx context.Context, q string, args ...any) (sql.Result, error)
    QueryContext(ctx context.Context, q string, args ...any) (*sql.Rows, error)
    QueryRowContext(ctx context.Context, q string, args ...any) *sql.Row
}
```

Every repository function in both packages takes a `DBTX`. Logistics calls `BeginTx`, passes the resulting `*sql.Tx` into its own writes and into inventory's functions, and commits at the end.

**Inventory's transaction-aware surface** (coarse and intention-revealing — _not_ a generic CRUD API). Each validates the legal status transition, because inventory owns that rule:

|Function|Status effect|Called by|
|---|---|---|
|`ReserveForWorkOrder(tx, invIDs, woID)`|`AVAILABLE → IN_PROGRESS`|Assign inputs|
|`ProduceFromWorkOrder(tx, outputs…) → []invID`|inserts new `AVAILABLE` rows|Receive outputs|
|`ConsumeForWorkOrder(tx, invIDs)`|`IN_PROGRESS → CONSUMED`|Receive outputs|
|`MarkShipped(tx, invIDs)`|`AVAILABLE → SHIPPED`|Create delivery|

If a transition is illegal (e.g. consuming something not `IN_PROGRESS`), the inventory function errors and the whole transaction rolls back.

## 3. The Two State Machines

Logistics drives one machine it owns and triggers transitions on one it does not.

**`order_status` (owned by logistics):**

```
PENDING ──assign inputs──▶ PROCESSING ──receive outputs──▶ COMPLETED
```

- `PENDING` — work order created (process + partner chosen); nothing dispatched.
- `PROCESSING` — inputs assigned and sent to the vendor; the vendor holds the goods.
- `COMPLETED` — outputs received; inputs consumed.

Note the schema enum has **no `CANCELLED`** state. Cancellation isn't modelled, and adding it later means an enum change _plus_ compensating inventory rollback logic (returning reserved stock to `AVAILABLE`) — see §10.

**`inventory_status` (owned by `inventory`, transitioned by logistics via §2's functions):**

```
AVAILABLE ──reserve──▶ IN_PROGRESS ──consume──▶ CONSUMED
AVAILABLE ──ship────▶ SHIPPED
```

Logistics never writes these values by hand; it calls the named inventory function, which enforces the transition.

## 4. Work Orders — the Core Loop

Three operations, each a single transaction owned by logistics.

**4.1 Create — `POST /api/work-orders`**

Insert one `work_orders` row (`process_type_id`, `assigned_partner_id`, `status = PENDING`, `created_by`). No inventory effect yet. Trivial, single-table.

**4.2 Assign Inputs — `POST /api/work-orders/{id}/inputs`**

```
tx := BeginTx()
  insert INPUT line items (inventory_id, quantity, uom_id, direction=INPUT)
  inventory.ReserveForWorkOrder(tx, inputInvIDs, woID)   // AVAILABLE → IN_PROGRESS
  advance work order: PENDING → PROCESSING
tx.Commit()
```

Guard: only valid from `PENDING`. The reserved stock is now visibly "with the vendor."

**4.3 Receive Outputs — `POST /api/work-orders/{id}/outputs`** (the hardest path)

FK ordering matters: the new inventory rows must exist **before** the line items that reference them.

```
tx := BeginTx()
  outIDs := inventory.ProduceFromWorkOrder(tx, outputs…)   // INSERT new AVAILABLE rows
  insert OUTPUT line items (inventory_id=outIDs, quantity, uom_id, direction=OUTPUT)
  inventory.ConsumeForWorkOrder(tx, inputInvIDs)           // IN_PROGRESS → CONSUMED
  advance work order: PROCESSING → COMPLETED
tx.Commit()
```

Guards: only valid from `PROCESSING`; rejecting a second receive (status is already `COMPLETED`) is what makes the operation idempotent against double-submit. No yield/conservation check — input and output are in different units (yards in, pieces out), so the system records both sides and does not try to reconcile them (that would need conversions, which don't exist yet).

## 5. Delivery — Outbound

**Create Delivery Note — `POST /api/delivery-notes`**

```
tx := BeginTx()
  generate delivery_note_number (see §10)
  insert delivery_notes row (recipient_partner_id, number, created_by)
  insert delivery_note_items (inventory_id, quantity)
  inventory.MarkShipped(tx, invIDs)                        // AVAILABLE → SHIPPED
tx.Commit()
```

Structurally a lighter cousin of receive-outputs: it consumes finished goods and mutates inventory state, but creates no new inventory and only touches one status transition.

## 6. Lineage — the Backward Trace

This is the product's headline promise — trace a defective shirt back through the delivery note, the tailor, the cutter, to the source roll — and the key insight is that **it is not a feature you build, it is a read over data the write path already recorded.** If §4 lays down the INPUT/OUTPUT line items correctly, lineage is just a graph walk; if it doesn't, no amount of read-side cleverness recovers it. That is the main reason work orders and lineage had to be designed together.

The walk, starting from a shipped/finished inventory item:

```
inventory item
  └─ the OUTPUT work_order_line_item that produced it
       └─ its work_order
            └─ that work order's INPUT line items
                 └─ each input inventory item  ── recurse ──▶ … ▶ RAW
```

This is naturally a recursive CTE over `work_order_line_items` joined on `work_order_id`, following OUTPUT→work order→INPUT at each level until reaching items with no producing work order (the raw rolls).

**Granularity is the work order, not the individual piece.** The schema links all of a work order's inputs and outputs together, not specific output piece N to specific input roll M. For the defect use case this is almost always enough — you learn which order/batch and therefore which roll a defective item came from. True piece-to-piece lineage would require an explicit output-to-input link column, a deliberate schema addition (see §10), not something to retrofit by accident.

`GET /api/inventory/{id}/lineage` returns the assembled tree. (Mounted by logistics even though the path reads as inventory's — the linkage lives here.)

## 7. Process Types

The configurable vocabulary of the engine — the named transformations the factory can run (`Cutting`, `Sewing`, `Printing`). These are **data, not code**: adding `Dyeing` is a row insert, not a new enum value and a redeploy.

It is a flat, two-column lookup (`id`, unique `name`) with plain CRUD, kept in a `process_types.go` file separate from the transactional code in `repository.go`. Its only consumer is `work_orders.process_type_id` — which is why it sits in `logistics` rather than a shared package, unlike `units_of_measure`.

Behaviour mirrors the other lookups: names are unique with surrounding whitespace trimmed, and deletion is blocked while a process is referenced by any work order. `work_orders.process_type_id` carries no `ON DELETE` clause, so the database refuses the delete and the service surfaces it as `409 Conflict` rather than a raw constraint error. A migration seeds the starter set — at minimum `Cutting` and `Sewing`, plus `Printing` for the send-out-for-printing flow — alongside the other seed data.

## 8. API Surface

All routes behind `RequireAuth`; mounted on the logistics router.

|Method|Path|Purpose|
|---|---|---|
|POST|`/api/work-orders`|Create a work order (`PENDING`)|
|POST|`/api/work-orders/{id}/inputs`|Assign + dispatch inputs (`→ PROCESSING`)|
|POST|`/api/work-orders/{id}/outputs`|Receive outputs (`→ COMPLETED`)|
|GET|`/api/work-orders`|List; filter `?status=PROCESSING` powers the WIP dashboard|
|GET|`/api/work-orders/{id}`|One work order with its line items|
|POST|`/api/delivery-notes`|Create a delivery note, mark goods `SHIPPED`|
|GET|`/api/delivery-notes` / `/{id}`|List / detail|
|GET|`/api/inventory/{id}/lineage`|Backward trace tree|
|GET|`/api/process-types`|List processes|
|POST|`/api/process-types`|Create a process|
|PATCH|`/api/process-types/{id}`|Rename a process|
|DELETE|`/api/process-types/{id}`|Delete a process (`409` if in use)|

The WIP view ("what are vendors holding right now?") is just `GET /api/work-orders?status=PROCESSING` — no separate machinery.

## 9. Layering & Files

Standard Handler → Service → Repository, but the repository carries unusual weight here.

- **`handler.go`** — decode/validate JSON, map domain errors to status codes.
- **`service.go`** — orchestration and guards: status gating, sequencing the steps within each operation, beginning/committing the transaction.
- **`repository.go`** — raw SQL and the cross-domain transactions; this is where `BeginTx`, the line-item writes, and the calls into inventory's `DBTX` functions live.
- Group by concern within the package: `work_orders.go`, `delivery.go`, `lineage.go`, `process_types.go`, sharing the transaction helpers.

## 10. Open Decisions

1. **Input disposition on receive — the important one.** Does receiving outputs always fully `CONSUME` the inputs, or can stock partially return? Send 100 yards, use 80 — is there a 20-yard remainder back to `AVAILABLE`, plus scrap? MVP assumes full consumption; remainder/scrap handling is a real post-MVP need and affects the `ConsumeForWorkOrder` contract.
2. **Partial / multiple receipts.** Can one work order receive outputs in several batches, or is receiving a single terminal step (`PROCESSING → COMPLETED`)? MVP treats it as single; partial receipts would need an intermediate status and relax the guard in §4.3.
3. **Cancellation.** No `CANCELLED` in the enum. If a dispatched work order can be aborted, that's an enum change plus compensating logic to return reserved inventory to `AVAILABLE`.
4. **Delivery-note number format.** Plain sequence, or a formatted human key like `DN-2026-0001`? Affects the generation step in §5 and whether it needs its own counter.
5. **Lineage granularity.** Work-order-level (current schema) vs. piece-level (needs an explicit output→input link). Confirm work-order-level is sufficient for the defect-tracing use case before building.
6. **Assign vs. dispatch as one step or two.** §4.2 treats assigning inputs and sending them to the vendor as a single action. If you need an interval where inputs are reserved but not yet shipped, that's a second status and a split.
