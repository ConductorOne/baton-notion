// test-server is a mock of Notion's SCIM API used by the baton-notion CI and
// for local development.
//
// It implements the read endpoints documented at the Notion help center plus
// the Notion-specific role extension under
// urn:ietf:params:scim:schemas:extension:notion:2.0:User.
//
// IMPORTANT: this mock implements only the surface the connector currently
// uses (read, account create/delete, and PATCH on the workspace role
// extension). It is intentionally NOT a full SCIM 2.0 server. PATCH on
// arbitrary user attributes (name, email, etc.) is not supported because the
// connector doesn't drive that path — and Notion's domain-verification
// caveat applies to those updates anyway.
//
// Reference docs:
//
//   - Notion SCIM API (source of truth for endpoint behavior, the role
//     extension and the enum of valid role values):
//     https://www.notion.com/help/provision-users-and-groups-with-scim
//
//   - SCIM 2.0 Protocol — RFC 7644 (PATCH, ListResponse, Error envelope):
//     https://datatracker.ietf.org/doc/html/rfc7644
//   - SCIM 2.0 Core Schema — RFC 7643 (User, Group, ServiceProviderConfig,
//     ResourceTypes):
//     https://datatracker.ietf.org/doc/html/rfc7643
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/google/uuid"
)

const (
	defaultAddr = ":8765"

	// SCIM 2.0 schema URNs.
	schemaUserCore     = "urn:ietf:params:scim:schemas:core:2.0:User"
	schemaGroupCore    = "urn:ietf:params:scim:schemas:core:2.0:Group"
	schemaListResponse = "urn:ietf:params:scim:api:messages:2.0:ListResponse"
	schemaError        = "urn:ietf:params:scim:api:messages:2.0:Error"
	schemaNotionUser   = "urn:ietf:params:scim:schemas:extension:notion:2.0:User"

	// Notion workspace roles documented by the Notion SCIM extension.
	roleOwner            = "owner"
	roleMembershipAdmin  = "membership_admin"
	roleMember           = "member"
	roleRestrictedMember = "restricted_member"

	maxPerPage     = 100
	defaultPerPage = 100

	// Wire-format JSON field names extracted as constants to satisfy goconst
	// and keep response shapes uniform across handlers.
	jsonKeyResources    = "Resources"
	jsonKeySchema       = "schema"
	jsonKeySchemas      = "schemas"
	jsonKeyTotalResults = "totalResults"
	jsonKeyName         = "name"
	jsonKeyResourceType = "urn:ietf:params:scim:schemas:core:2.0:ResourceType"
	jsonKeySupported    = "supported"

	memberTypeUser = "User"
)

type scimName struct {
	GivenName  string `json:"givenName,omitempty"`
	FamilyName string `json:"familyName,omitempty"`
	Formatted  string `json:"formatted,omitempty"`
}

type scimEmail struct {
	Primary bool   `json:"primary"`
	Value   string `json:"value"`
	Type    string `json:"type,omitempty"`
}

type notionExtension struct {
	Role string `json:"role,omitempty"`
}

type scimUser struct {
	Schemas         []string         `json:"schemas"`
	ID              string           `json:"id"`
	UserName        string           `json:"userName"`
	Name            scimName         `json:"name"`
	Emails          []scimEmail      `json:"emails,omitempty"`
	Active          bool             `json:"active"`
	NotionExtension *notionExtension `json:"urn:ietf:params:scim:schemas:extension:notion:2.0:User,omitempty"`
}

// patchOperation is one SCIM 2.0 PATCH op. Value is decoded as RawMessage so
// we can dispatch on shape — the connector emits the value-as-object form
// (no `path`), but we accept the path form too as a defensive measure.
type patchOperation struct {
	Op    string          `json:"op"`
	Path  string          `json:"path,omitempty"`
	Value json.RawMessage `json:"value,omitempty"`
}

type patchRequest struct {
	Schemas    []string         `json:"schemas"`
	Operations []patchOperation `json:"Operations"`
}

type scimGroupMember struct {
	Value string `json:"value"`
	Ref   string `json:"$ref,omitempty"`
	Type  string `json:"type,omitempty"`
}

type scimGroup struct {
	Schemas     []string          `json:"schemas"`
	ID          string            `json:"id"`
	DisplayName string            `json:"displayName"`
	Members     []scimGroupMember `json:"members"`
}

// scimError mirrors the SCIM 2.0 Error response envelope (RFC 7644 §3.12).
type scimError struct {
	Schemas  []string `json:"schemas"`
	Status   string   `json:"status"`
	ScimType string   `json:"scimType,omitempty"`
	Detail   string   `json:"detail,omitempty"`
}

// store is the in-memory backing data the mock serves. All operations take
// the mutex to keep `go test -race` clean — sync.RWMutex is enough since
// CI traffic against this server is single-digit RPS.
type store struct {
	mu     sync.RWMutex
	users  map[string]*scimUser
	groups map[string]*scimGroup
}

func newStore() *store {
	s := &store{
		users:  make(map[string]*scimUser),
		groups: make(map[string]*scimGroup),
	}
	s.seed()
	return s
}

// seed populates the mock with deterministic data that covers every role the
// connector models, plus two groups with mixed membership. CI assertions
// depend on these specific counts and IDs, so keep them stable.
func (s *store) seed() {
	users := []*scimUser{
		newUser("11111111-1111-1111-1111-111111111111", "owner@example.com", "Alice", "Owner", roleOwner),
		newUser("22222222-2222-2222-2222-222222222222", "admin@example.com", "Bob", "Admin", roleMembershipAdmin),
		newUser("33333333-3333-3333-3333-333333333333", "carol.member@example.com", "Carol", "Member", roleMember),
		newUser("44444444-4444-4444-4444-444444444444", "dave.member@example.com", "Dave", "Member", roleMember),
		newUser("55555555-5555-5555-5555-555555555555", "eve.restricted@example.com", "Eve", "Restricted", roleRestrictedMember),
	}
	for _, u := range users {
		s.users[u.ID] = u
	}

	engineeringID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	designersID := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"

	s.groups[engineeringID] = newGroup(engineeringID, "Engineering",
		users[0].ID, users[2].ID, users[3].ID,
	)
	s.groups[designersID] = newGroup(designersID, "Designers",
		users[1].ID, users[4].ID,
	)
}

func newUser(id, email, given, family, role string) *scimUser {
	return &scimUser{
		Schemas:  []string{schemaUserCore, schemaNotionUser},
		ID:       id,
		UserName: email,
		Name: scimName{
			GivenName:  given,
			FamilyName: family,
			Formatted:  given + " " + family,
		},
		Emails:          []scimEmail{{Primary: true, Value: email, Type: "Primary"}},
		Active:          true,
		NotionExtension: &notionExtension{Role: role},
	}
}

func newGroup(id, displayName string, memberIDs ...string) *scimGroup {
	g := &scimGroup{
		Schemas:     []string{schemaGroupCore},
		ID:          id,
		DisplayName: displayName,
		Members:     make([]scimGroupMember, 0, len(memberIDs)),
	}
	for _, mid := range memberIDs {
		g.Members = append(g.Members, scimGroupMember{
			Value: mid,
			Type:  memberTypeUser,
			Ref:   fmt.Sprintf("https://www.notion.so/scim/v2/Users/%s", mid),
		})
	}
	return g
}

func main() {
	addr := flag.String("addr", defaultAddr, "listen address")
	flag.Parse()

	s := newStore()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /scim/v2/ServiceProviderConfig", handleServiceProviderConfig)
	mux.HandleFunc("GET /scim/v2/ResourceTypes", handleResourceTypes)
	mux.HandleFunc("GET /scim/v2/Users", s.handleListUsers)
	mux.HandleFunc("GET /scim/v2/Users/{id}", s.handleGetUser)
	mux.HandleFunc("POST /scim/v2/Users", s.handleCreateUser)
	mux.HandleFunc("PATCH /scim/v2/Users/{id}", s.handlePatchUser)
	mux.HandleFunc("DELETE /scim/v2/Users/{id}", s.handleDeleteUser)
	mux.HandleFunc("GET /scim/v2/Groups", s.handleListGroups)
	mux.HandleFunc("GET /scim/v2/Groups/{id}", s.handleGetGroup)

	handler := requestLogger(bearerAuth(mux))

	log.Printf("baton-notion mock SCIM server listening on %s", *addr)
	log.Printf("base URL for connector: http://localhost%s/scim/v2", *addr)
	//nolint:gosec // mock server, TLS intentionally not configured.
	if err := http.ListenAndServe(*addr, handler); err != nil {
		log.Printf("server exited: %v", err)
		os.Exit(1)
	}
}

// requestLogger is a tiny access-log middleware so CI logs make sense.
// URL is logged for debugging — log injection is out of scope for a mock server.
func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.RequestURI()) //nolint:gosec // mock server, log injection out of scope.
		next.ServeHTTP(w, r)
	})
}

// bearerAuth rejects requests without a non-empty Bearer token. The mock does
// not validate the token value (any non-empty string is accepted) — that lets
// CI use a literal "test-token" without secrets plumbing.
func bearerAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := r.Header.Get("Authorization")
		const prefix = "Bearer "
		if !strings.HasPrefix(h, prefix) || strings.TrimSpace(h[len(prefix):]) == "" {
			writeError(w, http.StatusUnauthorized, "", "missing or empty bearer token")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func handleServiceProviderConfig(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		jsonKeySchemas:   []string{"urn:ietf:params:scim:schemas:core:2.0:ServiceProviderConfig"},
		"patch":          map[string]bool{jsonKeySupported: true},
		"bulk":           map[string]bool{jsonKeySupported: false},
		"filter":         map[string]any{jsonKeySupported: true, "maxResults": maxPerPage},
		"changePassword": map[string]bool{jsonKeySupported: false},
		"sort":           map[string]bool{jsonKeySupported: false},
		"etag":           map[string]bool{jsonKeySupported: false},
		"authenticationSchemes": []map[string]string{
			{"type": "oauthbearertoken", jsonKeyName: "OAuth Bearer Token"},
		},
	})
}

func handleResourceTypes(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		jsonKeySchemas:      []string{schemaListResponse},
		jsonKeyTotalResults: 2,
		jsonKeyResources: []map[string]any{
			{
				jsonKeySchemas: []string{jsonKeyResourceType},
				"id":           memberTypeUser,
				jsonKeyName:    memberTypeUser,
				"endpoint":     "/Users",
				jsonKeySchema:  schemaUserCore,
				"schemaExtensions": []map[string]any{
					{jsonKeySchema: schemaNotionUser, "required": false},
				},
			},
			{
				jsonKeySchemas: []string{jsonKeyResourceType},
				"id":           "Group",
				jsonKeyName:    "Group",
				"endpoint":     "/Groups",
				jsonKeySchema:  schemaGroupCore,
			},
		},
	})
}

func (s *store) handleListUsers(w http.ResponseWriter, r *http.Request) {
	startIndex, perPage := paginationParams(r.URL.Query())
	filter := r.URL.Query().Get("filter")

	s.mu.RLock()
	defer s.mu.RUnlock()

	all := make([]*scimUser, 0, len(s.users))
	for _, u := range s.users {
		all = append(all, u)
	}
	sortUsersByID(all)

	filtered, err := applyUserFilter(all, filter)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalidFilter", err.Error())
		return
	}

	page := pageSlice(filtered, startIndex, perPage)

	writeJSON(w, http.StatusOK, map[string]any{
		jsonKeySchemas:      []string{schemaListResponse},
		jsonKeyTotalResults: len(filtered),
		"startIndex":        startIndex,
		"itemsPerPage":      len(page),
		jsonKeyResources:    page,
	})
}

func (s *store) handleGetUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	s.mu.RLock()
	defer s.mu.RUnlock()

	u, ok := s.users[id]
	if !ok {
		writeError(w, http.StatusNotFound, "", fmt.Sprintf("user %q not found", id))
		return
	}
	writeJSON(w, http.StatusOK, u)
}

// handleCreateUser supports the account-create flow exposed by the connector.
// Newly-created users default to `member` because that is the seat tier Notion
// assigns to a brand new workspace member created via POST /Users (mentioned
// in the "Provision restricted members through SCIM" section of the help doc).
func (s *store) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var incoming scimUser
	if err := json.NewDecoder(r.Body).Decode(&incoming); err != nil {
		writeError(w, http.StatusBadRequest, "invalidSyntax", "invalid JSON body: "+err.Error())
		return
	}
	if strings.TrimSpace(incoming.UserName) == "" {
		writeError(w, http.StatusBadRequest, "invalidValue", "userName is required")
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, existing := range s.users {
		if strings.EqualFold(existing.UserName, incoming.UserName) {
			writeError(w, http.StatusConflict, "uniqueness", "user with that userName already exists")
			return
		}
	}

	incoming.ID = uuid.NewString()
	incoming.Schemas = []string{schemaUserCore, schemaNotionUser}
	if incoming.NotionExtension == nil {
		incoming.NotionExtension = &notionExtension{Role: roleMember}
	}
	s.users[incoming.ID] = &incoming
	writeJSON(w, http.StatusCreated, &incoming)
}

// handlePatchUser applies SCIM 2.0 PATCH operations to the workspace role
// extension. Only the `role` attribute is supported — the connector does not
// drive other PATCH paths, and Notion's "verified domain" caveat applies to
// the other attributes anyway.
func (s *store) handlePatchUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req patchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalidSyntax", "invalid PATCH body: "+err.Error())
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	u, ok := s.users[id]
	if !ok {
		writeError(w, http.StatusNotFound, "", fmt.Sprintf("user %q not found", id))
		return
	}

	for _, op := range req.Operations {
		if err := applyPatchOp(u, op); err != nil {
			writeError(w, http.StatusBadRequest, "invalidValue", err.Error())
			return
		}
	}
	writeJSON(w, http.StatusOK, u)
}

func (s *store) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.users[id]; !ok {
		writeError(w, http.StatusNotFound, "", fmt.Sprintf("user %q not found", id))
		return
	}
	delete(s.users, id)
	for _, g := range s.groups {
		g.Members = filterMembers(g.Members, id)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *store) handleListGroups(w http.ResponseWriter, r *http.Request) {
	startIndex, perPage := paginationParams(r.URL.Query())
	filter := r.URL.Query().Get("filter")

	s.mu.RLock()
	defer s.mu.RUnlock()

	all := make([]*scimGroup, 0, len(s.groups))
	for _, g := range s.groups {
		all = append(all, g)
	}
	sortGroupsByID(all)

	filtered, err := applyGroupFilter(all, filter)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalidFilter", err.Error())
		return
	}

	page := pageSlice(filtered, startIndex, perPage)

	writeJSON(w, http.StatusOK, map[string]any{
		jsonKeySchemas:      []string{schemaListResponse},
		jsonKeyTotalResults: len(filtered),
		"startIndex":        startIndex,
		"itemsPerPage":      len(page),
		jsonKeyResources:    page,
	})
}

func (s *store) handleGetGroup(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	s.mu.RLock()
	defer s.mu.RUnlock()

	g, ok := s.groups[id]
	if !ok {
		writeError(w, http.StatusNotFound, "", fmt.Sprintf("group %q not found", id))
		return
	}
	writeJSON(w, http.StatusOK, g)
}

// paginationParams parses Notion-style SCIM pagination: 1-indexed startIndex,
// count clamped at 100 (per the help doc).
func paginationParams(q url.Values) (int, int) {
	startIndex := 1
	if v := q.Get("startIndex"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 1 {
			startIndex = n
		}
	}

	perPage := defaultPerPage
	if v := q.Get("count"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			perPage = n
		}
	}
	if perPage > maxPerPage {
		perPage = maxPerPage
	}
	return startIndex, perPage
}

func pageSlice[T any](items []T, startIndex, perPage int) []T {
	from := startIndex - 1
	if from < 0 || from >= len(items) {
		return []T{}
	}
	end := from + perPage
	if end > len(items) {
		end = len(items)
	}
	return items[from:end]
}

// applyUserFilter implements the subset of SCIM filters the Notion help doc
// lists for /Users: `email eq "x"`, `given_name eq "x"`, `family_name eq "x"`.
// Email comparison is case-insensitive to match the doc's note that email is
// "converted to lowercase" before matching.
func applyUserFilter(users []*scimUser, filter string) ([]*scimUser, error) {
	if filter == "" {
		return users, nil
	}
	attr, op, value, err := parseSimpleFilter(filter)
	if err != nil {
		return nil, err
	}
	if op != "eq" {
		return nil, fmt.Errorf("only `eq` operator is supported, got %q", op)
	}
	out := make([]*scimUser, 0, len(users))
	for _, u := range users {
		switch attr {
		case "email", "username", "userName":
			if strings.EqualFold(u.UserName, value) {
				out = append(out, u)
			}
		case "given_name":
			if u.Name.GivenName == value {
				out = append(out, u)
			}
		case "family_name":
			if u.Name.FamilyName == value {
				out = append(out, u)
			}
		default:
			return nil, fmt.Errorf("unsupported user filter attribute %q", attr)
		}
	}
	return out, nil
}

// applyGroupFilter implements `displayName eq "x"` (the only documented group filter).
func applyGroupFilter(groups []*scimGroup, filter string) ([]*scimGroup, error) {
	if filter == "" {
		return groups, nil
	}
	attr, op, value, err := parseSimpleFilter(filter)
	if err != nil {
		return nil, err
	}
	if op != "eq" {
		return nil, fmt.Errorf("only `eq` operator is supported, got %q", op)
	}
	if attr != "displayName" {
		return nil, fmt.Errorf("unsupported group filter attribute %q", attr)
	}
	out := make([]*scimGroup, 0, len(groups))
	for _, g := range groups {
		if g.DisplayName == value {
			out = append(out, g)
		}
	}
	return out, nil
}

// parseSimpleFilter parses `attr op "value"` (quotes optional, trimmed).
func parseSimpleFilter(filter string) (string, string, string, error) {
	parts := strings.SplitN(strings.TrimSpace(filter), " ", 3)
	if len(parts) != 3 {
		return "", "", "", fmt.Errorf("filter must be `attr op value`, got %q", filter)
	}
	value := strings.Trim(parts[2], `"`)
	return parts[0], parts[1], value, nil
}

// applyPatchOp mutates the user in place per a single SCIM PATCH operation.
// Supports the two payload shapes the connector might emit and that stricter
// IdPs commonly send:
//
//  1. {"op":"replace","value":{"urn:...notion...":{"role":"member"}}}  (no path)
//  2. {"op":"replace","path":"urn:...notion...:role","value":"member"} (path form)
//
// Only the workspace role extension attribute is supported. Other
// attributes return invalidValue — the connector should never PATCH them.
func applyPatchOp(u *scimUser, op patchOperation) error {
	switch strings.ToLower(op.Op) {
	case "replace", "add":
	default:
		return fmt.Errorf("unsupported PATCH op %q (only `replace` / `add` are supported)", op.Op)
	}

	// Shape 2: path-style. Accept the extension attribute path with `:`.
	if op.Path != "" {
		switch op.Path {
		case schemaNotionUser + ":role", "role":
			var role string
			if err := json.Unmarshal(op.Value, &role); err != nil {
				return fmt.Errorf("invalid role value: %w", err)
			}
			return setRole(u, role)
		default:
			return fmt.Errorf("unsupported PATCH path %q (only the role extension is supported)", op.Path)
		}
	}

	// Shape 1: value-as-object form. Walk the namespaces and dispatch.
	var asObject map[string]json.RawMessage
	if err := json.Unmarshal(op.Value, &asObject); err != nil {
		return fmt.Errorf("PATCH op without `path` requires an object value: %w", err)
	}
	for key, raw := range asObject {
		if key != schemaNotionUser {
			return fmt.Errorf("unsupported PATCH namespace %q (only the Notion extension is supported)", key)
		}
		var ext notionExtension
		if err := json.Unmarshal(raw, &ext); err != nil {
			return fmt.Errorf("invalid notion extension value: %w", err)
		}
		if err := setRole(u, ext.Role); err != nil {
			return err
		}
	}
	return nil
}

func setRole(u *scimUser, role string) error {
	if !isKnownRole(role) {
		return fmt.Errorf("unknown role %q (must be owner, membership_admin, member, or restricted_member)", role)
	}
	if u.NotionExtension == nil {
		u.NotionExtension = &notionExtension{}
	}
	u.NotionExtension.Role = role
	return nil
}

func isKnownRole(role string) bool {
	switch role {
	case roleOwner, roleMembershipAdmin, roleMember, roleRestrictedMember:
		return true
	}
	return false
}

func filterMembers(members []scimGroupMember, removeID string) []scimGroupMember {
	out := make([]scimGroupMember, 0, len(members))
	for _, m := range members {
		if m.Value != removeID {
			out = append(out, m)
		}
	}
	return out
}

func sortUsersByID(users []*scimUser) {
	stableSort(users, func(a, b *scimUser) bool { return a.ID < b.ID })
}

func sortGroupsByID(groups []*scimGroup) {
	stableSort(groups, func(a, b *scimGroup) bool { return a.ID < b.ID })
}

// stableSort uses insertion sort — N here is at most low double digits (seed
// + a few created in CI), so an N² sort is fine and avoids pulling sort just
// for a tiny dataset.
func stableSort[T any](items []T, less func(a, b T) bool) {
	for i := 1; i < len(items); i++ {
		for j := i; j > 0 && less(items[j], items[j-1]); j-- {
			items[j], items[j-1] = items[j-1], items[j]
		}
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/scim+json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("failed to encode response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, scimType, detail string) {
	writeJSON(w, status, scimError{
		Schemas:  []string{schemaError},
		Status:   strconv.Itoa(status),
		ScimType: scimType,
		Detail:   detail,
	})
}
