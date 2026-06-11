// Types mirroring the Notion SCIM 2.0 wire format consumed by client.go.
// Reference: https://www.notion.com/help/provision-users-and-groups-with-scim

package client

// SCIMPatchOperation uses the value-as-object form (no `path` field) to update
// extension attributes — the conservative SCIM 2.0 spelling that works against
// implementations that don't accept extension paths with `:` separators.
type SCIMPatchOperation struct {
	Op    string `json:"op"`
	Value any    `json:"value"`
}

type SCIMPatchRequest struct {
	Schemas    []string             `json:"schemas"`
	Operations []SCIMPatchOperation `json:"Operations"`
}

type NotionUserExtension struct {
	Role string `json:"role,omitempty"`
}

type Group struct {
	Schemas     []string `json:"schemas"`
	ID          string   `json:"id"`
	DisplayName string   `json:"displayName"`
	Members     []Member `json:"members"`
}

type GroupsResponse struct {
	TotalResults int64   `json:"totalResults"`
	Resources    []Group `json:"Resources"`
	StartIndex   int64   `json:"startIndex"`
	ItemsPerPage int64   `json:"itemsPerPage"`
}

type Member struct {
	Value string `json:"value"`
	Ref   string `json:"$ref"`
	Type  string `json:"type"`
}

type UsersResponse struct {
	TotalResults int64  `json:"totalResults"`
	Resources    []User `json:"Resources"`
	StartIndex   int64  `json:"startIndex"`
	ItemsPerPage int64  `json:"itemsPerPage"`
}

type User struct {
	ID       string   `json:"id"`
	Schemas  []string `json:"schemas"`
	UserName string   `json:"userName"` // Username corresponds to the email of the account.
	Name     struct {
		GivenName  string `json:"givenName"`
		FamilyName string `json:"familyName"`
		Formatted  string `json:"formatted"`
	} `json:"name"`
	Emails []struct {
		Primary bool   `json:"primary"`
		Value   string `json:"value"`
		Type    string `json:"type"`
	} `json:"emails"`
	Active          bool                 `json:"active"`
	NotionExtension *NotionUserExtension `json:"urn:ietf:params:scim:schemas:extension:notion:2.0:User,omitempty"`
}

func (u User) Role() string {
	if u.NotionExtension == nil {
		return ""
	}
	return u.NotionExtension.Role
}
