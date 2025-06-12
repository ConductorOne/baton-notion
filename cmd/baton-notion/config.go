package main

import (
	"github.com/conductorone/baton-sdk/pkg/field"
	"github.com/spf13/viper"
)

const (
	apiKeyFlag    = "api-key"
	scimTokenFlag = "scim-token"
)

var (
	APIKeyField = field.StringField(
		apiKeyFlag,
		field.WithRequired(true),
		field.WithDescription("The Notion API key used to connect to the Notion API. ($BATON_API_KEY)"),
	)

	SCIMTokenField = field.StringField(
		scimTokenFlag,
		field.WithRequired(false),
		field.WithDescription("The Notion SCIM token used to connect to the Notion SCIM API. ($BATON_SCIM_TOKEN)"),
	)

	ConfigurationFields = []field.SchemaField{APIKeyField, SCIMTokenField}
)

// ValidateConfig is run after the configuration is loaded, and should return an
// error if it isn't valid. Implementing this function is optional, it only
// needs to perform extra validations that cannot be encoded with configuration
// parameters.
func ValidateConfig(_ *viper.Viper) error {
	return nil
}
