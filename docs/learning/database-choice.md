# Database Choice — Why PostgreSQL

Why PostgreSQL was chosen for this project, and how it compares to the alternatives.

---

## Why PostgreSQL Fits This Domain

The property management domain has specific characteristics that make PostgreSQL a strong fit:

### 1. Financial Data Needs ACID Transactions

Rent collection, idempotency keys, and lease state transitions all require that operations either fully succeed or fully fail — no partial writes. PostgreSQL's transaction model with configurable isolation levels (`SERIALIZABLE`, `REPEATABLE READ`) is built for this.

The project explicitly requires optimistic locking, which maps directly to a Postgres pattern:

```sql
UPDATE leases
SET status = 'active', version = version + 1
WHERE id = $1 AND version = $2
```

If another request updated the row first, `version` won't match and the update affects 0 rows — your application detects the conflict and retries. This is safer than pessimistic locking (which holds DB locks and reduces throughput).

### 2. Relational Data with Complex Joins

The domain is inherently relational:

```
Owner → Properties → Units → Tenants → Leases → RentPayments
```

You'll constantly query across these boundaries — e.g., "all overdue rent for properties owned by X". PostgreSQL handles this with joins, foreign keys, and referential integrity enforced at the DB level (not just application code).

Declaring a foreign key like this:

```sql
CONSTRAINT fk_unit FOREIGN KEY (unit_id) REFERENCES units(id) ON DELETE RESTRICT
```

...means the database itself will reject orphaned records, regardless of which service or script is writing data.

### 3. JSONB for Semi-Structured Data

Maintenance requests, document metadata, and audit logs often have variable schemas. Postgres's `JSONB` column type gives you structured storage + indexing on semi-structured data without needing a separate document store.

```sql
-- Query inside a JSONB field
SELECT * FROM maintenance_requests
WHERE metadata->>'priority' = 'urgent';
```

### 4. AWS RDS

The stack targets AWS ECS + RDS. RDS Postgres is a managed offering with automated backups, read replicas, and Multi-AZ failover — no operational overhead to manage.

### 5. Go Ecosystem Tooling

The Go ecosystem's best database tooling is primarily built around PostgreSQL:

- **`golang-migrate`** — SQL migration management (up/down migrations as versioned files)
- **`pgx`** — high-performance PostgreSQL driver for Go
- **`sqlc`** — generates type-safe Go code from raw SQL queries (perfect for this project's interface-driven design)

---

## Alternatives and Trade-offs

| Database | Type | Why you'd choose it | Why not here |
|---|---|---|---|
| **MySQL / MariaDB** | Relational | Widely known, RDS supported | Weaker transaction isolation defaults, less powerful JSON support, no `RETURNING` clause |
| **CockroachDB** | Distributed SQL (Postgres-compatible) | Horizontal scaling, geo-distribution | Overkill for monolith phase; adds operational complexity early |
| **SQLite** | Embedded relational | Zero setup, great for local dev | No concurrent writes, no network access, not suitable for multi-tenant production |
| **MongoDB** | Document store | Flexible schema, fast for document reads | No ACID across documents (historically), complex joins are painful, poor fit for financial data |
| **DynamoDB** | Key-value / document (AWS) | Infinite scale, serverless pricing | No joins, eventual consistency by default, query patterns must be known upfront |
| **Cassandra / ScyllaDB** | Wide-column | Massive write throughput, multi-region | Eventual consistency, no transactions, overkill for this domain |
| **TimescaleDB** | Postgres extension (time-series) | Excellent for metrics/analytics over time | Specialized — would complement Postgres for reporting, not replace it |

---

## Frontend Analogy

PostgreSQL is like **TypeScript** for your data layer — it enforces contracts (foreign keys, column types, constraints) at the DB level, catching data integrity bugs before they reach your application. MongoDB is more like plain JavaScript — flexible and fast to start, but you pay later in inconsistent or missing data.

---

## When Postgres Might Not Be Enough

Postgres is an OLTP database (optimized for many small, consistent reads/writes). If this project later adds real-time analytics dashboards over millions of rent transactions, you'd introduce a **read replica** or a columnar store (like **Redshift** or **ClickHouse**) for those OLAP queries — not replace Postgres, but complement it.

**The rule of thumb:** use Postgres as your source of truth, and add specialized stores (Redis for caching, ClickHouse for analytics, S3 for files) only when you have a concrete bottleneck.

---

## Key Takeaway

Database choice should be driven by *data shape + consistency requirements*, not popularity.

Property management is a classic OLTP workload — many small, consistent reads/writes with strong relational integrity. That's PostgreSQL's sweet spot.
