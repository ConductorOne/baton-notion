package client

// API error classification helpers used by pkg/connector. Centralizing this
// here keeps Notion's error semantics out of the builder layer.
//
// uhttp.BaseHttpClient maps non-2xx HTTP responses to gRPC status codes; see
// https://github.com/conductorone/baton-sdk/blob/main/pkg/uhttp/wrapper.go
// (`getResponseStatusCodeFromHTTPResponse`). status.Code() unwraps
// fmt.Errorf("%w", ...) chains, so wrapped errors classify correctly.

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	NotionUserExtensionSchema = "urn:ietf:params:scim:schemas:extension:notion:2.0:User"

	SCIMPatchOpSchema = "urn:ietf:params:scim:api:messages:2.0:PatchOp"

	// Valid Notion workspace role values exposed through the SCIM extension.
	RoleOwner            = "owner"
	RoleMembershipAdmin  = "membership_admin"
	RoleMember           = "member"
	RoleRestrictedMember = "restricted_member"
)

// IsNotFoundError reports whether err carries the gRPC NotFound status —
// i.e. the upstream returned HTTP 404. Used by Revoke to short-circuit when
// the target user has been deleted between sync and revoke.
func IsNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	return status.Code(err) == codes.NotFound
}
