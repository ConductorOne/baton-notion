package notion

type Group struct {
	Schemas     []string `json:"schemas"`
	ID          string   `json:"id"`
	DisplayName string   `json:"displayName"`
	Members     []Member `json:"members"`
}

type Member struct {
	Value string `json:"value"`
	Ref   string `json:"$ref"`
	Type  string `json:"type"`
}

type User struct {
	Schemas  []string `json:"schemas"`
	UserName string   `json:"userName"`
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
	// Title  string `json:"title"`
	Active bool `json:"active"`
}
