# baton-notion
`baton-notion` is a connector for [Notion](https://www.notion.com) built using the [Baton SDK](https://github.com/conductorone/baton-sdk). It communicates with the Notion API to sync data about users and groups.

Check out [Baton](https://github.com/conductorone/baton) to learn more the project in general.

# Getting Started

## Prerequisites

1. Enterprise Notion account with a workspace
2. Admin level access to the workspace
3. SCIM API Token

## brew

```
brew install conductorone/baton/baton conductorone/baton/baton-notion
baton-notion --scim-token <scim_token>
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

baton-notion --scim-token <scim_token>

baton resources
```

# Data Model

`baton-notion` pulls down information about the following Notion resources:
- Users
- Groups 


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
  capabilities       Get connector capabilities
  completion         Generate the autocompletion script for the specified shell
  help               Help about any command

Flags:
      --scim-token string      The Notion SCIM token used to connect to the Notion SCIM API. ($BATON_SCIM_TOKEN)
      --client-id string       The client ID used to authenticate with ConductorOne ($BATON_CLIENT_ID)
      --client-secret string   The client secret used to authenticate with ConductorOne ($BATON_CLIENT_SECRET)
  -f, --file string            The path to the c1z file to sync with ($BATON_FILE) (default "sync.c1z")
  -h, --help                   help for baton-notion
      --log-format string      The output format for logs: json, console ($BATON_LOG_FORMAT) (default "json")
      --log-level string       The log level: debug, info, warn, error ($BATON_LOG_LEVEL) (default "info")
  -p, --provisioning           This must be set in order for provisioning actions to be enabled. ($BATON_PROVISIONING)
  -v, --version                version for baton-notion

Use "baton-notion [command] --help" for more information about a command.
```