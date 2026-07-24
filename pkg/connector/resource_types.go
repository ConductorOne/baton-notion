package connector

import (
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
)

var (
	// userResourceType carries no static annotations: userBuilder.ResourceType
	// computes SkipEntitlements or SkipEntitlementsAndGrants dynamically based
	// on whether "role" is included in the sync (see user.go).
	userResourceType = &v2.ResourceType{
		Id:          "user",
		DisplayName: "User",
		Traits:      []v2.ResourceType_Trait{v2.ResourceType_TRAIT_USER},
	}

	groupResourceType = &v2.ResourceType{
		Id:          "group",
		DisplayName: "Group",
		Traits:      []v2.ResourceType_Trait{v2.ResourceType_TRAIT_GROUP},
	}

	roleResourceType = &v2.ResourceType{
		Id:          "role",
		DisplayName: "Role",
		Traits: []v2.ResourceType_Trait{
			v2.ResourceType_TRAIT_ROLE,
			v2.ResourceType_TRAIT_LICENSE_PROFILE,
		},
		Annotations: annotations.New(&v2.SkipGrants{}),
	}
)
