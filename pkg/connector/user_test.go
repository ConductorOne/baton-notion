package connector

import (
	"context"
	"testing"

	"github.com/conductorone/baton-notion/pkg/client"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
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

// TestUserBuilder_Grants_AlwaysEmitsRoleGrant asserts that userBuilder.Grants
// unconditionally emits the cross-type role grant (see role.go:
// roleBuilder.Grants is a no-op delegate to this method) whenever the user
// has a supported role, regardless of syncRoles. Gating on whether "role" is
// included in the sync now happens in ResourceType via annotations (see
// TestUserBuilder_ResourceType_Annotations below), not inside Grants itself
// -- when "role" is excluded from the sync, the SDK simply never calls
// Grants for user resources at all, so Grants has no need to check syncRoles
// on its own.
func TestUserBuilder_Grants_AlwaysEmitsRoleGrant(t *testing.T) {
	tests := []struct {
		name      string
		syncRoles bool
	}{
		{name: "syncRoles=true -> role grant emitted", syncRoles: true},
		{name: "syncRoles=false -> role grant still emitted", syncRoles: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &userBuilder{syncRoles: tt.syncRoles}

			userResource := newTestUserResourceWithRole(t, client.RoleMember)

			grants, _, err := b.Grants(context.Background(), userResource, rs.SyncOpAttrs{})
			if err != nil {
				t.Fatalf("Grants returned error: %v", err)
			}

			if len(grants) != 1 {
				t.Fatalf("expected 1 grant, got %d: %+v", len(grants), grants)
			}

			got := grants[0].Entitlement.Resource.Id.ResourceType
			if got != roleResourceType.Id {
				t.Fatalf("expected grant to reference resource type %q, got %q", roleResourceType.Id, got)
			}
		})
	}
}

// TestUserBuilder_ResourceType_Annotations asserts that ResourceType sets the
// annotation the SDK uses to decide whether to call Entitlements/Grants for
// user resources: SkipEntitlements when "role" is being synced (user still
// has no entitlements of its own), or SkipEntitlementsAndGrants when it is
// not (so the SDK skips calling Grants entirely, since userBuilder.Grants
// would otherwise emit a dangling role grant). The two annotations are
// mutually exclusive -- SkipEntitlementsAndGrants supersedes
// SkipEntitlements, so both should never be set at once.
func TestUserBuilder_ResourceType_Annotations(t *testing.T) {
	tests := []struct {
		name                          string
		syncRoles                     bool
		wantSkipEntitlements          bool
		wantSkipEntitlementsAndGrants bool
	}{
		{
			name:                          "syncRoles=true -> SkipEntitlements only",
			syncRoles:                     true,
			wantSkipEntitlements:          true,
			wantSkipEntitlementsAndGrants: false,
		},
		{
			name:                          "syncRoles=false -> SkipEntitlementsAndGrants only",
			syncRoles:                     false,
			wantSkipEntitlements:          false,
			wantSkipEntitlementsAndGrants: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &userBuilder{syncRoles: tt.syncRoles}

			rt := b.ResourceType(context.Background())

			annos := annotations.Annotations(rt.Annotations)
			gotSkipEnts := annos.Contains(&v2.SkipEntitlements{})
			gotSkipEntsAndGrants := annos.Contains(&v2.SkipEntitlementsAndGrants{})

			if gotSkipEnts != tt.wantSkipEntitlements {
				t.Fatalf("SkipEntitlements: expected %v, got %v", tt.wantSkipEntitlements, gotSkipEnts)
			}
			if gotSkipEntsAndGrants != tt.wantSkipEntitlementsAndGrants {
				t.Fatalf("SkipEntitlementsAndGrants: expected %v, got %v", tt.wantSkipEntitlementsAndGrants, gotSkipEntsAndGrants)
			}
		})
	}
}

// TestUserBuilder_ResourceType_DoesNotMutatePackageVar guards against a
// regression where ResourceType mutates the package-level userResourceType
// var in place (e.g. by appending annotations directly to it) instead of
// operating on a clone. If that happened, repeated calls with different
// syncRoles values would leak annotations onto each other through the shared
// pointer, and worse, the package var itself would accumulate skip
// annotations as a side effect of unrelated calls.
func TestUserBuilder_ResourceType_DoesNotMutatePackageVar(t *testing.T) {
	if len(userResourceType.Annotations) != 0 {
		t.Fatalf("precondition failed: userResourceType already has annotations: %+v", userResourceType.Annotations)
	}

	trueBuilder := &userBuilder{syncRoles: true}
	falseBuilder := &userBuilder{syncRoles: false}

	for i := 0; i < 3; i++ {
		_ = trueBuilder.ResourceType(context.Background())
		_ = falseBuilder.ResourceType(context.Background())

		annos := annotations.Annotations(userResourceType.Annotations)
		if annos.Contains(&v2.SkipEntitlements{}) {
			t.Fatalf("iteration %d: package var userResourceType picked up SkipEntitlements as a side effect", i)
		}
		if annos.Contains(&v2.SkipEntitlementsAndGrants{}) {
			t.Fatalf("iteration %d: package var userResourceType picked up SkipEntitlementsAndGrants as a side effect", i)
		}
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
