-- Setup Extensions & Enums
CREATE TYPE item_type AS ENUM('RAW', 'SEMI_FINISHED', 'FINISHED');
CREATE TYPE order_status AS ENUM('PENDING', 'PROCESSING', 'COMPLETED');
CREATE TYPE inventory_status AS ENUM('AVAILABLE', 'IN_PROGRESS', 'CONSUMED', 'SHIPPED');
CREATE TYPE io_direction AS ENUM('INPUT', 'OUTPUT');

-- 1. Users & Roles (Passwordless: OTP / SSO ready)
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

-- 2. Master Data Tables
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

-- 3. Workflow Engine Tables (Flexible Processing)
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
);

-- 4. Outbound Logistics
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

-- 5. Performance Indexes
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_user_roles_user ON user_roles(user_id);
CREATE INDEX idx_inventory_product ON inventory(product_id);
CREATE INDEX idx_inventory_status ON inventory(status);
CREATE INDEX idx_woli_work_order ON work_order_line_items(work_order_id);
CREATE INDEX idx_dni_delivery_note ON delivery_note_items(delivery_note_id);
