# `baton-notion` [![Go Reference](https://pkg.go.dev/badge/github.com/conductorone/baton-notion.svg)](https://pkg.go.dev/github.com/conductorone/baton-notion) [![main ci](https://github.com/conductorone/baton-notion/actions/workflows/ci.yaml/badge.svg)](https://github.com/conductorone/baton-notion/actions/workflows/ci.yaml)

`baton-notion` is a connector for [Notion](https://www.notion.com) built using the [Baton SDK](https://github.com/conductorone/baton-sdk). It communicates with the [Notion SCIM 2.0 API](https://www.notion.com/help/provision-users-and-groups-with-scim) to sync workspace users, groups, and roles, and to provision accounts and role assignments.

Check out [Baton](https://github.com/conductorone/baton) to learn more about the project in general.

# Getting Started

## Prerequisites

1. A Notion workspace on the [Enterprise plan](https://www.notion.com/pricing) — SCIM provisioning is an Enterprise-only feature.
2. The **Organization Owner** role on that workspace. Only Organization Owners can generate SCIM API tokens.
3. A **SCIM API token** generated from the workspace's **Settings** > **Identity** > **SCIM provisioning** panel. See the [Notion help center](https://www.notion.com/help/provision-users-and-groups-with-scim) for the full walk-through.

## brew

```
brew install conductorone/baton/baton conductorone/baton/baton-notion

baton-notion --scim-token <scim-token>
baton resources
```

## docker

```
docker run --rm -v $(pwd):/out \
  -e BATON_SCIM_TOKEN=<scim-token> \
  ghcr.io/conductorone/baton-notion:latest -f "/out/sync.c1z"

docker run --rm -v $(pwd):/out \
  ghcr.io/conductorone/baton:latest -f "/out/sync.c1z" resources
```

## source

```
go install github.com/conductorone/baton/cmd/baton@main
go install github.com/conductorone/baton-notion/cmd/baton-notion@main

baton-notion --scim-token <scim-token>
baton resources
```

# Data Model

`baton-notion` syncs the following resources from Notion via the [SCIM API](https://www.notion.com/help/provision-users-and-groups-with-scim):

- **Users** — workspace members from `GET /scim/v2/Users`.
- **Groups** — workspace groups and their memberships from `GET /scim/v2/Groups`.
- **Roles** — the four Notion workspace roles (`owner`, `membership_admin`, `member`, `restricted_member`) exposed through the SCIM extension `urn:ietf:params:scim:schemas:extension:notion:2.0:User.role`.

# Provisioning

## Account Management

- **Create account** — invite a new member to the workspace (`POST /scim/v2/Users`).
- **Delete account** — remove a member from the workspace and revoke their active sessions (`DELETE /scim/v2/Users/{id}`). Notion does not permanently delete the underlying user record; that step must be performed manually in Notion.

## Entitlement Management

- **Grant role** — assign a workspace role to a user (`PATCH /scim/v2/Users/{id}` on the Notion role extension).
- **Revoke role** — Notion users always carry a role, so revoke is modelled as a downgrade to `restricted_member`. Revoking `restricted_member` itself is a no-op (the account must be deprovisioned to fully leave the workspace).

For local development and CI without a real Enterprise tenant, an in-process SCIM mock lives under [`test-server/`](./test-server/README.md).

# Contributing, Support and Issues

We started Baton because we were tired of taking screenshots and manually building spreadsheets. We welcome contributions, and ideas, no matter how small — our goal is to make identity and permissions sprawl less painful for everyone. If you have questions, problems, or ideas: Please open a GitHub Issue!

See [CONTRIBUTING.md](https://github.com/ConductorOne/baton/blob/main/CONTRIBUTING.md) for more details.

# `baton-notion` Command Line Usage

```
baton-notion

Usage:
  baton-notion [flags]
  baton-notion [command]

Available Commands:
  capabilities       Get connector capabilities
  completion         Generate the autocompletion script for the specified shell
  config             Get the connector config schema
  health-check       Check the health of a running connector
  help               Help about any command

Flags:
      --client-id string                                 The client ID used to authenticate with ConductorOne ($BATON_CLIENT_ID)
      --client-secret string                             The client secret used to authenticate with ConductorOne ($BATON_CLIENT_SECRET)
      --external-resource-c1z string                     The path to the c1z file to sync external baton resources with ($BATON_EXTERNAL_RESOURCE_C1Z)
      --external-resource-entitlement-id-filter string   The entitlement that external users, groups must have access to sync external baton resources ($BATON_EXTERNAL_RESOURCE_ENTITLEMENT_ID_FILTER)
  -f, --file string                                      The path to the c1z file to sync with ($BATON_FILE) (default "sync.c1z")
  -h, --help                                             help for baton-notion
      --log-format string                                The output format for logs: json, console ($BATON_LOG_FORMAT) (default "json")
      --log-level string                                 The log level: debug, info, warn, error ($BATON_LOG_LEVEL) (default "info")
      --otel-collector-endpoint string                   The endpoint of the OpenTelemetry collector to send observability data to ($BATON_OTEL_COLLECTOR_ENDPOINT)
  -p, --provisioning                                     This must be set in order for provisioning actions to be enabled ($BATON_PROVISIONING)
      --scim-token string                                required: The Notion SCIM token used to connect to the Notion SCIM API. ($BATON_SCIM_TOKEN)
      --skip-full-sync                                   This must be set to skip a full sync ($BATON_SKIP_FULL_SYNC)
      --ticketing                                        This must be set to enable ticketing support ($BATON_TICKETING)
  -v, --version                                          version for baton-notion

Use "baton-notion [command] --help" for more information about a command.
```
