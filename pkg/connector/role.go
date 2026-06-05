// Notion workspace roles, modeled as a first-class C1 Role resource.
//
// The Notion SCIM API exposes the workspace role as a fixed enum on the
// `urn:ietf:params:scim:schemas:extension:notion:2.0:User` extension
// (owner | membership_admin | member | restricted_member). There is no
// /Licenses endpoint and no `license` attribute — "license" in Notion
// means "paid seat" and is binary, not tiered — so this connector models
// the role itself as the resource.
//
// Grants are emitted from userBuilder.Grants (one per user) to keep sync
// at O(N) without re-paginating users once per role. Grant/Revoke call
// PATCH /scim/v2/Users/{id} on the role attribute. Revoke is modeled as
// a downgrade to restricted_member since Notion users must always carry
// a role.
//
// Every paid Notion role consumes one workspace seat, so each role doubles
// as a license tier. The resource carries TRAIT_LICENSE_PROFILE alongside
// TRAIT_ROLE (Adobe-style hybrid — see baton-adobe PR #34 / CXH-1573) and
// wires WithLicenseEntitlementIDs so the C1 App Utilization feature
// (CE-720) can correlate seat-holders to their grants. Seat counts are
// intentionally omitted: Notion's SCIM API does not expose a per-role
// purchased/consumed endpoint.
//
// References:
//   - https://www.notion.com/help/provision-users-and-groups-with-scim
//   - LicenseProfileTrait: baton-sdk pkg/types/resource/license_trait.go
package connector

import (
	"context"
	"fmt"

	"github.com/conductorone/baton-notion/pkg/client"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	ent "github.com/conductorone/baton-sdk/pkg/types/entitlement"
	rs "github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"
)

const (
	assignedEntitlement = "assigned"

	// defaultRevokedRole is the floor tier a user is downgraded to on revoke.
	defaultRevokedRole = client.RoleRestrictedMember
)

type notionRole struct {
	id          string
	displayName string
	description string
}

var supportedRoles = []notionRole{
	{
		id:          client.RoleOwner,
		displayName: "Owner",
		description: "Workspace owner. Has full administrative control, including billing, security, and SCIM token management.",
	},
	{
		id:          client.RoleMembershipAdmin,
		displayName: "Membership Admin",
		description: "Can add and remove workspace members and manage groups, but cannot access billing or security settings.",
	},
	{
		id:          client.RoleMember,
		displayName: "Member",
		description: "Standard workspace member with full access to non-administrative workspace content.",
	},
	{
		id:          client.RoleRestrictedMember,
		displayName: "Restricted Member",
		description: "Workspace member with restricted access — useful for contractors or contributors who should only see explicitly shared pages.",
	},
}

type roleBuilder struct {
	client *client.NotionClient
}

func (b *roleBuilder) ResourceType(_ context.Context) *v2.ResourceType {
	return roleResourceType
}

func roleResource(role notionRole) (*v2.Resource, error) {
	roleTraitOpts := []rs.RoleTraitOption{
		rs.WithRoleProfile(map[string]any{
			"role_id":           role.id,
			"description":       role.description,
			"is_administrative": role.id == client.RoleOwner || role.id == client.RoleMembershipAdmin,
		}),
	}

	// EntitlementIDs links the seat counts back to the grants that consume
	// them so the C1 App Utilization feature (CE-720) can map seat-holders to
	// their grants without manual entitlement mapping. WithLicenseSeats is
	// intentionally not set: Notion's SCIM API does not expose a per-role
	// purchased/consumed endpoint.
	assignedEntitlementID := ent.NewEntitlementID(
		&v2.Resource{Id: &v2.ResourceId{ResourceType: roleResourceType.Id, Resource: role.id}},
		assignedEntitlement,
	)
	licenseOpts := []rs.LicenseProfileTraitOption{
		rs.WithLicenseName(role.displayName),
		rs.WithLicenseEntitlementIDs(assignedEntitlementID),
	}

	return rs.NewRoleResource(
		role.displayName,
		roleResourceType,
		role.id,
		roleTraitOpts,
		rs.WithLicenseProfileTrait(licenseOpts...),
	)
}

func (b *roleBuilder) List(_ context.Context, _ *v2.ResourceId, _ rs.SyncOpAttrs) ([]*v2.Resource, *rs.SyncOpResults, error) {
	resources := make([]*v2.Resource, 0, len(supportedRoles))
	for _, role := range supportedRoles {
		r, err := roleResource(role)
		if err != nil {
			return nil, nil, fmt.Errorf("baton-notion: failed to build role resource %q: %w", role.id, err)
		}
		resources = append(resources, r)
	}
	return resources, nil, nil
}

func (b *roleBuilder) Entitlements(_ context.Context, resource *v2.Resource, _ rs.SyncOpAttrs) ([]*v2.Entitlement, *rs.SyncOpResults, error) {
	description := fmt.Sprintf("Assigned the %s role in Notion", resource.DisplayName)
	for _, role := range supportedRoles {
		if role.id == resource.Id.Resource {
			description = role.description
			break
		}
	}

	options := []ent.EntitlementOption{
		ent.WithGrantableTo(userResourceType),
		ent.WithDisplayName(fmt.Sprintf("%s role %s", resource.DisplayName, assignedEntitlement)),
		ent.WithDescription(description),
	}
	return []*v2.Entitlement{ent.NewAssignmentEntitlement(resource, assignedEntitlement, options...)}, nil, nil
}

// Grants is intentionally empty — role grants are emitted from userBuilder.Grants.
func (b *roleBuilder) Grants(_ context.Context, _ *v2.Resource, _ rs.SyncOpAttrs) ([]*v2.Grant, *rs.SyncOpResults, error) {
	return nil, nil, nil
}

// Grant assigns a Notion workspace role to a user. Idempotent: if the user
// already holds the target role, returns GrantAlreadyExists without issuing
// a PATCH; a 409 from the PATCH is also treated as success.
func (b *roleBuilder) Grant(
	ctx context.Context, principal *v2.Resource, entitlement *v2.Entitlement,
) ([]*v2.Grant, annotations.Annotations, error) {
	if principal.Id.ResourceType != userResourceType.Id {
		return nil, nil, fmt.Errorf("baton-notion: roles can only be granted to users, got principal type %q", principal.Id.ResourceType)
	}

	userID := principal.Id.Resource
	targetRole := entitlement.Resource.Id.Resource

	if !isSupportedRole(targetRole) {
		return nil, nil, fmt.Errorf("baton-notion: unsupported role %q", targetRole)
	}

	current, err := b.client.GetUser(ctx, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("baton-notion: grant role %s to user %s (fetch current): %w", targetRole, userID, err)
	}

	if current.Role() == targetRole {
		return nil, annotations.New(&v2.GrantAlreadyExists{}), nil
	}

	if _, err := b.client.PatchUserRole(ctx, userID, targetRole); err != nil {
		return nil, nil, fmt.Errorf("baton-notion: grant role %s to user %s: %w", targetRole, userID, err)
	}

	return nil, nil, nil
}

// Revoke downgrades the user to restricted_member (the floor tier), since
// Notion users must always carry a role. Idempotent: returns
// GrantAlreadyRevoked when the user no longer holds the role, when the user
// is gone (404), or when the role being revoked is already restricted_member
// — in the last case, the account must be deprovisioned to fully remove the
// user from the workspace.
func (b *roleBuilder) Revoke(ctx context.Context, grant *v2.Grant) (annotations.Annotations, error) {
	logger := ctxzap.Extract(ctx)
	principal := grant.Principal

	if principal.Id.ResourceType != userResourceType.Id {
		return nil, fmt.Errorf("baton-notion: roles can only be revoked from users, got principal type %q", principal.Id.ResourceType)
	}

	userID := principal.Id.Resource
	targetRole := grant.Entitlement.Resource.Id.Resource

	if !isSupportedRole(targetRole) {
		return nil, fmt.Errorf("baton-notion: unsupported role %q", targetRole)
	}

	current, err := b.client.GetUser(ctx, userID)
	if err != nil {
		if client.IsNotFoundError(err) {
			return annotations.New(&v2.GrantAlreadyRevoked{}), nil
		}
		return nil, fmt.Errorf("baton-notion: revoke role %s from user %s (fetch current): %w", targetRole, userID, err)
	}

	if current.Role() != targetRole {
		return annotations.New(&v2.GrantAlreadyRevoked{}), nil
	}

	if targetRole == defaultRevokedRole {
		logger.Warn(
			"baton-notion: cannot revoke restricted_member role — it is the floor tier; deprovision the account to remove the user from the workspace",
			zap.String("user_id", userID),
			zap.String("role_id", targetRole),
		)
		return annotations.New(&v2.GrantAlreadyRevoked{}), nil
	}

	if _, err := b.client.PatchUserRole(ctx, userID, defaultRevokedRole); err != nil {
		if client.IsNotFoundError(err) {
			return annotations.New(&v2.GrantAlreadyRevoked{}), nil
		}
		return nil, fmt.Errorf("baton-notion: revoke role %s from user %s (downgrade to %s): %w", targetRole, userID, defaultRevokedRole, err)
	}

	return nil, nil
}

func newRoleBuilder(scimClient *client.NotionClient) *roleBuilder {
	return &roleBuilder{client: scimClient}
}

func isSupportedRole(roleID string) bool {
	for _, role := range supportedRoles {
		if role.id == roleID {
			return true
		}
	}
	return false
}
