package connector

import (
	"context"
	"testing"

	"github.com/conductorone/baton-notion/pkg/client"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/cli"
	rs "github.com/conductorone/baton-sdk/pkg/types/resource"
)

// newTestUserResourceWithRole builds a *v2.Resource the same way List() does,
// so Grants() sees a resource shaped exactly like the real sync path.
func newTestUserResourceWithRole(t *testing.T, role string) *v2.Resource {
	t.Helper()

	u := client.User{
		ID:       "user-1",
		UserName: "person@example.com",
		Active:   true,
	}
	u.Name.GivenName = "Person"
	u.Name.FamilyName = "One"
	u.Name.Formatted = "Person One"
	if role != "" {
		u.NotionExtension = &client.NotionUserExtension{Role: role}
	}

	res, err := parseIntoUserResource(u)
	if err != nil {
		t.Fatalf("parseIntoUserResource: %v", err)
	}
	return res
}

// TestUserBuilder_Grants_RoleGating asserts that userBuilder.Grants only
// emits the cross-type role grant (see role.go: roleBuilder.Grants is a
// no-op delegate to this method) when the "role" resource type is included
// in the sync -- either because no explicit filter was configured, or
// because "role" was explicitly selected. When a customer's sync filter
// excludes "role", no role grant should be emitted, since roleBuilder never
// lists role resources for that grant to reference.
func TestUserBuilder_Grants_RoleGating(t *testing.T) {
	tests := []struct {
		name         string
		syncRoles    bool
		wantGrantLen int
	}{
		{
			name:         "role type synced (default/no filter or explicit) -> role grant emitted",
			syncRoles:    true,
			wantGrantLen: 1,
		},
		{
			name:         "role type excluded from sync filter -> no role grant emitted",
			syncRoles:    false,
			wantGrantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &userBuilder{syncRoles: tt.syncRoles}

			userResource := newTestUserResourceWithRole(t, client.RoleMember)

			grants, _, err := b.Grants(context.Background(), userResource, rs.SyncOpAttrs{})
			if err != nil {
				t.Fatalf("Grants returned error: %v", err)
			}

			if len(grants) != tt.wantGrantLen {
				t.Fatalf("expected %d grants, got %d: %+v", tt.wantGrantLen, len(grants), grants)
			}

			if tt.wantGrantLen > 0 {
				got := grants[0].Entitlement.Resource.Id.ResourceType
				if got != roleResourceType.Id {
					t.Fatalf("expected grant to reference resource type %q, got %q", roleResourceType.Id, got)
				}
			}
		})
	}
}

// TestNewUserBuilder_SyncRolesWiring covers the constructor wiring: nil
// connectorOpts and an opts value with no explicit filter both mean "sync
// everything", matching cli.ConnectorOpts.WillSyncResourceType's own
// no-filter semantics. An explicit filter that excludes "role" should
// compute syncRoles=false; one that includes it should compute true.
func TestNewUserBuilder_SyncRolesWiring(t *testing.T) {
	tests := []struct {
		name          string
		connectorOpts *cli.ConnectorOpts
		wantSyncRoles bool
	}{
		{
			name:          "nil opts -> sync everything",
			connectorOpts: nil,
			wantSyncRoles: true,
		},
		{
			name:          "opts with no filter set -> sync everything",
			connectorOpts: &cli.ConnectorOpts{},
			wantSyncRoles: true,
		},
		{
			name:          "opts with role explicitly included",
			connectorOpts: &cli.ConnectorOpts{SyncResourceTypeIDs: []string{"user", "role"}},
			wantSyncRoles: true,
		},
		{
			name:          "opts with role explicitly excluded",
			connectorOpts: &cli.ConnectorOpts{SyncResourceTypeIDs: []string{"user", "group"}},
			wantSyncRoles: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := newUserBuilder(nil, tt.connectorOpts)
			if b.syncRoles != tt.wantSyncRoles {
				t.Fatalf("expected syncRoles=%v, got %v", tt.wantSyncRoles, b.syncRoles)
			}
		})
	}
}
