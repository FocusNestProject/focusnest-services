# FocusNest User Service

Manages authenticated profile data for the FocusNest ecosystem. Profiles are stored in Firestore under the `profiles/{userID}` document path and enriched at read time with derived productivity metadata.

## API Surface

All routes require a valid access token handled by the shared auth middleware. The HTTP handler also expects the upstream gateway to inject the `X-User-ID` header for the authenticated subject.

### `GET /v1/users/me`

Returns the caller's profile and derived metadata. If a profile document has not been created yet, the service responds with default values (empty strings, `null` birthdate) while still calculating metadata counters based on the user's `productivities` sub-collection.

**Response Body:**

```jsonc
{
  "user_id": "string",
  "bio": "string",
  "birthdate": "string (ISO 8601) | null",
  "longest_streak": "integer",
  "total_productivities": "integer",
  "total_sessions": "integer",
  "created_at": "string (ISO 8601)",
  "updated_at": "string (ISO 8601)"
}
```

### `PATCH /v1/users/me`

Allows partial profile updates. The payload may include any combination of `bio` and `birthdate`. Display name and username remain the source of truth inside Clerk, so this service intentionally ignores those fields.

**Request Body:**

| Field       | Type               | Description                                                |
| :---------- | :----------------- | :--------------------------------------------------------- |
| `bio`       | `string`           | Optional. A short biography.                               |
| `birthdate` | `string` \| `null` | Optional. Date in `YYYY-MM-DD` format, or `null` to clear. |

**Example Request:**

```json
{
  # Documentation moved

  Service-level docs now live in the centralized root `README.md`. Please consult the root handbook for configuration, API reference, and troubleshooting details for the user-service.
```
