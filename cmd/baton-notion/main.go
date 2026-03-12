package main

import (
	"context"

	"github.com/conductorone/baton-notion/pkg/connector"
	cfg "github.com/conductorone/baton-notion/pkg/config"
	"github.com/conductorone/baton-sdk/pkg/config"
	"github.com/conductorone/baton-sdk/pkg/connectorrunner"
)

var version = "dev"

func main() {
	ctx := context.Background()
	config.RunConnector(ctx,
		"baton-notion",
		version,
		cfg.Config,
		connector.New,
		connectorrunner.WithDefaultCapabilitiesConnectorBuilderV2(&connector.Connector{}),
	)
}
