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

	ConfigurationFields = []field.SchemaField{
		ScimTokenField,
	}

	Config = field.NewConfiguration(
		ConfigurationFields,
		field.WithConnectorDisplayName("Notion"),
		field.WithHelpUrl("/docs/baton/notion"),
		field.WithIconUrl("/static/app-icons/notion.svg"),
	)
)
