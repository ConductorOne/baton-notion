# baton-notion test-server

In-process mock of [Notion's SCIM 2.0 API](https://www.notion.com/help/provision-users-and-groups-with-scim) used by the connector's CI and for local development. Implements the read endpoints, account create / delete, and `PATCH /Users/{id}` against the Notion workspace-role extension.

This is **not** a full SCIM 2.0 server. PATCH against arbitrary user attributes (name, email, etc.) is intentionally unsupported — the connector doesn't drive that path and Notion's "verified domain" caveat applies to those updates anyway.

## Run it

```bash
go run ./test-server                        # listens on :8765
go run ./test-server --addr=:9090           # custom port
```

On startup the server logs the base URL the connector should use:

```
baton-notion mock SCIM server listening on :8765
base URL for connector: http://localhost:8765/scim/v2
```

## Authentication

Every request must carry an `Authorization: Bearer <token>` header. **The token value is not validated** — any non-empty string is accepted. This lets CI use a literal `test-token` without secret plumbing. Requests without the header (or with an empty token) return SCIM `401 Unauthorized`.

## Point the connector at the mock

```bash
go build -o baton-notion ./cmd/baton-notion
BATON_SCIM_TOKEN=test-token ./baton-notion \
    --base-url=http://127.0.0.1:8765/scim/v2 \
    --file=sync.c1z

baton resources --file=sync.c1z
baton grants    --file=sync.c1z
```

## Endpoints

| Method | Path                             | Body         | Notes                                                                                 |
| ------ | -------------------------------- | ------------ | ------------------------------------------------------------------------------------- |
| GET    | `/scim/v2/ServiceProviderConfig` | —            | Advertises `patch: true`, OAuth Bearer auth scheme                                    |
| GET    | `/scim/v2/ResourceTypes`         | —            | Lists `User` (with Notion extension) and `Group`                                      |
| GET    | `/scim/v2/Users`                 | —            | Supports `startIndex` (1-indexed), `count` (≤100), `filter`                           |
| GET    | `/scim/v2/Users/{id}`            | —            | 404 with SCIM error envelope when missing                                             |
| POST   | `/scim/v2/Users`                 | SCIM User    | 201 on success, 409 (`uniqueness`) on duplicate `userName`, defaults role to `member` |
| PATCH  | `/scim/v2/Users/{id}`            | SCIM PatchOp | Only the workspace-role extension attribute is mutable                                |
| DELETE | `/scim/v2/Users/{id}`            | —            | 204 on success; also removes the user from every group's members                      |
| GET    | `/scim/v2/Groups`                | —            | Same `startIndex` / `count` / `filter` conventions as `/Users`                        |
| GET    | `/scim/v2/Groups/{id}`           | —            | 404 SCIM error envelope when missing                                                  |

### Supported `filter` expressions

Only `eq` is supported. The mock accepts the same attributes the [Notion help center](https://www.notion.com/help/provision-users-and-groups-with-scim) documents:

- `/Users` — `email eq "x"` (case-insensitive), `given_name eq "x"`, `family_name eq "x"`
- `/Groups` — `displayName eq "x"`

### `PATCH /Users/{id}` payload shapes

Both SCIM 2.0 PATCH spellings are accepted; the connector emits shape 1:

```jsonc
// Shape 1 — value-as-object (no `path`):
{
  "schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
  "Operations": [{
    "op": "replace",
    "value": {
      "urn:ietf:params:scim:schemas:extension:notion:2.0:User": { "role": "member" }
    }
  }]
}

// Shape 2 — path form:
{
  "schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
  "Operations": [{
    "op": "replace",
    "path": "urn:ietf:params:scim:schemas:extension:notion:2.0:User:role",
    "value": "member"
  }]
}
```

Valid role values: `owner`, `membership_admin`, `member`, `restricted_member`.

## Seed data

Resets on every restart (the store is in-memory). IDs are stable so CI can assert against them.

| User ID                                | Email                        | Role                |
| -------------------------------------- | ---------------------------- | ------------------- |
| `11111111-1111-1111-1111-111111111111` | `owner@example.com`          | `owner`             |
| `22222222-2222-2222-2222-222222222222` | `admin@example.com`          | `membership_admin`  |
| `33333333-3333-3333-3333-333333333333` | `carol.member@example.com`   | `member`            |
| `44444444-4444-4444-4444-444444444444` | `dave.member@example.com`    | `member`            |
| `55555555-5555-5555-5555-555555555555` | `eve.restricted@example.com` | `restricted_member` |

| Group ID                               | Display Name | Members            |
| -------------------------------------- | ------------ | ------------------ |
| `aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa` | Engineering  | Alice, Carol, Dave |
| `bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb` | Designers    | Bob, Eve           |

## Postman collection

A ready-to-use collection lives under `postman/`:

| File                                                 | Purpose                                  |
| ---------------------------------------------------- | ---------------------------------------- |
| `postman/baton-notion-mock.postman_collection.json`  | All endpoints with sample request bodies |
| `postman/baton-notion-mock.postman_environment.json` | `baseUrl`, `token`, and pre-filled IDs   |

**Import in Postman:** File → Import → drop both JSONs in → select the **baton-notion mock (local)** environment in the top-right dropdown. Then hit Send on any request — auth, base URL and IDs are wired automatically.
