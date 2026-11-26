# Books API

REST API in Go with JWT authentication. Users register and log in, and only logged-in users can use the books CRUD endpoints.

**Stack:** Gin, PostgreSQL (pgx), golang-migrate, JWT (HS256), bcrypt, Docker Compose.

## Features

- Register / login with email and password; login returns an `access_token` (JWT)
- All `/api/v1/books` routes require `Authorization: Bearer <access_token>`
- Strict request validation with field-level error messages
- One JSON format for every success and error response, with custom messages and status codes
- Request IDs, structured logging, panic recovery, graceful shutdown
- Rate limiting on register and login
- Versioned SQL migrations that run automatically on startup
- Tests that need no database

## Project layout

```
golang_crud/
├── cmd/api/main.go            # entry point: config, wiring, graceful shutdown
├── auth/
│   ├── password.go            # bcrypt hashing and checking
│   └── token.go               # JWT issue / verify
├── config/config.go           # environment variables, fails fast when misconfigured
├── database/
│   ├── postgres.go            # connection pool
│   └── migrate.go             # runs embedded migrations on startup
├── handlers/
│   ├── auth_handlers.go       # register, login, me
│   ├── books_handlers.go      # books CRUD
│   └── health_handlers.go     # GET /health
├── middleware/
│   ├── auth.go                # requires and verifies the Bearer token
│   ├── rate_limit.go          # per-IP limiter (login / register)
│   ├── request_id.go          # X-Request-ID on every request
│   ├── logger.go              # one structured log line per request
│   └── recovery.go            # panics become a clean JSON 500
├── models/                    # Book, User and domain errors
├── repository/                # all SQL (books and users)
├── routes/routes.go           # URL -> handler mapping, public vs protected routes
├── utils/
│   ├── errors.go              # AppError and error codes
│   ├── response.go            # success / error JSON envelope
│   ├── binding.go             # strict JSON + query binding
│   └── validator.go           # custom rules and friendly messages
├── migrations/                # versioned .sql files (embedded in the binary)
├── test/                      # all tests (separate package, in-memory fakes)
├── Dockerfile  docker-compose.yml  Makefile
├── .env.example  .gitignore  .dockerignore
└── go.mod  go.sum
```

A request flows `routes -> middleware -> handlers -> repository -> database`, and the response comes back the same way.
Handlers never write SQL, and repositories never know about HTTP.

## First-time setup

```bash
cp .env.example .env
openssl rand -base64 48      # copy the output into JWT_SECRET in .env
go mod tidy                  # downloads dependencies and CREATES go.sum
make db-up                   # PostgreSQL in Docker
make run                     # migrations run automatically on startup
```

Commit both `go.mod` and `go.sum`. Never commit `.env`.

### Configuration

All settings come from environment variables (a local `.env` file is loaded if present).
The app refuses to start if a required value is missing or unsafe.

| Variable | Required | Default | Meaning |
|---|---|---|---|
| `DATABASE_URL` | yes | none | Postgres connection string (host `localhost` when running on your machine) |
| `JWT_SECRET` | yes | none | Signing key, at least 32 characters. Use a different one per environment |
| `JWT_ISSUER` | no | `golang_crud` | `iss` claim; tokens from other issuers are rejected |
| `ACCESS_TOKEN_TTL` | no | `1h` | Token lifetime, e.g. `15m`, `1h`. Maximum `24h` |
| `PORT` | no | `8080` | HTTP port |
| `GIN_MODE` | no | `debug` | `release` in production (also switches logs to JSON) |
| `RUN_MIGRATIONS` | no | `true` | Apply pending migrations on startup |
| `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB` | Docker only | none | Used by `docker-compose.yml` for the database and the API container |

Want a different module name (e.g. `github.com/you/golang_crud`)? Replace it in `go.mod` and in every import:

```bash
grep -rl 'golang_crud/' --include='*.go' . | xargs sed -i 's#"golang_crud/#"github.com/you/golang_crud/#'
sed -i 's#^module golang_crud#module github.com/you/golang_crud#' go.mod
```

## Everything in Docker

```bash
go mod tidy          # go.sum must exist before building the image
make docker-up       # builds the API image and starts API + PostgreSQL
```

`.env` must contain `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB` and `JWT_SECRET`.
Inside Docker the API connects to the host `db`, so `docker-compose.yml` builds its own `DATABASE_URL` from those values.
Your `DATABASE_URL` in `.env` (with `localhost`) is only for running the app on your machine.

## Endpoints

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/health` | public | Liveness + database check |
| POST | `/api/v1/auth/register` | public | Create an account |
| POST | `/api/v1/auth/login` | public | Log in, returns `access_token` |
| GET | `/api/v1/auth/me` | **token** | Current user's profile |
| POST | `/api/v1/books` | **token** | Create a book |
| GET | `/api/v1/books?limit=20&offset=0` | **token** | List books (paginated, limit 1-100) |
| GET | `/api/v1/books/:id` | **token** | Get one book |
| PUT | `/api/v1/books/:id` | **token** | Update a book |
| DELETE | `/api/v1/books/:id` | **token** | Delete a book |

## Authentication

### 1. Register

```bash
curl -X POST localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"name":"Ada Lovelace","email":"ada@example.com","password":"secret123"}'
```

Rules: `name` required (max 100); `email` valid (stored lowercased); `password` 8-72 bytes with at least one letter and one number.
Returns `201`. Registering an email that already exists returns `409`.

### 2. Log in

```bash
curl -X POST localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"ada@example.com","password":"secret123"}'
```

```json
{
  "success": true,
  "message": "Login successful",
  "data": {
    "access_token": "eyJhbGciOi...",
    "token_type": "Bearer",
    "expires_in": 3600,
    "user": { "id": 1, "name": "Ada Lovelace", "email": "ada@example.com" }
  },
  "request_id": "9f2c..."
}
```

### 3. Call protected endpoints

Send the token in the `Authorization` header:

```bash
TOKEN="<paste access_token here>"

curl localhost:8080/api/v1/auth/me -H "Authorization: Bearer $TOKEN"

curl -X POST localhost:8080/api/v1/books \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"title":"The Go Programming Language","author":"Donovan","year":2015}'

curl localhost:8080/api/v1/books -H "Authorization: Bearer $TOKEN"
```

### How it works

- Passwords are hashed with bcrypt (cost 12). The plain password and the hash are never returned or logged.
- The token is a JWT signed with HS256. It holds the user ID (`sub`), email, issuer, issue time, expiry and a unique `jti`.
- On every protected request the middleware checks the signature, the algorithm (pinned to HS256), the issuer and the expiry.
  Only then does the handler run.
- Wrong password and unknown email return the same `401` message, and the unknown-email path takes the same time,
  so the API does not reveal which emails are registered.
- When the token expires, the client gets `401` with code `TOKEN_EXPIRED` and must log in again.
- Register and login are limited to 10 requests per minute per IP (bursts of 5); beyond that you get `429` and a `Retry-After` header.

### Limitations

- **No refresh tokens or logout.** A JWT is stateless, so a stolen token works until it expires. Keep `ACCESS_TOKEN_TTL` short.
  Real logout needs a refresh-token table or a revocation list (each token already has a unique `jti` for this).
- **Books are not owned by users.** Any logged-in user can read, edit and delete any book.
  To make them per-user, add a `user_id` column with a new migration and filter queries by `utils.UserID(c)`.
- **The rate limiter is in memory**, so it is per server instance. Behind a reverse proxy every client looks like the proxy's IP
  unless you configure trusted proxies. For several instances, limit at the proxy or use Redis.
- **Use HTTPS in production.** Tokens and passwords travel in plain text over HTTP.

## Response format

Every endpoint returns the same JSON shape.

Success:

```json
{
  "success": true,
  "message": "Book created successfully",
  "data": { "id": 1, "title": "The Go Programming Language", "author": "Donovan", "year": 2015 },
  "meta": { "limit": 20, "offset": 0, "count": 1 },
  "request_id": "9f2c..."
}
```

(`meta` appears on list responses only.)

Error:

```json
{
  "success": false,
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Validation failed",
    "details": [
      { "field": "title", "message": "title is required" },
      { "field": "year", "message": "year cannot be in the future" }
    ]
  },
  "request_id": "9f2c..."
}
```

Clients should switch on `error.code`, not on the message text. Every response also has an `X-Request-ID` header,
which matches the `request_id` in the body and in the server logs.

| Status | `error.code` | When |
|---|---|---|
| 200 / 201 | n/a | Success / created |
| 400 | `INVALID_JSON`, `BAD_REQUEST` | Empty or malformed JSON, unparseable query value |
| 401 | `UNAUTHORIZED` | Missing, malformed or invalid token; wrong email or password |
| 401 | `TOKEN_EXPIRED` | Token is valid but expired |
| 404 | `NOT_FOUND` | Book or route does not exist |
| 405 | `METHOD_NOT_ALLOWED` | Wrong HTTP method for the route |
| 409 | `CONFLICT` | Email already registered |
| 413 | `PAYLOAD_TOO_LARGE` | Body larger than 1 MiB |
| 415 | `UNSUPPORTED_MEDIA_TYPE` | Content-Type is not `application/json` |
| 422 | `VALIDATION_ERROR` | Invalid fields, unknown fields, wrong types, bad `:id`, bad `limit` / `offset` |
| 429 | `TOO_MANY_REQUESTS` | Rate limit hit on register or login |
| 500 | `INTERNAL_ERROR` | Unexpected error. The real cause is logged, never sent to the client |
| 503 | `SERVICE_UNAVAILABLE` | Database unreachable (`/health`) |

## Validation

Rules are `binding:"..."` tags on the request structs in `handlers/`. The JSON body is decoded strictly:
unknown fields, wrong types, trailing data, empty bodies and bodies over 1 MiB are all rejected with a clear message.

| Field | Rules |
|---|---|
| book `title`, `author` | required, not blank, max 255 (surrounding spaces are trimmed) |
| book `year` | optional, at least 1, not later than next year |
| user `name` | required, not blank, max 100 |
| user `email` | required, valid email, max 254 |
| user `password` | required, 8-72 bytes, at least one letter and one number |

To add a rule: put it in the struct tag, and if it is a new custom tag, register it and add its message in `utils/validator.go`.

## Makefile commands

| Command | What it does |
|---|---|
| `make run` | Run the API locally |
| `make build` | Build the binary into `bin/api` |
| `make test` | Run all tests |
| `make fmt` / `make vet` / `make tidy` | Format, static checks, sync `go.mod` and `go.sum` |
| `make db-up` / `make db-down` | Start / stop only PostgreSQL in Docker |
| `make docker-up` / `make docker-down` | Run / stop API + PostgreSQL in Docker |
| `make migrate-*` | Migration commands, see below |

## Migrations

A migration is a pair of SQL files that change the **database schema** (create a table, add a column, add an index).
`migrate` only runs SQL files. It never reads your Go code, and your Go code never changes the schema.
So adding a column needs **both**: a migration (database) and a code change (model + queries).

Current migrations: `000001` creates `books`, `000002` creates `users` (with a unique email).

### What each command does

| Command | What it does | Does it change the database? |
|---|---|---|
| `make migrate-install` | Installs the `migrate` CLI tool (once per machine) | No |
| `make migrate-create name=xyz` | Creates two **empty** files: `NNNNNN_xyz.up.sql` and `NNNNNN_xyz.down.sql` | No |
| `make migrate-up` | Runs every `.up.sql` that hasn't been applied yet | **Yes** |
| `make migrate-down` | Runs the `.down.sql` of the last applied migration (undo one step) | **Yes** |
| `make migrate-version` | Shows which migration version the database is at | No |
| `make migrate-force version=N` | Marks the database as version N without running SQL (only to fix a "dirty" state) | Only the version marker |

The migrate commands read `DATABASE_URL` from `.env`. If `migrate: command not found` appears after installing,
add Go's bin folder to your PATH: `export PATH=$PATH:$(go env GOPATH)/bin`.

`migrate-create` does **not** create the column. You write the SQL yourself in step 2 below.

### Workflow: change the schema

```bash
# 1. Create the empty pair of files
make migrate-create name=add_isbn_to_books
#    -> migrations/000003_add_isbn_to_books.up.sql
#    -> migrations/000003_add_isbn_to_books.down.sql

# 2. Write the SQL yourself
#    up.sql   : ALTER TABLE books ADD COLUMN isbn TEXT;
#    down.sql : ALTER TABLE books DROP COLUMN isbn;

# 3. Apply it (or just restart the app: it applies pending migrations on startup)
make migrate-up

# 4. Update the Go code to use the new column (models, repository, handlers)

# Changed your mind? Undo the last migration:
make migrate-down
```

The `up` file moves the schema forward and the `down` file must exactly reverse it.
`migrate` records the current version in a table called `schema_migrations`, which is how it knows what is already applied.

You do not need to run `make migrate-up` if the app starts with `RUN_MIGRATIONS=true`; the app does the same thing on startup.
Both use the same `schema_migrations` table, so mixing them is safe.
New `.sql` files are embedded in the binary at build time, so restart (or rebuild) the app after adding one.

### Adding another model (example: categories)

Say you want a new `categories` resource. There are two separate parts.

**Part A: the database (migration)**

```bash
make migrate-create name=create_categories_table
```

`migrations/000003_create_categories_table.up.sql`
```sql
CREATE TABLE IF NOT EXISTS categories (
    id         BIGSERIAL   PRIMARY KEY,
    name       TEXT        NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

`migrations/000003_create_categories_table.down.sql`
```sql
DROP TABLE IF EXISTS categories;
```

```bash
make migrate-up
```

**Part B: the Go code (copy the pattern used for books)**

| Step | File to add or edit |
|---|---|
| Struct | `models/category.go` |
| SQL queries | `repository/categories_repository.go` |
| HTTP handlers (+ a `CategoryStore` interface) | `handlers/categories_handlers.go` |
| Routes under `/api/v1/categories`, inside the protected group so they need a token | edit `routes/routes.go` |
| Create the repo and handler, pass them to the router | edit `cmd/api/main.go` |
| Tests | add `test/categories_handlers_test.go` |

To link two tables (for example each book belongs to a category), use another migration. Do not edit an old one:

```bash
make migrate-create name=add_category_id_to_books
```
```sql
-- up
ALTER TABLE books ADD COLUMN category_id BIGINT REFERENCES categories(id);
-- down
ALTER TABLE books DROP COLUMN category_id;
```

Then add `CategoryID` to `models.Book` and update the SQL in `repository/books_repository.go`.

### Rules of thumb

- One migration per change; give it a clear name (`add_isbn_to_books`, `create_categories_table`).
- Always write the `down` file so you can undo the change.
- **Never edit a migration that has already been applied** (especially on a shared or production database). Add a new migration instead.
- Before committing, test the cycle on your local database: `make migrate-up`, then `make migrate-down`, then `make migrate-up` again.

### If you see "Dirty database version N"

A migration failed halfway, and `migrate` refuses to continue until you fix it.

1. Look at the error and fix the SQL in the failing migration file (safe, because it never fully applied). If part of it did run, undo that part by hand in `psql`.
2. Reset the marker to the last good version: `make migrate-force version=N-1` (for a failed migration 3, run `make migrate-force version=2`).
3. Run `make migrate-up` again.

## Tests

```bash
make test                                  # all tests
go test ./test/... -v                      # list every test
go test ./test/... -run TestLogin          # only tests whose name matches
go test ./test/... -cover -coverpkg=./...  # coverage of the app code
```

All tests live in the `test/` package and use in-memory fake stores, so **no database is needed**.

| File | Covers |
|---|---|
| `test/helpers_test.go` | Fakes, test router, request helpers |
| `test/books_handlers_test.go` | Books CRUD, validation, not found |
| `test/auth_handlers_test.go` | Register, login, me |
| `test/middleware_test.go` | Token checks (missing, forged, expired), 404 / 405, request IDs |

## Troubleshooting

| Problem | Fix |
|---|---|
| `invalid configuration: JWT_SECRET is required...` | Set `JWT_SECRET` in `.env` (at least 32 characters: `openssl rand -base64 48`) |
| `invalid configuration: DATABASE_URL is required` | Add `DATABASE_URL` to `.env` |
| `401 UNAUTHORIZED` on books routes | Send `Authorization: Bearer <access_token>` (log in first) |
| `401 TOKEN_EXPIRED` | Log in again to get a new token |
| `422 ... unknown field` on login | Login accepts only `email` and `password`; unknown fields are rejected on purpose |
| `connection refused` on startup | PostgreSQL is not ready yet: run `make db-up`, wait a few seconds |
| `port 5432 is already allocated` | Another Postgres is running. Stop it, or change the port in `docker-compose.yml` and `.env` |
| `Makefile: *** missing separator` | Recipe lines in the Makefile must start with a real tab, not spaces |
| `migrate: command not found` | `make migrate-install`, then add `$(go env GOPATH)/bin` to your PATH |
| `Dirty database version N` | See "If you see Dirty database version N" above |