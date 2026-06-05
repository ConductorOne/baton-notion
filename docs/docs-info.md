## Connector capabilities

1. What resources does the connector sync?

   **Users** — `GET /scim/v2/Users` with pagination via `startIndex` (1-indexed) and `count` (≤100). Each user resource carries the SCIM core attributes; the Notion-specific `urn:ietf:params:scim:schemas:extension:notion:2.0:User.role` value is stashed on the user profile during list so role grants can be emitted from `userBuilder.Grants` without an extra API call (keeps sync at O(users) instead of O(users × roles)).

   **Groups** — `GET /scim/v2/Groups`, same `startIndex` / `count` pagination. Each group exposes a single `member` entitlement; membership is read from the group's `members` array.

   **Roles** — Static set of four resources (`owner`, `membership_admin`, `member`, `restricted_member`) matching the enum on the Notion SCIM role extension. The role resource declares both `TRAIT_ROLE` and `TRAIT_LICENSE_PROFILE` (Adobe-style hybrid — every paid Notion role consumes one workspace seat, so each role doubles as a license tier). The license trait wires the role's display name plus the `assigned` entitlement ID so the ConductorOne App Utilization feature can correlate seat-holders to user grants. Seat counts are intentionally omitted — Notion's SCIM API does not expose a per-role purchased/consumed endpoint.

2. Can the connector provision any resources? If so, which ones?

   **Accounts** — `POST /scim/v2/Users` (the mock and Notion both default the user to the `member` role on creation) and `DELETE /scim/v2/Users/{id}` (removes the user from the workspace and revokes active sessions). Notion does not permanently delete the underlying user account — that step must be performed manually in Notion.

   **Roles** — Grant and Revoke via [`PATCH /scim/v2/Users/{id}`](https://www.notion.com/help/provision-users-and-groups-with-scim) replacing the `role` attribute on the [Notion SCIM extension](https://www.notion.com/help/provision-users-and-groups-with-scim) (`urn:ietf:params:scim:schemas:extension:notion:2.0:User`). The PATCH body follows [SCIM 2.0 RFC 7644 §3.5.2](https://datatracker.ietf.org/doc/html/rfc7644#section-3.5.2).

   Both operations are idempotent via a single pre-flight [`GET /scim/v2/Users/{id}`](https://www.notion.com/help/provision-users-and-groups-with-scim):

   - **Grant** — if the user already holds the target role, return `GrantAlreadyExists` and skip the PATCH.
   - **Revoke** — Notion users must always carry a role, so revoke is modelled as a downgrade to `restricted_member` (the floor tier). Three short-circuits return `GrantAlreadyRevoked` without issuing a PATCH: (a) the pre-flight `GET` returns 404 (user deleted between sync and revoke), (b) the user no longer holds the role being revoked, (c) the role being revoked is `restricted_member` itself — there is nothing to downgrade to; the account must be deprovisioned to fully remove the user from the workspace.

## Connector credentials

1. What credentials or information are needed to set up the connector? (For example, API key, client ID and secret, domain, etc.)
- An API Token for the SCIM API should be provided.

2. For each item in the list above:

   * How does a user create or look up that credential or info? Please include links to (non-gated) documentation, screenshots (of the UI or of gated docs), or a video of the process.
    - In order to generate an API Token for the SCIM API, a Organization Owner of an Enterprise Notion account should go to the settings panel.
    - On the settings search for the 'Identity' tab on the left-panel and scroll down to the "SCIM Provisioning" section.
    - In the "SCIM Provisioning" section, users should be able to create a Token to use the SCIM API.

   * Does the credential need any specific scopes or permissions? If so, list them here.
    note: this isn't specified on the Notion docs and we didn't have the chance to test it 'cause we don't have an enterprise instance.

   * If applicable: Is the list of scopes or permissions different to sync (read) versus provision (read-write)? If so, list the difference here.
    note: this isn't specified on the Notion docs and we didn't have the chance to test it 'cause we don't have an enterprise instance.

   * What level of access or permissions does the user need in order to create the credentials? (For example, must be a super administrator, must have access to the admin console, etc.)
   - It should be an Organization Owner.