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
