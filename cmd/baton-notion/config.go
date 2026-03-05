package main

import (
	"github.com/conductorone/baton-sdk/pkg/field"
	"github.com/spf13/viper"
)

const (
	scimTokenFlag = "scim-token"
	baseURLFlag   = "base-url"
)

var (
	SCIMTokenField = field.StringField(
		scimTokenFlag,
		field.WithRequired(false),
		field.WithDescription("The Notion SCIM token used to connect to the Notion SCIM API. ($BATON_SCIM_TOKEN)"),
	)
	BaseURLField = field.StringField(
		baseURLFlag,
		field.WithDescription("Override the Notion SCIM API URL (for testing)"),
		field.WithHidden(true),
	)

	ConfigurationFields = []field.SchemaField{SCIMTokenField, BaseURLField}
)

// ValidateConfig is run after the configuration is loaded, and should return an
// error if it isn't valid. Implementing this function is optional, it only
// needs to perform extra validations that cannot be encoded with configuration
// parameters.
func ValidateConfig(_ *viper.Viper) error {
	return nil
}
