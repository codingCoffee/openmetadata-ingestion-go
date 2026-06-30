-- Seed data for testing omingest against Postgres.
--
-- Runs once on first container startup (mounted into /docker-entrypoint-initdb.d).
-- Creates three databases so you can test both single-database ingestion and the
-- ingestAllDatabases option:
--
--   postgres   (default)  -> public schema with customers, orders, a view, orders_tmp
--   sales                 -> public schema with invoices
--   inventory             -> reporting schema with items
--
-- Exercises: multiple DBs/schemas, tables + a view, varied column types,
-- table/column comments, primary keys, and a "_tmp" table for filter testing.
--
-- Also creates a least-privilege, read-only login role "omingest_ro" (see the
-- bottom of each database section) so you can run the binary as a user that can
-- only read metadata + SELECT, never write. Local test credentials only.

-- =====================================================================
-- Default database: postgres
-- =====================================================================
\connect postgres

-- Read-only role used to test the binary with limited privileges. Cluster-wide,
-- so created once here; CONNECT / USAGE / SELECT grants are applied per database
-- below, after each one's tables exist. Password is for the local test DB only.
CREATE ROLE omingest_ro WITH LOGIN PASSWORD 'omingest_ro';

CREATE TABLE public.customers (
    id          bigint PRIMARY KEY,
    email       varchar(255) NOT NULL,
    full_name   text,
    is_active   boolean DEFAULT true,
    balance     numeric(18, 2),
    created_at  timestamp without time zone DEFAULT now(),
    metadata    jsonb
);
COMMENT ON TABLE  public.customers         IS 'Customer master records';
COMMENT ON COLUMN public.customers.id      IS 'Surrogate primary key';
COMMENT ON COLUMN public.customers.email   IS 'Unique login email';
COMMENT ON COLUMN public.customers.balance IS 'Account balance in INR';

CREATE TABLE public.orders (
    id           bigint PRIMARY KEY,
    customer_id  bigint NOT NULL REFERENCES public.customers (id),
    total        numeric(12, 2) NOT NULL,
    status       varchar(32),
    placed_at    timestamp with time zone DEFAULT now()
);
COMMENT ON TABLE public.orders IS 'Customer orders';

CREATE VIEW public.active_customers AS
    SELECT id, email, full_name
    FROM public.customers
    WHERE is_active = true;
COMMENT ON VIEW public.active_customers IS 'Customers whose account is active';

-- Matches the example tableFilterPattern exclude ("_tmp$") -> should be skipped.
CREATE TABLE public.orders_tmp (
    id bigint PRIMARY KEY
);

INSERT INTO public.customers (id, email, full_name, is_active, balance, metadata) VALUES
    (1, 'a@example.com', 'Customer A', true,  100.50, '{"tier": "gold"}'),
    (2, 'b@example.com', 'Customer B', true,  250.00, '{"tier": "silver"}'),
    (3, 'c@example.com', 'Customer C', false,   0.00, '{"tier": "bronze"}');
INSERT INTO public.orders (id, customer_id, total, status) VALUES
    (1, 1,  50.25, 'PAID'),
    (2, 2,  75.00, 'PENDING'),
    (3, 1, 120.00, 'PAID'),
    (4, 3,  19.99, 'CANCELLED');

-- Read-only grants for this database (omingest_ro can read metadata + SELECT).
GRANT CONNECT ON DATABASE postgres TO omingest_ro;
GRANT USAGE ON SCHEMA public TO omingest_ro;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO omingest_ro;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT ON TABLES TO omingest_ro;

-- =====================================================================
-- Second database: sales
-- =====================================================================
CREATE DATABASE sales;
\connect sales

CREATE TABLE public.invoices (
    id          bigint PRIMARY KEY,
    amount      numeric(14, 2) NOT NULL,
    currency    char(3) DEFAULT 'INR',
    issued_on   date
);
COMMENT ON TABLE public.invoices IS 'Issued invoices';

INSERT INTO public.invoices (id, amount, currency, issued_on) VALUES
    (1, 1500.00, 'INR', DATE '2026-01-15'),
    (2,  990.50, 'INR', DATE '2026-02-01'),
    (3,  250.00, 'USD', DATE '2026-02-20');

-- Read-only grants for this database.
GRANT CONNECT ON DATABASE sales TO omingest_ro;
GRANT USAGE ON SCHEMA public TO omingest_ro;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO omingest_ro;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT ON TABLES TO omingest_ro;

-- =====================================================================
-- Third database: inventory (with a non-public schema)
-- =====================================================================
CREATE DATABASE inventory;
\connect inventory

CREATE SCHEMA reporting;

CREATE TABLE reporting.items (
    sku         varchar(64) PRIMARY KEY,
    name        text NOT NULL,
    qty_on_hand integer DEFAULT 0,
    price       numeric(10, 2)
);
COMMENT ON TABLE reporting.items IS 'Inventory items with stock levels';

INSERT INTO reporting.items (sku, name, qty_on_hand, price) VALUES
    ('SKU-001', 'Widget', 120,  9.99),
    ('SKU-002', 'Gadget',  45, 24.50),
    ('SKU-003', 'Gizmo',    0,  4.75);

-- Read-only grants for this database (note: items lives in the reporting schema).
GRANT CONNECT ON DATABASE inventory TO omingest_ro;
GRANT USAGE ON SCHEMA reporting TO omingest_ro;
GRANT SELECT ON ALL TABLES IN SCHEMA reporting TO omingest_ro;
ALTER DEFAULT PRIVILEGES IN SCHEMA reporting GRANT SELECT ON TABLES TO omingest_ro;
