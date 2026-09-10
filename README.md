# apic

`apic` turns a single YAML contract into a working Go HTTP server: request/response
types, route registration, JSON binding, and a gRPC-style error envelope — all
generated code, with your handler logic living in a plain Go interface you implement
by hand.

## Why

Wiring up a REST endpoint by hand means repeating the same boilerplate every time:
define a request struct, decode the body/query/path/headers into it, validate
required fields, call your handler, encode the response or map the error to the
right HTTP status. `apic` generates all of that from one YAML file, so the contract
is the single source of truth for your API's shape, and adding a route is a few
lines of YAML instead of a few files of plumbing.

## How it works

1. **Describe your API in `apic.yaml`** — routes, request/response fields, shared
   types, and middleware, e.g.:

   ```yaml
   version: 1
   service: Service

   groups:
     - prefix: /
       use: []
       routes:
         - GET /health:
             resp:
               status: str!
         - GET /posts/{id}:
             resp: Post
   ```

2. **Run `apic`**. With no subcommand it bootstraps (`apic init`, if `apic.yaml`
   doesn't exist yet) and generates (`apic generate`) in one go — so a brand new
   project just needs `apic` run twice: once to create the starter contract, once
   (after you edit it) to generate code.

3. **Generated output** lands in the contract's `out:` directory (`gen/` by
   default): request/response structs, a `RegisterRoutes` function wiring them to
   `net/http`, and a `Service` interface with one method per route — the only part
   you implement yourself.

4. **Implement `Service`** in your own `service.go` (or scaffold it instantly with
   `apic generate --stubs`, which stubs every method to return
   `apic.Unimplemented`) and pass it to `RegisterRoutes`.

5. **Keep contract and code in sync** with `apic diff`, which regenerates in memory
   and exits non-zero if the checked-in `gen/` package has drifted from
   `apic.yaml` — useful as a CI check.

### Runtime library

The `apic` Go package (imported by generated code) provides the pieces every
handler needs at request time:

- `Bind` — decodes `path:`, `query:`, `header:`, and `json:` struct tags out of an
  `*http.Request`, failing with `InvalidArgument` and a list of missing fields when
  a required field is absent.
- `Code` / `Status` — gRPC-style error codes (`NotFound`, `InvalidArgument`,
  `Unimplemented`, ...) mapped to HTTP status codes, so handlers return typed
  errors instead of picking status codes by hand.
- `WriteSuccess` / `WriteError` — encode responses (or errors) into a consistent
  JSON envelope (`{code, status, data|message, details}`).
- `Option`s (`WithEncoder`, `WithLogger`) — customize the envelope shape or where
  unexpected errors get logged, passed into generated `RegisterRoutes`.

## Contract reference

Top-level keys in `apic.yaml`:

| Key | Required | Meaning |
|---|---|---|
| `version` | yes | Contract format version (currently `1`). |
| `service` | yes | Name of the generated `Service` interface. |
| `out` | no | Output directory for generated files (default `gen/`). CLI-only, ignored by the parser itself. |
| `middlewares` | no | Named middlewares routes can `use`/`skip`. |
| `types` | no | Named, reusable field sets. |
| `groups` | yes | Route groups, each with a path `prefix` and its own `routes`. |

**Field types** (used in `body`, `query`, `header`, `resp`, and `types` blocks):

| Expr | Go type |
|---|---|
| `str` | `string` |
| `num` | `float64` |
| `int` | `int64` |
| `bool` | `bool` |
| `Name` | a named type from `types:` |
| `[]Name` | slice of any of the above (`[]str`, `[]Post`, ...) |

Append `!` to make a field required (`str!`, `[]Post!`); without it the generated
field is a pointer (`*string`) so its absence is distinguishable from the zero
value. A slice is never a pointer either way — see the `ponytail:` note on
`Bind` in `bind.go` for the one gap this leaves (a required slice can't be told
apart from an optional one at bind time).

`extends: Name` inside any fields block inlines every field of the named type
in place — used both to give a type its own fields via inheritance (`Post`
extending `Page`) and to reuse a type's fields directly in a route's `query`
block without a wrapper type.

**Middlewares:**

```yaml
middlewares:
  auth:
    provides: User   # injects `User` into the request struct as a ctx-sourced field
  ratelimit:          # no `provides` — runs, but adds nothing to the request
```

A group applies middlewares via `use: [auth, ratelimit]`; an individual route can
add more with its own `use:` or opt out of a group-level one with `skip: [auth]`
(only valid for middlewares the group already `use`s).

**Routes** are `"METHOD /path":` entries (`GET`, `POST`, `PUT`, `PATCH`, `DELETE`)
under a group's `routes:` list, each with:

| Key | Meaning |
|---|---|
| `name` | Overrides the generated method name (default: derived from method + path). |
| `body` | JSON body fields. |
| `query` | Query-string fields. |
| `header` | Header fields (`X-Client-Id: str`). |
| `resp` | Response fields, or a bare type name (`resp: Post`) to reuse a named type. Omit `resp` entirely for a 204 No Content response. |
| `use` / `skip` | Per-route middleware additions/exclusions. |

Path parameters (`/posts/{id}`) are picked up automatically as required
string fields — no need to declare `id` under `body`/`query`.

See `internal/gen/testdata/example.yaml` for a complete contract, and
`docs/superpowers/specs/2026-09-10-apic-parser-generator-design.md` for the
full design/validation rules (§5.6).

## Commands

| Command | What it does |
|---|---|
| `apic` | Bootstrap + generate in one step (init if `apic.yaml` is missing, then generate). |
| `apic init` | Write a starter `apic.yaml` (`--force` to overwrite). |
| `apic generate` | Parse, validate, and regenerate the `gen/` package (`--stubs` to also scaffold `service.go`). |
| `apic diff` | Check whether `gen/` is up to date with `apic.yaml`; non-zero exit on drift. |

## License

MIT — see [LICENSE](LICENSE).
