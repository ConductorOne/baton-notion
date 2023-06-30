# baton-notion
`baton-notion` is a connector for Notion built using the [Baton SDK](https://github.com/conductorone/baton-sdk). It communicates with the Notion API to sync data about users and groups.

Check out [Baton](https://github.com/conductorone/baton) to learn more the project in general.

# Getting Started

## Prerequisites

1. Notion account with a workspace
2. Admin level access to the workspace
3. Created integration with access to the workspace. More info [here](https://developers.notion.com/docs/create-a-notion-integration#step-1-create-an-integration).
5. Set capabilities: 
  - Read content
  - Read user information including email addresses
4. Notion integration token, also called an API key used to communicate with Notion API. You can find it [here](https://www.notion.so/my-integrations).
5. If you have Enterprise Plan you can generate SCIM API token which can be used to sync information about Notion groups. You can create the token by going to `Settings & members → Security & identity → SCIM configuration`.

## brew

```
brew install conductorone/baton/baton conductorone/baton/baton-notion
baton-notion
baton resources
```

## docker

```
docker run --rm -v $(pwd):/out -e BATON_API_KEY=apiKey ghcr.io/conductorone/baton-notion:latest -f "/out/sync.c1z"
docker run --rm -v $(pwd):/out ghcr.io/conductorone/baton:latest -f "/out/sync.c1z" resources
```

## source

```
go install github.com/conductorone/baton/cmd/baton@main
go install github.com/conductorone/baton-notion/cmd/baton-notion@main

BATON_API_KEY=apiKey
baton resources
```

# Data Model

`baton-notion` pulls down information about the following Notion resources:
- Users
- Groups (only with Notion Enterprise Plan)

By default, `baton-notion` will only sync information about users. If you have an enterprise plan you can pass the SCIM token using the `--scim-token` flag and sync groups as well.

# Contributing, Support, and Issues

We started Baton because we were tired of taking screenshots and manually building spreadsheets. We welcome contributions, and ideas, no matter how small -- our goal is to make identity and permissions sprawl less painful for everyone. If you have questions, problems, or ideas: Please open a Github Issue!

See [CONTRIBUTING.md](https://github.com/ConductorOne/baton/blob/main/CONTRIBUTING.md) for more details.

# `baton-notion` Command Line Usage

```
baton-notion

Usage:
  baton-notion [flags]
  baton-notion [command]

Available Commands:
  completion         Generate the autocompletion script for the specified shell
  help               Help about any command

Flags:
      --api-key string                The Notion API key used to connect to the Notion API. ($BATON_API_KEY)
      --client-id string              The client ID used to authenticate with ConductorOne ($BATON_CLIENT_ID)
      --client-secret string          The client secret used to authenticate with ConductorOne ($BATON_CLIENT_SECRET)
  -f, --file string                   The path to the c1z file to sync with ($BATON_FILE) (default "sync.c1z")
      --grant-entitlement string      The entitlement to grant to the supplied principal ($BATON_GRANT_ENTITLEMENT)
      --grant-principal string        The resource to grant the entitlement to ($BATON_GRANT_PRINCIPAL)
      --grant-principal-type string   The resource type of the principal to grant the entitlement to ($BATON_GRANT_PRINCIPAL_TYPE)
  -h, --help                          help for baton-notion
      --log-format string             The output format for logs: json, console ($BATON_LOG_FORMAT) (default "json")
      --log-level string              The log level: debug, info, warn, error ($BATON_LOG_LEVEL) (default "info")
      --revoke-grant string           The grant to revoke ($BATON_REVOKE_GRANT)
      --scim-token string             The Notion SCIM token used to connect to the Notion SCIM API. ($BATON_SCIM_TOKEN)
  -v, --version                       version for baton-notion

Use "baton-notion [command] --help" for more information about a command.

```