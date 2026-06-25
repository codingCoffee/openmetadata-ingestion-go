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

-- =====================================================================
-- Default database: postgres
-- =====================================================================
\connect postgres

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

INSERT INTO public.customers (id, email, full_name, balance) VALUES
    (1, 'a@example.com', 'Customer A', 100.50),
    (2, 'b@example.com', 'Customer B', 250.00);
INSERT INTO public.orders (id, customer_id, total, status) VALUES
    (1, 1, 50.25, 'PAID'),
    (2, 2, 75.00, 'PENDING');

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
