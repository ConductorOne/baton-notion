package config

//go:generate go run ./gen

import (
	"github.com/conductorone/baton-sdk/pkg/field"
)

var (
	ScimTokenField = field.StringField(
		"scim-token",
		field.WithDisplayName("Notion SCIM API token"),
		field.WithDescription("The Notion SCIM token used to connect to the Notion SCIM API."),
		field.WithIsSecret(true),
		field.WithRequired(true),
	)

	BaseURLField = field.StringField(
		"base-url",
		field.WithDescription("Override the Notion SCIM API URL (for testing)"),
		field.WithHidden(true),
		field.WithExportTarget(field.ExportTargetCLIOnly),
	)

	ConfigurationFields = []field.SchemaField{
		ScimTokenField,
		BaseURLField,
	}

	Config = field.NewConfiguration(
		ConfigurationFields,
		field.WithConnectorDisplayName("Notion"),
		field.WithHelpUrl("/docs/baton/notion"),
		field.WithIconUrl("/static/app-icons/notion.svg"),
	)
)
