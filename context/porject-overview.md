# Project Overview

### Core Idea

In textile manufacturing, raw materials don't just sit on a shelf, they transform. A roll of fabric is cut into pieces, sent out to third-party vendors, sewn into garments, and sometimes sent out again for printing. Traditional inventory systems fail here because they only track "what you buy" and "what you sell." They lose track of the messy middle, leading to lost inventory ("Where did those 50 yards go?"), lack of accountability with vendors, and an inability to trace defects back to the source roll.

**The Solution:** **Traceable Manufacturing & Logistics Engine**.

Instead of tracking static items, track the _lifecycle_ of materials. The core idea is to provide absolute visibility into:

- **Location & Custody:** Who currently has the material (Warehouse, Cutter, or Tailor)?
- **Transformation:** What did this material turn into (Yards of fabric $\rightarrow$ Pieces of cut cloth $\rightarrow$ Finished T-shirts)?
- **Traceability (Lineage):** If a customer receives a defective t-shirt, the owner can trace it backward through the delivery note, the tailor, the cutter, and exactly which roll of fabric it originated from.

### Users

The MVP is optimized for a **single-user operational flow**, but the system is built to expand.

**For the MVP:**

- **The Factory Owner / Production Manager (You / Your Client):** They will act as the "omnipresent" user. They are using the app as a central command center to manually log when fabric arrives, when they dispatch it to a vendor, when the vendor returns the finished work, and when goods are shipped to customers.

**Post-MVP (Why we structured the database the way we did):**

- **Warehouse Operators:** Scanning QR codes or entering data when physical goods move in and out of the factory doors.

### 3. Main Features for MVP

To get this off the ground quickly, the MVP is sticked to the critical path of data flow.

**1. Authentication**

- A simple passwordless login screen using WhatsApp OTP (a 6-digit code sent directly to the user's WhatsApp number) or SSO.
- Session management via JWT (JSON Web Tokens) or secure cookies in your Go backend.

**2. Master Data Management**

- **Products:** Add/Edit blueprints (e.g., "Navy Cotton Roll", "Cut T-Shirt Panel", "Finished T-Shirt").
- **Partners:** Add/Edit vendors, cutters, tailors, and clients.
- **Processes & UOM:** Manage the units of measure and the types of processes (Cutting, Sewing).

**3. Inbound Logistics (Receiving)**

- A feature to "Receive Goods."
- _Backend Action:_ Creates a new row in the `inventory` table with status `AVAILABLE` (e.g., logging a 100-yard roll from a supplier).

**4. The Workflow Engine (The Core Loop)**

- **Create Work Order:** Select a Process (e.g., Cutting), select a Partner (e.g., Cutter Vendor A).
- **Assign Inputs:** Select active inventory (e.g., Fabric Roll #101) to "send" to the vendor. This marks the input inventory as `CONSUMED` or `IN_PROGRESS`.
- **Receive Outputs:** When the vendor is done, log the resulting items (e.g., 200 Cut Pieces). This creates _new_ rows in the `inventory` table linked to that specific Work Order.

**5. Outbound Logistics (Shipping)**

- **Create Delivery Note:** Select a customer (Partner), select finished goods from `inventory`, and generate a delivery note number.
- _Backend Action:_ Changes the inventory status of those items to `SHIPPED`.

**6. The "Where is my stuff?" Dashboard**

- A real-time summary view.
- **Current Stock:** Querying the `inventory` table for anything marked `AVAILABLE`.
- **Work in Progress (WIP):** Querying the `work_orders` table for anything marked `PROCESSING` to see exactly what vendors are currently holding.

### Tech Stack

**Backend & Architecture**

- **Programming Language:** Go (Golang). Excellent for high-performance concurrent operations and handling the transactional logic required for inventory state changes.
- **Database:** PostgreSQL. Provides robust relational data integrity and rock-solid transaction handling (ACID compliance) necessary for the workflow engine.
- **Architectural Pattern:** Idiomatic Go Project Layout (`cmd/` and `internal/`) combined with Domain-Driven Package Organization. The logic will follow a pragmatic Handler $\rightarrow$ Service $\rightarrow$ Store layering to separate HTTP routing from logistics business rules and raw database queries, ensuring high maintainability without unnecessary abstraction overhead.

**Infrastructure & Tooling**

- **Routing:** `go-chi/chi`. A lightweight, idiomatic router for building Go HTTP services. Selected for its powerful middleware ecosystem and compatibility with standard `net/http` handlers.
- **Database Interface:** Raw SQL via standard library `database/sql` combined with the `github.com/lib/pq` driver. Migrations and schema changes are managed manually via SQL scripts to ensure absolute control over transactions and logic.
- **Configuration:** Environment variables are strictly managed via `joho/godotenv` and loaded into a central `config.Env` struct on startup to ensure type safety and fail-fast validation.
- **Containerization:** Docker. Containerizing both the Go backend and the PostgreSQL database ensures a consistent environment from local development to production.
- **Authentication:** Native Go logic for generating and validating OTPs. OTP delivery is handled via HTTP requests to a WhatsApp Business API provider. Session state is managed via HTTP-only secure cookies or JWTs.

### File Structure

```text
.
├── cmd/
│   └── server/
│       └── main.go              # The entry point: loads config, connects DB, starts HTTP server
├── internal/
│   ├── auth/                    # Domain: Users, OTP, Sessions
│   │   ├── handler.go           # HTTP routes (e.g., POST /login)
│   │   ├── service.go           # Business logic (e.g., generate OTP, validate)
│   │   └── repository.go             # Database queries for users and roles
│   ├── inventory/               # Domain: Products (blueprints) and Physical Stock
│   │   ├── handler.go
│   │   ├── service.go
│   │   └── repository.go
│   ├── logistics/               # Domain: The Workflow Engine (Work Orders & Delivery)
│   │   ├── handler.go
│   │   ├── service.go           # Logic: deducts input inventory, adds output inventory
│   │   └── store.go             # Complex SQL transactions live here
│   ├── partners/                # Domain: Vendors, Cutters, Tailors, Customers
│   │   ├── handler.go
│   │   ├── service.go
│   │   └── repository.go
│   └── config/                  # Environment variables loading
│       └── config.go
├── go.mod
└── go.sum

```

### Database Schema

```sql
--Setup Extensions & Enums
CREATE TYPE item_type AS ENUM('RAW', 'SEMI_FINISHED', 'FINISHED');
CREATE TYPE order_status AS ENUM('PENDING', 'PROCESSING', 'COMPLETED');
CREATE TYPE inventory_status AS ENUM('AVAILABLE', 'IN_PROGRESS', 'CONSUMED', 'SHIPPED');
CREATE TYPE io_direction AS ENUM('INPUT', 'OUTPUT');

--1. Users & Roles(Passwordless: OTP / SSO ready)
CREATE TABLE roles(
    id SERIAL PRIMARY KEY,
    name VARCHAR(50) NOT NULL UNIQUE,
    description TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE users(
    id SERIAL PRIMARY KEY,
    email VARCHAR(255) NOT NULL UNIQUE,
    phone_number VARCHAR(20) NOT NULL UNIQUE,
    full_name VARCHAR(255),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    last_login_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE user_roles(
    user_id INT REFERENCES users(id) ON DELETE CASCADE,
    role_id INT REFERENCES roles(id) ON DELETE CASCADE,
    PRIMARY KEY(user_id, role_id)
);

--2. Master Data Tables
CREATE TABLE units_of_measure(
    id SERIAL PRIMARY KEY,
    name VARCHAR(50) NOT NULL UNIQUE
);

CREATE TABLE products(
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    type item_type NOT NULL,
    default_uom_id INT REFERENCES units_of_measure(id),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE partners(
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    email VARCHAR(255),
    phone VARCHAR(50),
    address TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE partner_roles(
    partner_id INT REFERENCES partners(id) ON DELETE CASCADE,
    role_name VARCHAR(100) NOT NULL,
    PRIMARY KEY(partner_id, role_name)
);

CREATE TABLE inventory(
    id SERIAL PRIMARY KEY,
    product_id INT REFERENCES products(id) ON DELETE RESTRICT,
    batch_number VARCHAR(100),
    current_quantity DECIMAL(12, 4) NOT NULL DEFAULT 0.0000,
    uom_id INT REFERENCES units_of_measure(id),
    status inventory_status NOT NULL DEFAULT 'AVAILABLE',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

--3. Workflow Engine Tables(Flexible Processing)
CREATE TABLE process_types(
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL UNIQUE
);

CREATE TABLE work_orders(
    id SERIAL PRIMARY KEY,
    process_type_id INT REFERENCES process_types(id),
    assigned_partner_id INT REFERENCES partners(id),
    status order_status NOT NULL DEFAULT 'PENDING',
    created_by INT REFERENCES users(id) ON DELETE SET NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE work_order_line_items(
    id SERIAL PRIMARY KEY,
    work_order_id INT REFERENCES work_orders(id) ON DELETE CASCADE,
    inventory_id INT REFERENCES inventory(id),
    quantity DECIMAL(12, 4) NOT NULL,
    direction io_direction NOT NULL
    consumed, 'OUTPUT'
    produced
);

--4. Outbound Logistics
CREATE TABLE delivery_notes(
    id SERIAL PRIMARY KEY,
    delivery_note_number VARCHAR(100) NOT NULL UNIQUE,
    recipient_partner_id INT REFERENCES partners(id),
    created_by INT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE delivery_note_items(
    id SERIAL PRIMARY KEY,
    delivery_note_id INT REFERENCES delivery_notes(id) ON DELETE CASCADE,
    inventory_id INT REFERENCES inventory(id),
    quantity DECIMAL(12, 4) NOT NULL
);

--5. Performance Indexes
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_user_roles_user ON user_roles(user_id);
CREATE INDEX idx_inventory_product ON inventory(product_id);
CREATE INDEX idx_inventory_status ON inventory(status);
CREATE INDEX idx_woli_work_order ON work_order_line_items(work_order_id);
CREATE INDEX idx_dni_delivery_note ON delivery_note_items(delivery_note_id);
```
