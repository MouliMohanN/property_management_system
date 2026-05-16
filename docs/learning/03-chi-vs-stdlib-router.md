# HTTP Routing in Go: stdlib vs chi

## Why this matters

In a production API you need more than just "route this URL to this function." You need:

- **Middleware chains** — auth, logging, rate limiting, RBAC applied in layers
- **Route groups** — apply middleware to a set of routes without repeating yourself
- **Nested resources** — `/properties/{id}/units/{unitId}` cleanly
- **Path parameters** — extract `{id}` without string splitting

Go 1.22 added method routing and path params to the standard library. It's capable, but the ergonomics break down quickly in a real API. `chi` fills exactly those gaps.

---

## stdlib net/http (Go 1.22+)

### What it can do now

```go
mux := http.NewServeMux()

// Method + path routing (Go 1.22+)
mux.HandleFunc("GET /properties/{id}", getPropertyHandler)
mux.HandleFunc("POST /properties", createPropertyHandler)

// Read a path parameter
func getPropertyHandler(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id") // e.g. "abc-123"
}
```

### Where it falls apart: middleware

In a real API every protected route needs something like:

```
request → rate limiter → auth (JWT) → RBAC → request logger → your handler
```

With stdlib you compose middleware by hand-wrapping each handler:

```go
// Option 1: wrap per-handler — repetitive, error-prone
mux.HandleFunc("GET /properties/{id}", rateLimiter(auth(rbac(logger(getPropertyHandler)))))
mux.HandleFunc("GET /properties",      rateLimiter(auth(rbac(logger(listPropertiesHandler)))))
mux.HandleFunc("GET /leases",          rateLimiter(auth(rbac(logger(listLeasesHandler)))))
// ... repeat for every route

// Option 2: write your own chain helper
type Middleware func(http.Handler) http.Handler

func chain(h http.Handler, middlewares ...Middleware) http.Handler {
    for i := len(middlewares) - 1; i >= 0; i-- {
        h = middlewares[i](h)
    }
    return h
}

mux.Handle("GET /properties/{id}", chain(
    http.HandlerFunc(getPropertyHandler),
    rateLimiter, auth, rbac, logger,
))
```

You end up building a mini-framework yourself. That's exactly what chi already is.

### Route groups don't exist

There's no built-in way to say "all routes under `/api` require auth." You manage this manually per-handler or by wrapping sub-muxes.

---

## chi router

chi is a lightweight (~1500 lines), dependency-free router built on top of `net/http`. Every chi handler is a standard `http.Handler` — no lock-in, no magic types.

### Same example, chi style

```go
r := chi.NewRouter()

// Global middleware — runs on every single request
r.Use(middleware.RealIP)       // resolve client IP behind proxies
r.Use(middleware.RequestID)    // inject a unique ID per request (useful for logging)
r.Use(loggingMiddleware)       // structured request logs

// Public routes — no auth needed
r.Get("/health", healthHandler)
r.Post("/auth/login", loginHandler)
r.Post("/auth/refresh", refreshTokenHandler)

// Protected routes — auth applied to the whole group
r.Group(func(r chi.Router) {
    r.Use(authMiddleware)  // validates JWT, injects user into context
    r.Use(rbacMiddleware)  // checks user role against required permissions

    // Property routes
    r.Route("/properties", func(r chi.Router) {
        r.Get("/", listPropertiesHandler)
        r.Post("/", createPropertyHandler)

        r.Route("/{propertyID}", func(r chi.Router) {
            r.Get("/", getPropertyHandler)
            r.Put("/", updatePropertyHandler)
            r.Delete("/", deletePropertyHandler)

            // Nested resource — units belong to a property
            r.Route("/units", func(r chi.Router) {
                r.Get("/", listUnitsHandler)
                r.Post("/", createUnitHandler)
            })
        })
    })
})
```

`authMiddleware` runs once — automatically applied to every route inside that group. Add a new route inside the group and it gets auth for free. This is the production pattern.

### Frontend analogy

| Frontend concept | chi equivalent |
|---|---|
| Next.js `layout.tsx` wrapping child routes | `r.Group` with `r.Use` middleware |
| Express.js `router.use('/api', apiRouter)` | `r.Route("/api", func(r chi.Router) {...})` |
| Axios interceptors | global `r.Use(...)` middleware |
| Route params `:id` in Express | `{id}` in chi, read with `chi.URLParam(r, "id")` |

### Reading path parameters in chi

```go
func getPropertyHandler(w http.ResponseWriter, r *http.Request) {
    propertyID := chi.URLParam(r, "propertyID")
    // vs stdlib: r.PathValue("propertyID")
    // Both work. chi's version is the same under the hood.
}
```

### Passing data between middleware and handlers

chi uses Go's `context.Context` to pass values down the chain — the same pattern used everywhere in Go for request-scoped data:

```go
// In authMiddleware: decode JWT, inject the user
func authMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        user := validateJWT(r.Header.Get("Authorization"))
        ctx := context.WithValue(r.Context(), userCtxKey, user)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

// In any handler downstream: retrieve the user
func getPropertyHandler(w http.ResponseWriter, r *http.Request) {
    user := r.Context().Value(userCtxKey).(*domain.User)
    // now we have the authenticated user without touching the DB again
}
```

This is how RBAC works in practice — auth middleware validates the token and writes the user into context; RBAC middleware reads the user from context and checks their role.

---

## Decision

| Factor | stdlib | chi |
|---|---|---|
| Middleware chaining | Manual, verbose | Built-in `r.Use()` |
| Route groups | None | First-class `r.Group()` |
| Nested resources | Manual sub-muxes | `r.Route()` nesting |
| Path parameters | `r.PathValue("id")` | `chi.URLParam(r, "id")` |
| External dependencies | Zero | Zero (chi has no deps) |
| Handler compatibility | `http.Handler` | `http.Handler` (identical) |
| Production prevalence | Lower for complex APIs | High |

**We use chi.** The middleware chain complexity in this system (auth → RBAC → logging → future rate limiting) makes stdlib unwieldy. chi solves exactly those problems with no lock-in — every handler is still a plain `http.Handler`.
