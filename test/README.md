# Test Postgres fixture

A throwaway Postgres instance for testing `omingest` locally.

## Start

```bash
docker compose up -d            # from the repo root
docker compose ps              # wait until omingest_test_postgres is healthy
```

This loads [`01_seed.sql`](01_seed.sql), which creates:

| Database    | Schema      | Objects                                             |
|-------------|-------------|-----------------------------------------------------|
| `postgres`  | `public`    | `customers`, `orders`, view `active_customers`, `orders_tmp` |
| `sales`     | `public`    | `invoices`                                          |
| `inventory` | `reporting` | `items`                                             |

Connection: `localhost:5432`, user `postgres`, password `postgres`.

## Ingest

`testdata/workflow.postgres.yaml` already points at this instance (it ingests the
`postgres` database, includes the `public` schema, and excludes `*_tmp` tables):

```bash
omingest test-connection -c testdata/workflow.postgres.yaml
omingest ingest          -c testdata/workflow.postgres.yaml
```

To ingest **all three databases** at once, set `ingestAllDatabases: true` in the
config (and drop the `public`-only `schemaFilterPattern` so `inventory`'s
`reporting` schema is included).

## Reset

```bash
docker compose down -v          # wipes the data volume so the seed runs again
docker compose up -d
```

> The seed only runs on first startup (empty data volume). After changing
> `01_seed.sql`, run `docker compose down -v` before `up` again.
