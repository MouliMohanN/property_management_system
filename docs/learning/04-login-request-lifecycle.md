# Login Request Lifecycle — Deep Dive

A complete, concept-by-concept trace of `POST /api/v1/auth/login`.
Every security decision is explained with the attack it prevents.
Every architectural decision is explained with the problem it solves.

---

## The Full Journey at a Glance

```
Browser
  │
  │  POST /api/v1/auth/login
  │  Headers: Content-Type: application/json, Origin: http://localhost:5173
  │  Body:    { "email": "admin@pms.dev", "password": "secret123" }
  │
  ▼ TCP handshake (SYN → SYN-ACK → ACK)
  ▼ Go net/http: parse HTTP/1.1, construct *http.Request, spawn goroutine
  │
  ├─► CORS middleware          check Origin header, attach response headers
  ├─► RealIP middleware        rewrite r.RemoteAddr from proxy headers
  ├─► RequestID middleware     stamp a unique ID on this request
  ├─► Recoverer middleware     defer a panic trap
  │
  ▼ chi radix tree matches POST /api/v1/auth/login → AuthHandler.Login
  │
  ▼ AuthHandler.Login          [transport layer]
  │   decode JSON body
  │   validate struct tags
  │   call usecase
  │
  ▼ LoginUseCase.Execute       [application layer — orchestrates the steps below]
  │
  ├─► UserRepository.FindByEmail    SELECT from Postgres via pgxpool
  │     pgx.ErrNoRows → user.ErrInvalidCredentials (not ErrNotFound — intentional)
  │
  ├─► BcryptHasher.Compare          bcrypt.CompareHashAndPassword
  │     mismatch → user.ErrInvalidCredentials
  │
  ├─► user.IsActive()               pure domain logic, zero I/O
  │     inactive → user.ErrAccountInactive
  │
  ├─► JWTService.GenerateAccessToken
  │     crypto/rand JTI + HMAC-SHA256 signed JWT
  │
  ├─► generateOpaqueToken()
  │     32 bytes from crypto/rand → 64-char hex string
  │
  └─► TokenStore.Set (Redis)
        SET refresh:<token> <userID> EX <ttl>
  │
  ▼ AuthHandler.Login
      respondJSON 200 { data: { access_token, refresh_token, user: {id,email,role} } }
  │
  ▼
Browser stores:
  - access_token  → in-memory (module-level variable, lost on page refresh)
  - refresh_token → localStorage (survives page refresh)
```

---

## Phase 0: Before Your Code Runs — TCP and `net/http`

### What actually happens when a browser sends a request

When the FE calls `fetch("http://localhost:8080/api/v1/auth/login", ...)`, the
browser does not magically deliver a function call to your Go handler. There is a
chain of lower-level events first.

**TCP three-way handshake:**
```
Browser                         Go server (port 8080)
  │                                      │
  │──────── SYN ─────────────────────►  │   "I want to connect"
  │  ◄──── SYN-ACK ────────────────────  │   "OK, and I want to connect back"
  │──────── ACK ─────────────────────►  │   "Acknowledged"
  │                                      │
  │  [TCP connection is now open]        │
  │──────── HTTP request bytes ───────►  │
```

Go's standard library owns this. You never write `accept()`, `bind()`, or `recv()`.
`http.ListenAndServe(":8080", router)` does all of it. It runs a loop that calls
`Accept()` — a blocking call that returns when a new TCP connection arrives.

**How Go handles concurrency without threads-per-request:**

Most web servers (old Java, PHP-FPM) create one OS thread per request. OS threads
are expensive: each one gets a 1–8MB stack by default, and context-switching between
them requires a kernel trap. At 10,000 concurrent requests you'd need 10–80 GB of
RAM just for stacks.

Go's model is different. When a new TCP connection arrives, `net/http` spawns a
**goroutine** — not an OS thread. A goroutine starts with a 2–8KB stack that grows
dynamically. Go's runtime multiplexes thousands of goroutines onto a small pool of
OS threads (one per CPU core by default). When a goroutine blocks on I/O (waiting
for Postgres to respond), the runtime parks it and runs another goroutine on that
same OS thread. No kernel trap, no context switch cost.

```
OS Thread 1    [goroutine A: running SQL query... blocked on network]
               [runtime parks A, runs goroutine B immediately]
OS Thread 2    [goroutine C: hashing password... CPU-bound, runs fully]
```

This is why a single Go binary can serve tens of thousands of concurrent requests
with a few hundred MB of RAM — most goroutines are parked waiting on I/O at any
given moment.

---

## Phase 1: The Middleware Chain

### What middleware actually is in Go

The word "middleware" gets used loosely. In Go's `net/http`, it has a precise meaning:
a function that takes an `http.Handler` and returns an `http.Handler`.

```go
type Handler interface {
    ServeHTTP(ResponseWriter, *Request)
}

// A middleware wraps one Handler with another
type Middleware func(http.Handler) http.Handler
```

When chi assembles the chain:
```go
r.Use(middleware.CORS(corsOrigins))
r.Use(chiMiddleware.RealIP)
r.Use(chiMiddleware.RequestID)
r.Use(chiMiddleware.Recoverer)
```

It builds a nested closure structure. If you unrolled it, it would look like:
```go
CORS(
    RealIP(
        RequestID(
            Recoverer(
                authHandler.Login   // the actual handler at the center
            )
        )
    )
)
```

Each wrapper calls the next one via `next.ServeHTTP(w, r)`. Code before that call
runs on the way **in** (request phase). Code after it runs on the way **out**
(response phase). The outermost middleware (CORS) runs first on request and last
on response.

This is assembled **once at startup** — not per-request. The structure is fixed in
memory. Per-request, Go just traverses the call stack.

---

### CORS (`middleware/cors.go`)

**The problem CORS solves:**

Browsers enforce the Same-Origin Policy: JavaScript on `http://evil.com` cannot
make authenticated requests to `http://yourbank.com` using your cookies — the browser
blocks the response. Without this, visiting a malicious website could silently trigger
requests to your banking session.

CORS is a controlled relaxation of Same-Origin Policy. The server declares which
origins it trusts, and the browser enforces that declaration.

**How it works for a simple POST:**
```
Browser sends:
  Origin: http://localhost:5173

Server responds with:
  Access-Control-Allow-Origin: http://localhost:5173

Browser checks: does the response origin match what I sent? Yes → allow JS to read it.
```

**Preflight (OPTIONS) — for non-simple requests:**
Before sending a request with a custom header like `Authorization`, the browser
first sends an OPTIONS request:
```
OPTIONS /api/v1/auth/me HTTP/1.1
Origin: http://localhost:5173
Access-Control-Request-Method: GET
Access-Control-Request-Headers: Authorization
```
The server responds with what it allows:
```
Access-Control-Allow-Origin: http://localhost:5173
Access-Control-Allow-Methods: GET, POST, PUT, ...
Access-Control-Allow-Headers: Authorization, Content-Type
Access-Control-Max-Age: 300
```
`MaxAge: 300` means the browser caches this preflight response for 300 seconds — it
won't send another OPTIONS for the same endpoint for 5 minutes.

**What this middleware does in code** (`middleware/cors.go`):
```go
cors.Handler(cors.Options{
    AllowedOrigins: allowedOrigins,           // ["http://localhost:5173"]
    AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
    AllowedHeaders: []string{"Authorization", "Content-Type"},
    MaxAge:         300,
})
```
It attaches the right headers. For the login request (a POST with Content-Type:
application/json), there is no preflight — `Content-Type: application/json` makes
it a "non-simple" request but only CORS custom headers trigger preflight; the browser
sends the POST directly.

---

### RealIP

When your server sits behind a reverse proxy or load balancer (nginx, AWS ALB), all
requests appear to come from the proxy's IP address — not the actual client.

```
Real client: 203.0.113.42
  ──► nginx (192.168.1.1) ──► Go server
                                r.RemoteAddr = "192.168.1.1:54231" ← useless
```

Proxies add headers to preserve the original IP:
```
X-Real-IP: 203.0.113.42
X-Forwarded-For: 203.0.113.42, 192.168.1.1
```

`RealIP` middleware reads these and rewrites `r.RemoteAddr` to `203.0.113.42`.
Now your rate limiter, audit log, and security monitoring all see the real client.

**Security note:** Only trust these headers when you are actually behind a proxy.
If your server is directly on the internet, an attacker can set `X-Real-IP: 1.2.3.4`
and spoof any IP. The header is only trustworthy when it's set by infrastructure you
control.

---

### RequestID

Every request gets a unique identifier stamped into its context and response header:
```
X-Request-Id: 7f3b9c2a-1d4e-4f5a-8b6c-9d0e1f2a3b4c
```

**Why this matters in practice:**

Without request IDs, debugging looks like:
```
14:32:01 ERROR: connection refused to postgres://...
14:32:01 ERROR: connection refused to postgres://...
14:32:01 ERROR: connection refused to postgres://...
```
Three errors. Which request caused them? Who were the users? Impossible to say.

With request IDs, every log line emitted during a request is tagged with the same ID:
```
14:32:01 INFO  request_id=7f3b9c2a method=POST path=/api/v1/auth/login
14:32:01 ERROR request_id=7f3b9c2a error="connection refused to postgres://..."
14:32:01 INFO  request_id=7f3b9c2a status=500 latency=42ms
```
Now you can filter all logs for `request_id=7f3b9c2a` and see the complete story of
that one request. Users can also send you the `X-Request-Id` from their response
headers and you can find their exact error in seconds.

---

### Recoverer

A deferred panic trap. It uses `defer` — Go's mechanism for scheduling cleanup code
to run when the enclosing function returns (or panics).

```go
func RecovererMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        defer func() {
            if rvr := recover(); rvr != nil {
                // log the panic with stack trace
                w.WriteHeader(http.StatusInternalServerError)
            }
        }()
        next.ServeHTTP(w, r)
    })
}
```

**What happens without it:**
A nil-pointer dereference anywhere in the handler chain causes a panic. In Go,
an unrecovered panic crashes the goroutine. The HTTP connection is dropped —
the client gets a TCP reset, not a proper 500 response. Worse, if the panic happens
while holding a database connection, that connection may not be returned to the pool.

With Recoverer, a panic in any downstream handler is caught, logged with a full stack
trace, and translated into a 500 response. The goroutine is cleaned up properly.

---

## Phase 2: Route Matching — The Radix Tree

chi uses a **radix tree** (also called a Patricia tree or compressed trie) for routing.
To understand why, consider the naive approach first.

**Naive approach: linear scan**
```
routes = [
    ("GET", "/health"),
    ("POST", "/api/v1/auth/login"),
    ("POST", "/api/v1/auth/refresh"),
    ("GET", "/api/v1/auth/me"),
    ...100 more routes
]

for route in routes:
    if method matches and path matches:
        call handler
```
This is O(n) — as routes grow, every request scans more entries.

**Radix tree: O(log n) by path depth**

A radix tree compresses shared prefixes. The paths:
```
/api/v1/auth/login
/api/v1/auth/refresh
/api/v1/auth/me
/api/v1/auth/register
```
Are stored as:
```
/api/v1/auth/
    ├── login     → POST → AuthHandler.Login
    ├── refresh   → POST → AuthHandler.Refresh
    ├── me        → GET  → AuthHandler.Me
    └── register  → POST → AuthHandler.Register
```
chi traverses the tree character by character and stops at the longest matching prefix.
The depth of traversal is bounded by the path length, not the number of routes.

**Route ordering and security:**

In `router.go`, `POST /login` is declared outside any `r.Use(middleware.Authenticate(...))` group:
```go
r.Route("/api/v1/auth", func(r chi.Router) {
    r.Post("/login", authHandler.Login)      // no auth middleware — public
    r.Post("/refresh", authHandler.Refresh)  // no auth middleware — public

    r.Group(func(r chi.Router) {
        r.Use(middleware.Authenticate(tokenSvc))
        r.Post("/logout", authHandler.Logout)
        r.Get("/me", authHandler.Me)
    })
})
```
Login must be public — the user has no token yet. This is not a security hole;
it is an intentional public endpoint. The protection is inside the usecase: it
validates credentials before issuing tokens.

---

## Phase 3: The Handler — Transport Layer (`handler/auth.go:130`)

The handler is the **translation boundary** between HTTP and your application.
Think of it as a customs officer: it checks what came in (decoding, validation),
passes it to the application in the application's language (domain types), and
translates the result back into HTTP (status codes, JSON).

### JSON decoding

```go
var req loginRequest
if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
    respondError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
    return
}
```

`r.Body` is typed as `io.ReadCloser` — a streaming byte reader. `json.NewDecoder`
reads from it incrementally as it parses, rather than loading the entire body into
a `[]byte` first. This matters for large request bodies: a file upload that uses
`ioutil.ReadAll` first allocates the full body in RAM; a streaming decoder does not.

For login, the body is tiny, so it doesn't matter practically — but the pattern is
correct and scales.

**What `Decode` does:**
It uses Go's reflection system to map JSON keys to struct fields by name (or `json`
struct tag). `"email"` maps to `loginRequest.Email` because of the tag
`json:"email"`. Fields without a matching JSON key are left at their zero value.
Fields in the JSON with no matching struct field are silently ignored (unless you
use `json.Decoder.DisallowUnknownFields()`).

If the body is:
- Empty → `EOF` error → 400
- Malformed JSON (missing `}`) → syntax error → 400
- Wrong type (`"email": 123`) → unmarshal type error → 400
- Valid but missing fields → no error from Decode; caught by validation next

### Struct tag validation

```go
type loginRequest struct {
    Email    string `json:"email"    validate:"required,email"`
    Password string `json:"password" validate:"required"`
}

if err := validate.Struct(req); err != nil {
    respondError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
    return
}
```

**Struct tags** are Go's compile-time metadata system. They're key-value pairs
embedded in backtick strings after field declarations. They're invisible at runtime
unless something explicitly reads them via the `reflect` package.

`go-playground/validator` reads the `validate` tag using reflection, parses the
comma-separated rules, and runs each check:

- `required` — field must not be the zero value (`""` for string)
- `email` — field must match RFC 5322 email format

The validator is a `package-level` variable:
```go
var validate = validator.New()
```

Initialized once when the package is loaded, safe for concurrent use by multiple
goroutines. Creating a new validator per-request would be wasteful — `validator.New()`
compiles the reflection cache.

**Why validate at the handler, not the usecase?**

Validation has two kinds:
1. **Format validation** (is this a valid email string?) — transport concern, handler
2. **Business validation** (does this email exist in our system?) — usecase concern

The handler rejects structurally invalid requests before they consume any resources.
The usecase handles business rules. Mixing them would make the usecase aware of HTTP
concepts (like "the client sent a malformed email") which violates the layer boundary.

---

## Phase 4: The UseCase — Orchestration Layer (`usecase/auth/login.go:51`)

### What a usecase is and why it exists

A usecase is a **single application operation** — one unit of work the system can
perform. `LoginUseCase` knows exactly one thing: how to authenticate a user and issue
tokens. It does not know how users are stored, how passwords are hashed, or how tokens
are formatted.

This is enforced by coding against interfaces defined in `ports.go`:

```go
// ports.go — the usecase's contract with the outside world
type PasswordHasher interface {
    Hash(password string) (string, error)
    Compare(hash, password string) error
}

type TokenService interface {
    GenerateAccessToken(userID uuid.UUID, role string) (string, error)
    ValidateAccessToken(token string) (*AccessTokenClaims, error)
}

type TokenStore interface {
    Set(ctx context.Context, token string, userID uuid.UUID, ttl time.Duration) error
    Get(ctx context.Context, token string) (uuid.UUID, error)
    Delete(ctx context.Context, token string) error
}
```

The `LoginUseCase` struct holds these as interface values:
```go
type LoginUseCase struct {
    userRepo        user.Repository  // interface
    hasher          PasswordHasher   // interface
    tokenSvc        TokenService     // interface
    tokenStore      TokenStore       // interface
    refreshTokenTTL time.Duration
}
```

**Concrete implementations are injected at startup in `main.go`:**
```go
// main.go wires everything together
pgPool   := postgres.NewPool(cfg.DSN)
redisClient := redis.NewClient(cfg.RedisAddr)

userRepo    := postgres.NewUserRepository(pgPool)     // implements user.Repository
hasher      := password.NewBcryptHasher()             // implements PasswordHasher
jwtSvc      := token.NewJWTService(cfg.JWTSecret, 15*time.Minute)  // implements TokenService
tokenStore  := redis.NewTokenStore(redisClient)       // implements TokenStore

loginUC := auth.NewLoginUseCase(userRepo, hasher, jwtSvc, tokenStore, 30*24*time.Hour)
```

**The concrete type `*postgres.UserRepository` never appears in the usecase.**
The usecase only holds an `user.Repository` interface value. This is Go's implicit
interface implementation — no `implements` keyword. If `postgres.UserRepository`
has a `FindByEmail` method with the right signature, it satisfies the interface.

**What this buys you:**
- Write a `fakeUserRepository` for tests with predetermined responses — no Docker,
  no Postgres needed for unit tests
- Swap `bcrypt` for `argon2` by writing a new `Argon2Hasher` — usecase untouched
- The usecase can be tested in full isolation from infrastructure

---

### Step 4a: Finding the User — Postgres (`infrastructure/postgres/user_repository.go:68`)

```go
u, err := uc.userRepo.FindByEmail(ctx, input.Email)
if err != nil {
    return nil, user.ErrInvalidCredentials
}
```

**Inside `FindByEmail`:**

```go
const q = `
    SELECT id, email, phone_number, password_hash, first_name, last_name,
           role, status, version, created_at, updated_at
    FROM users
    WHERE email = $1`

row := r.pool.QueryRow(ctx, q, email)
```

#### Parameterized queries and SQL injection

`$1` is a placeholder. The `email` value is sent as a **separate bind parameter**
over the Postgres wire protocol — it is never concatenated into the SQL string.

**What SQL injection looks like without parameterization:**
```go
// WRONG — never do this
query := "SELECT * FROM users WHERE email = '" + email + "'"
```

If `email` is `' OR '1'='1`, the query becomes:
```sql
SELECT * FROM users WHERE email = '' OR '1'='1'
```
This returns every user in the table. An attacker just bypassed authentication.

Worse, with `'; DROP TABLE users; --`, the query becomes:
```sql
SELECT * FROM users WHERE email = ''; DROP TABLE users; --'
```

With parameterized queries, the database treats the entire input as a data value,
never as SQL syntax. Even if the email contains SQL keywords or quotes, they are
harmless. pgx always uses parameterized queries when you pass values separately
from the query string.

#### pgxpool — why connection pooling matters

`r.pool.QueryRow(...)` doesn't open a new TCP connection to Postgres. A
`pgxpool.Pool` manages a pool of already-open, authenticated connections.

**What it costs to open a new connection:**
1. TCP three-way handshake to Postgres: ~1ms on localhost, ~10ms over a network
2. TLS handshake (if SSL is enabled): ~10–50ms
3. Postgres authentication (password exchange): ~5ms
4. Postgres spawns a backend process for this connection: non-trivial

Without pooling, at 1,000 requests/second, you'd open and close 1,000 connections
per second — spending 15–65ms of overhead per request just on connection setup.

With pooling, connections are established at startup and reused. `QueryRow` borrows
one from the pool, runs the query (~1–5ms), and returns it. Connection setup cost
amortizes to near zero.

**Pool sizing matters:** Too few connections → requests queue waiting for one.
Too many → Postgres runs out of connection slots (default max is 100). A typical
production setting is 10–30 connections per server instance.

#### scanUser — mapping rows to domain types

```go
func scanUser(row pgx.Row) (*user.User, error) {
    var u user.User
    var roleStr, statusStr string

    err := row.Scan(
        &u.ID, &u.Email, &u.PhoneNumber, &u.PasswordHash,
        &u.FirstName, &u.LastName, &roleStr, &statusStr,
        &u.Version, &u.CreatedAt, &u.UpdatedAt,
    )

    u.Role   = user.Role(roleStr)
    u.Status = user.Status(statusStr)
    return &u, nil
}
```

`row.Scan` maps Postgres wire types to Go types. Postgres sends data in its own
binary/text encoding; pgx converts it:
- `UUID` → `uuid.UUID` (16-byte value)
- `TEXT` → `string`
- `TIMESTAMPTZ` → `time.Time` (with timezone info)
- `NULL` → `*string` (nil pointer for optional fields like `phone_number`)

`role` and `status` come from Postgres as raw strings (`"admin"`, `"active"`). They
are cast to typed Go constants. This is the infrastructure-to-domain translation: raw
storage types become meaningful domain concepts.

#### User enumeration — why `ErrNotFound` becomes `ErrInvalidCredentials`

```go
u, err := uc.userRepo.FindByEmail(ctx, input.Email)
if err != nil {
    // deliberately not returning ErrNotFound
    return nil, user.ErrInvalidCredentials
}
```

**The attack this prevents:**

Imagine the handler returned 404 when the email doesn't exist and 401 when the
password is wrong. An attacker could write a script:

```python
emails = load_from_breach_database()   # millions of emails from past breaches

for email in emails:
    r = post("/api/v1/auth/login", email=email, password="wrong")
    if r.status == 401:   # 401 = email exists, password wrong
        print(f"Found valid account: {email}")
    if r.status == 404:   # 404 = email doesn't exist
        continue
```

In hours, they'd know which of your users are also in past breaches — a list they
can sell or use for targeted phishing.

By returning the same `ErrInvalidCredentials` (→ 401) for both "no such email" and
"wrong password", the attacker learns nothing. Both cases are indistinguishable.

**The cost:** A legitimate user who types the wrong email gets the same message as
one who typed the right email with the wrong password. This is intentional UX friction
accepted as the security trade-off.

---

### Step 4b: Password Verification — bcrypt (`infrastructure/password/bcrypt.go`)

```go
if err := uc.hasher.Compare(u.PasswordHash, input.Password); err != nil {
    return nil, user.ErrInvalidCredentials
}
```

```go
// BcryptHasher.Compare
func (h *BcryptHasher) Compare(hash, password string) error {
    err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
    if err != nil {
        if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
            return user.ErrInvalidCredentials
        }
        return fmt.Errorf("comparing password hash: %w", err)
    }
    return nil
}
```

#### What a bcrypt hash actually contains

A stored bcrypt hash looks like:
```
$2a$12$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy
```

Breaking it into parts:
```
$2a$          — algorithm version (2a = bcrypt)
12            — cost factor
$             — separator
N9qo8uLOickgx2ZMRZoMye  — 22 chars = 16 bytes of salt, base64-encoded
IjZAgcfl7p92ldGxad68LJZdL17lhWy  — 31 chars = hash output, base64-encoded
```

The **salt** is critical. Even if two users have the same password `"secret123"`,
their hashes will be completely different because the salts are different. Without
salt, an attacker with a precomputed table of common password hashes (a "rainbow
table") could look up any hash instantly. With salt, they'd need a separate table
for every possible salt value — computationally infeasible.

#### Why bcrypt is slow by design

**The cost factor** means bcrypt runs the underlying Blowfish cipher's key setup
2^cost times. At cost=12, that's 2^12 = 4,096 iterations.

Benchmark on a modern CPU (approximate):
```
Cost 10 → ~100ms per hash
Cost 12 → ~250ms per hash
Cost 14 → ~1000ms per hash
```

**Why deliberately slow?**

If passwords are stored as SHA-256 hashes (fast), an attacker who obtains the
hash database can test 10 billion guesses per second on a GPU cluster. The password
`"summer2024"` would be found in milliseconds.

With bcrypt at cost=12 (~250ms per comparison), that same attacker can test only:
```
1 GPU: 1000ms / 250ms = 4 hashes/second
100 GPUs: 400 hashes/second
```

Testing the top 10 million common passwords takes:
```
SHA-256: 10,000,000 / 10,000,000,000 = 0.001 seconds
bcrypt:  10,000,000 / 400            = 6.9 hours
```

The slowness protects your users' passwords even after a database breach. The cost
factor is designed to be increased as hardware improves — the right strategy is to
re-hash passwords with a higher cost on next login.

**The trade-off:** 250ms of CPU per login attempt is real cost on your server.
At 100 logins/second, bcrypt alone consumes 25 CPU-seconds per second — 25 cores
worth of work. This is why rate limiting on the login endpoint is essential in
production (not yet implemented here, worth noting).

#### Timing attacks and constant-time comparison

`bcrypt.CompareHashAndPassword` does not do a naive string comparison. Consider
what happens if we compared byte-by-byte and returned early on the first mismatch:

```go
// WRONG — timing attack vulnerable
func compare(a, b string) bool {
    if len(a) != len(b) { return false }
    for i := range a {
        if a[i] != b[i] { return false }  // returns early!
    }
    return true
}
```

If `a = "secret"` and `b = "sXXXXX"`, this returns after comparing 1 byte.
If `b = "secreX"`, it returns after comparing 5 bytes. The function takes slightly
longer the more characters match.

An attacker measuring response times with nanosecond precision (from the same
datacenter, for example) can exploit this:
1. Send `a[0]` = every possible byte, measure which takes longest → found first byte
2. Repeat for each position

This is a **timing side-channel attack**. `subtle.ConstantTimeCompare` always takes
exactly the same time regardless of where the strings first differ, by XOR-ing all
bytes and accumulating the result — no early exit.

bcrypt's comparison uses this internally. Our wrapper also maps the `ErrMismatchedHashAndPassword`
into a domain error, keeping bcrypt as an implementation detail the usecase never
sees directly.

---

### Step 4c: Account Status Check — Domain Logic (`domain/user/entity.go`)

```go
if !u.IsActive() {
    return nil, user.ErrAccountInactive
}
```

```go
// domain/user/entity.go
func (u *User) IsActive() bool {
    return u.Status == StatusActive
}
```

This is pure domain logic. No database. No imports. No interface. Just a method on
a struct that checks a field.

**Why does this matter architecturally?**

The domain layer is the innermost ring of clean architecture. It must have **zero**
external dependencies. `entity.go` imports nothing from the standard library except
`time`, `strings`, and `github.com/google/uuid` — pure data types, no I/O.

If `IsActive()` were implemented in the usecase:
```go
// In LoginUseCase — WRONG placement
if u.Status != "active" {
    return nil, user.ErrAccountInactive
}
```
It would work, but the business rule would be scattered. Every other usecase that
needs to check user status would duplicate the string comparison. If you change
`StatusActive` from `"active"` to `"enabled"`, you'd need to find every occurrence.
On the entity, it's defined once, in the one place that owns user business rules.

**The `Version` field — optimistic locking (for later):**
```go
type User struct {
    ...
    Version int  // used for optimistic locking — increment on every update
}
```
Not used in login, but worth noting. When updating a user, the SQL is:
```sql
UPDATE users
SET ..., version = version + 1
WHERE id = $1 AND version = $2
RETURNING version
```
If two requests try to update the same user concurrently, the second one finds that
`version` has changed (the first already incremented it) and returns 0 rows. The
repository maps this to `user.ErrConflict`. This prevents lost updates without
database-level locks that block other reads.

---

### Step 4d: Generating the Access Token — JWT (`infrastructure/token/jwt.go:38`)

```go
accessToken, err := uc.tokenSvc.GenerateAccessToken(u.ID, u.Role.String())
```

#### What a JWT is, mechanically

A JWT is three base64url-encoded JSON strings joined by dots:
```
eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiI1NTBlODQwMC1lMjliLTQxZDQtYTcxNi00NDY2NTU0NDAwMDAiLCJpYXQiOjE3MTYxMzQ0MDAsImV4cCI6MTcxNjEzNTMwMCwianRpIjoiYTNmOGIyYzFkNGU1ZjZhNyIsInVpZCI6IjU1MGU4NDAwLWUyOWItNDFkNC1hNzE2LTQ0NjY1NTQ0MDAwMCIsInJvbGUiOiJhZG1pbiJ9.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c
 ─────────────────────────┬─────────────────────  ───────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┬───────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────  ──────────────────────────┬─────────────────────────
                      Header                                                                                                                               Payload                                                                                                                                                                                Signature
```

**Header** (decoded from base64url):
```json
{
  "alg": "HS256",
  "typ": "JWT"
}
```

**Payload** (decoded from base64url):
```json
{
  "sub":  "550e8400-e29b-41d4-a716-446655440000",
  "iat":  1716134400,
  "exp":  1716135300,
  "jti":  "a3f8b2c1d4e5f6a7",
  "uid":  "550e8400-e29b-41d4-a716-446655440000",
  "role": "admin"
}
```

Standard claims:
- `sub` (Subject) — who this token identifies
- `iat` (Issued At) — Unix timestamp when it was created
- `exp` (Expires At) — Unix timestamp when it expires
- `jti` (JWT ID) — unique ID for this specific token

Custom claims (added by this codebase):
- `uid` — user UUID (redundant with `sub`, but explicit)
- `role` — user's role string

**Signature** — this is the important part:
```
HMAC-SHA256(
    secret_key,
    base64url(header) + "." + base64url(payload)
)
```

#### How HMAC-SHA256 works

HMAC (Hash-based Message Authentication Code) combines a secret key with a hash
function to produce a **keyed digest**:

```
HMAC(key, message) = SHA256((key XOR opad) || SHA256((key XOR ipad) || message))
```

Where `ipad` and `opad` are fixed padding constants. The double-hashing with the
key mixed in both times makes it resistant to **length extension attacks** that
affect plain SHA256.

**Why the FE cannot forge a token:**

The FE receives the JWT and can base64-decode both the header and payload in plain
JavaScript — JWTs are **not encrypted**. Anyone can read the payload. But:

1. To produce a valid token with `"role": "admin"`, you need to produce a valid
   signature
2. The signature = `HMAC-SHA256(secret, modified_payload)`
3. The secret is only known to the server
4. HMAC-SHA256 is a one-way function — given the output and the message, you cannot
   recover the key

If an attacker modifies `"role": "tenant"` → `"role": "admin"` in the payload and
sends it, `ValidateAccessToken` will:
```go
token, err := jwt.ParseWithClaims(tokenStr, &jwtClaims{}, func(t *jwt.Token) (interface{}, error) {
    // ...
    return s.secret, nil  // server's secret
})
```
Recompute the signature from the received header + modified payload + secret. It won't
match the signature the attacker left unchanged. `token.Valid` will be `false`.

**The JTI — future-proofing revocation:**
```go
jti, err := randomHex(16)   // 32-char hex, e.g. "a3f8b2c1d4e5f6a7b8c9d0e1f2a3b4c5"
```
Today, JTI is generated and embedded but not used for validation. Its purpose is to
enable **individual token revocation** in the future:

```
Scenario: user changes password, all existing sessions should be invalidated.

Without JTI:
  - You can't revoke individual JWTs. You'd have to change the signing secret,
    invalidating ALL tokens for ALL users. Catastrophic.

With JTI + Redis:
  - On password change, store all the user's current JTIs in a Redis blocklist
  - ValidateAccessToken checks: is this JTI in the blocklist? → reject
  - Existing tokens are revoked without affecting other users
```

The infrastructure is already in place (JTI is in every token); only the blocklist
check needs to be added when the feature is required.

**Why short-lived (15 minutes):**

Access tokens are **stateless** — the server validates them by recomputing the
signature. No database, no Redis, no I/O. This is their performance advantage: a
protected endpoint validates a request in microseconds.

The trade-off: they cannot be revoked before expiry without the blocklist approach
above. If a token is stolen (XSS, intercepted request), the attacker has access for
up to 15 minutes. Keeping the TTL short limits the damage window to something
operationally acceptable.

---

### Step 4e: Generating the Refresh Token — Opaque (`usecase/auth/token.go`)

```go
refreshToken, err := generateOpaqueToken()
```

```go
func generateOpaqueToken() (string, error) {
    b := make([]byte, 32)
    if _, err := rand.Read(b); err != nil {
        return "", fmt.Errorf("reading random bytes: %w", err)
    }
    return hex.EncodeToString(b), nil
}
```

#### What "opaque" means

A JWT is **transparent** — it carries data. Decode the payload and you learn the
user ID, role, and expiry. The server can validate it with no external lookup.

An opaque token is **opaque** — it is just random noise. It carries no data.
The string `"a3f8b2c1d4e5f6a7b8c9d0e1f2a3b4c5..."` tells you nothing by itself.
To use it, the server must look it up in Redis: "what user does this token map to?"

This look-up requirement is the key that makes opaque tokens revocable.

#### `crypto/rand` — why the OS's randomness source

```go
rand.Read(b)   // this is crypto/rand, not math/rand
```

`math/rand` is a **pseudorandom** number generator (PRNG). It produces a sequence of
numbers that look random but are derived from an initial seed by a deterministic
algorithm. Given the seed, every future number is predictable.

```go
// math/rand example — WRONG for security
src := rand.NewSource(time.Now().UnixNano())  // seed = current timestamp
token := rand.New(src).Int63()               // predictable if seed is known
```

If an attacker knows the server's approximate startup time (often visible in HTTP
headers, logs, or TLS certificates), they can narrow the seed space to millions of
values — brute-forceable in seconds.

`crypto/rand` reads from the OS's CSPRNG (Cryptographically Secure Pseudorandom
Number Generator): `/dev/urandom` on Linux, `CryptGenRandom` on Windows. These
are seeded from hardware entropy sources (interrupt timing, CPU noise, hardware
RNG). Their output is **computationally indistinguishable from true randomness** —
no known algorithm can predict future values from observed past values.

32 bytes = 256 bits of entropy. The token space is 2^256 ≈ 10^77. Even testing
a trillion tokens per second, an attacker would need more time than the age of the
universe to find a valid one by brute force.

---

### Step 4f: Storing the Refresh Token in Redis (`infrastructure/redis/token_store.go:29`)

```go
uc.tokenStore.Set(ctx, refreshToken, u.ID, uc.refreshTokenTTL)
```

```go
func (s *TokenStore) Set(ctx context.Context, token string, userID uuid.UUID, ttl time.Duration) error {
    key := refreshKeyPrefix + token   // "refresh:a3f8b2c1..."
    return s.client.Set(ctx, key, userID.String(), ttl).Err()
}
```

The Redis command sent over the network:
```
SET refresh:a3f8b2c1d4e5f6a7b8c9d0e1f2a3b4c5... 550e8400-e29b-41d4-a716-446655440000 EX 2592000
```
- `SET key value` stores a string value at a key
- `EX 2592000` sets a TTL of 2,592,000 seconds (30 days)

#### Why Redis and not Postgres for session storage

**A comparison across every relevant dimension:**

| Concern | Redis | Postgres |
|---|---|---|
| Read latency | 0.1–1ms (in-memory) | 2–15ms (disk + SQL parse) |
| Write latency | 0.1–1ms | 5–20ms |
| TTL management | First-class (`EX` flag, automatic deletion) | Manual `DELETE WHERE expires_at < NOW()` + scheduled job |
| Data model | Simple key→value | Relational (overkill for a token store) |
| Concurrency | Single-threaded event loop, no locks needed | MVCC, lock management |
| Durability | Optional (configurable RDB/AOF snapshots) | Always durable (WAL) |
| Horizontal scale | Easy cluster mode | Complex sharding |

Refresh tokens need **fast lookups** (every 15 minutes per active user) and
**automatic expiry**. Redis is purpose-built for this. Postgres's durability and
relational model are not needed here and carry unnecessary cost.

**The durability trade-off:**

By default, Redis is not durable — a crash without persistence loses all data.
For a session store, "all users get logged out" on a Redis restart is usually
acceptable (they re-login). Enable AOF persistence if you need better guarantees.

**Key namespacing with `refresh:`:**
```go
const refreshKeyPrefix = "refresh:"
```
This is simple but important. All refresh tokens live under `refresh:*`. A future
email verification token would use `verify:*`. Password reset tokens: `pwreset:*`.
They can all coexist in the same Redis instance without colliding, and you can
inspect or flush them by pattern (`KEYS refresh:*`).

---

## Phase 5: Error Propagation — Go's Error Model

### How errors travel through layers

Go errors are **values**, not exceptions. They travel up the call stack explicitly as
return values, not by stack unwinding. Every layer must decide what to do with them.

**The error journey for a wrong password:**

```
bcrypt.CompareHashAndPassword → returns bcrypt.ErrMismatchedHashAndPassword
  ↓
BcryptHasher.Compare → maps to user.ErrInvalidCredentials, returns it
  ↓
LoginUseCase.Execute → receives user.ErrInvalidCredentials, returns it unwrapped
  ↓
AuthHandler.Login → receives it, calls h.mapError(w, err)
  ↓
mapError: errors.Is(err, user.ErrInvalidCredentials) → true → respondError 401
```

**The error journey for a Postgres failure:**

```
pgx → returns some network/io error
  ↓
UserRepository.FindByEmail → wraps: fmt.Errorf("finding user by email: %w", pgxErr)
  ↓
LoginUseCase.Execute → receives wrapped error
                      FindByEmail returned an error → returns user.ErrInvalidCredentials
                      (intentionally hides the cause from the handler)
  ↓
AuthHandler.Login → receives user.ErrInvalidCredentials → 401 INVALID_CREDENTIALS
```

Wait — should a Postgres failure return 401? No! This is a genuine design question.
In this codebase, the usecase maps ALL `FindByEmail` errors to `ErrInvalidCredentials`
to prevent enumeration. A better production approach:

```go
u, err := uc.userRepo.FindByEmail(ctx, input.Email)
if err != nil {
    if errors.Is(err, user.ErrNotFound) {
        return nil, user.ErrInvalidCredentials  // expected case — wrong email
    }
    return nil, fmt.Errorf("finding user: %w", err)  // unexpected — Postgres down
}
```

This way:
- Email not found → `ErrInvalidCredentials` → 401 (prevents enumeration)
- Postgres down → wrapped infrastructure error → falls through `mapError`'s default → 500

### `fmt.Errorf("%w", err)` — error wrapping

```go
return nil, fmt.Errorf("storing refresh token: %w", err)
```

The `%w` verb wraps the original error. The returned error's string representation is:
```
"storing refresh token: dial tcp 127.0.0.1:6379: connect: connection refused"
```

You can still check the underlying cause:
```go
errors.Is(err, ErrTokenNotFound)   // traverses the wrapping chain
errors.As(err, &redisErr)          // extracts a specific error type from the chain
```

This is how infrastructure errors carry context (which operation failed, with what
input) while still being identifiable by their root cause. It's Go's alternative to
exception stack traces — explicit, composable, and zero-allocation.

### The `default` case — internal error hiding

```go
default:
    h.log.Error().Err(err).Msg("unhandled error in auth handler")
    respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "an unexpected error occurred")
```

The **real** error is logged internally (with zerolog, structured, queryable). The
**client** receives only a generic message. Why?

Infrastructure errors frequently expose sensitive information:
```
"pq: password authentication failed for user 'admin' on host 'prod-db.internal'"
"dial tcp 10.0.1.45:5432: i/o timeout"
"redis: WRONGPASS invalid username-password pair or user is disabled"
```

These reveal internal hostnames, ports, database usernames, and infrastructure
topology. An attacker who triggers errors deliberately (with crafted inputs) can
map your infrastructure from error messages alone. The rule: log everything
internally, expose nothing externally.

---

## Phase 6: `context.Context` — Cancellation Propagation

Every I/O call in the login flow receives `ctx`:
```go
uc.userRepo.FindByEmail(ctx, input.Email)
uc.tokenStore.Set(ctx, refreshToken, u.ID, uc.refreshTokenTTL)
```

`ctx` originates from `r.Context()` in the handler — it is tied to the HTTP
request's lifetime.

**What happens when the client disconnects:**

The user closes the browser tab mid-login. Go's `net/http` detects the TCP connection
was closed and **cancels** `r.Context()`. This cancellation propagates automatically
to any code holding that context.

pgx checks the context before and during query execution:
```
ctx cancelled → pgx sends a cancel to Postgres → Postgres aborts the query → pgx
returns ctx.Err() (context.Canceled) immediately
```

The goroutine exits cleanly. The database connection is returned to the pool. No
resources are held for a dead request.

Without context propagation, the goroutine would complete the full login flow — running
bcrypt (~250ms), querying Postgres, writing to Redis — for a client that has already
left. Under heavy load, this wastes significant CPU and I/O.

---

## The Two-Token Design — Complete Security Model

### Why two tokens at all?

The fundamental tension in session management:

**Requirement 1:** Sessions must be fast to validate (you validate on every API call)
**Requirement 2:** Sessions must be revocable (logout, password change, account suspension)

These two requirements conflict:
- **Pure stateful sessions** (session ID stored in DB/Redis) — revocable, but every
  request hits the database
- **Pure stateless tokens** (JWT) — fast (no DB), but irrevocable until expiry

The two-token system is a compromise:
- Access token (JWT, short-lived) → fast, used on every request, accepts the
  15-minute irrevocability window
- Refresh token (opaque, long-lived, in Redis) → slow (one Redis lookup per refresh),
  but fully revocable, used only every 15 minutes

### The complete lifecycle

```
Login
  └─► Server issues access_token (15min JWT) + refresh_token (30-day opaque in Redis)

API call (e.g. GET /api/v1/auth/me)
  └─► FE sends: Authorization: Bearer <access_token>
  └─► Authenticate middleware: ValidateAccessToken → checks signature + expiry
  └─► No Redis, no Postgres — pure in-memory validation (~50 microseconds)

access_token expires (15min later)
  └─► FE sends refresh_token to POST /api/v1/auth/refresh
  └─► Server: Redis GET refresh:<token> → gets userID
  └─► Server: Redis DEL refresh:<token>  ← ROTATION: old token deleted
  └─► Server: generates new access_token + new refresh_token
  └─► Server: Redis SET refresh:<new_token> → userID
  └─► FE stores new tokens, resumes API calls

Logout
  └─► FE sends refresh_token to POST /api/v1/auth/logout
  └─► Server: Redis DEL refresh:<token>
  └─► The access_token is still technically valid for up to 15min,
      but without the refresh_token, the attacker can't get new ones.
      15-minute window accepted as trade-off.
```

### Token rotation — detecting theft

When a refresh token is used, the old one is **deleted** and a new one is issued.
This is called **refresh token rotation**. Its security property:

```
Scenario: attacker steals the refresh_token (e.g. from localStorage via XSS)

Without rotation:
  Attacker can refresh tokens indefinitely. Legitimate user never knows.
  Only fix: user manually logs out.

With rotation:
  Attacker uses the stolen refresh_token → gets a new one (old one deleted from Redis)
  Later, legitimate user tries to refresh → token is gone → Redis returns ErrTokenNotFound
  User is forced to re-login.
  
  The anomaly (user being unexpectedly logged out) signals that their token was stolen.
  The server could also flag the account for security review.
```

This is why the FE (`AuthContext.tsx`) handles `ErrTokenNotFound` on refresh by
clearing tokens and redirecting to login — it's not just an error, it's a signal.

---

## Clean Architecture — The Dependency Rule in Full

Every import in this codebase follows one direction: inward.

```
┌─────────────────────────────────────────────────────────┐
│  transport/http/handler                                  │
│    imports: usecase/auth, domain/user (for error types) │
│    ← knows nothing about Postgres, Redis, bcrypt        │
├─────────────────────────────────────────────────────────┤
│  usecase/auth                                            │
│    imports: domain/user, usecase/auth/ports.go           │
│    ← knows nothing about net/http, Postgres, Redis      │
├─────────────────────────────────────────────────────────┤
│  domain/user                                             │
│    imports: time, uuid — pure data types only           │
│    ← zero infrastructure dependencies                   │
└─────────────────────────────────────────────────────────┘
           ▲           ▲           ▲           ▲
  infrastructure/   infrastructure/  infrastructure/  infrastructure/
     postgres          redis           token          password
  (implements        (implements     (implements     (implements
  user.Repository)   TokenStore)     TokenService)   PasswordHasher)
```

Infrastructure packages import domain and usecase packages (to implement the
interfaces). They do not import each other. The usecase and domain packages never
import infrastructure.

**What this buys you in practice:**

*Testing:* You can test `LoginUseCase` without any running services:
```go
func TestLoginUseCase_WrongPassword(t *testing.T) {
    repo := &fakeUserRepo{user: &user.User{
        Email:        "admin@pms.dev",
        PasswordHash: "$2a$12$...",
        Status:       user.StatusActive,
    }}
    hasher   := &fakeHasher{shouldFail: true}
    tokenSvc := &fakeTokenSvc{}
    store    := &fakeTokenStore{}

    uc := auth.NewLoginUseCase(repo, hasher, tokenSvc, store, 30*24*time.Hour)
    _, err := uc.Execute(ctx, auth.LoginInput{Email: "admin@pms.dev", Password: "wrong"})

    assert.ErrorIs(t, err, user.ErrInvalidCredentials)
}
```
No Docker. No real Postgres. No network. The test runs in microseconds.

*Swapping implementations:* To switch from bcrypt to argon2id (a newer, memory-hard
algorithm), you write:
```go
type Argon2Hasher struct{ ... }
func (h *Argon2Hasher) Hash(password string) (string, error) { ... }
func (h *Argon2Hasher) Compare(hash, password string) error  { ... }
```
And change one line in `main.go`. The usecase, handler, domain — untouched.

*Multiple transports:* Add a gRPC handler next to the HTTP handler — both call the
same `LoginUseCase`. The business logic is written once.

---

## Questions Worth Sitting With

These are not rhetorical — they represent real production decisions you'll face:

**1. What happens if Redis goes down during login?**
`TokenStore.Set` returns an error. `LoginUseCase` wraps it and returns it.
`mapError`'s default case catches it and returns 500. Users cannot log in while
Redis is down. How would you design for Redis unavailability — graceful degradation
vs. hard failure?

**2. When should you add JTI-based access token revocation?**
Every request to a protected endpoint would need a Redis lookup (`GET blocklist:<jti>`).
You've traded the stateless advantage of JWT for revocability. At what point is this
trade-off worth making? Password changes? Account suspension? Security incidents?

**3. The login endpoint has no rate limiting. What does that expose?**
An attacker can call `POST /api/v1/auth/login` with one email and millions of password
guesses. bcrypt's slowness helps (250ms per attempt = ~4 guesses/sec per connection),
but with many parallel connections they can do better. What mechanisms would you add?
(Token bucket per IP, CAPTCHA after N failures, account lockout after M failures — each
with its own trade-offs.)

**4. The login response only returns `{id, email, role}`. Should it return the full user?**
The current design requires two round trips after login (login + getMe). Enriching the
login response to return the full `User` would save one round trip. What is the
argument for keeping them separate? When does the extra round trip become a real problem?

**5. `refreshTokenTTL` is a `time.Duration` injected via `main.go`. How would you make it per-role?**
An admin account might need a shorter refresh TTL (higher security) than a regular
tenant. Where in the architecture would you introduce per-role TTL logic without
violating layer boundaries?
