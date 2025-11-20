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
  "bio": "building calm productivity",
  "birthdate": "1996-09-14"
}
```

**Response:**

Returns the updated profile object (same structure as `GET /v1/users/me`).

## Metadata Derivation

Metadata fields are computed on each request to guarantee freshness:

- **total_productivities / total_sessions**: number of non-deleted documents inside `users/{id}/productivities`.
- **longest_streak**: the longest run of consecutive days (Asia/Jakarta timezone) with at least one productivity entry. Duplicate entries within the same day are deduplicated to prevent streak inflation.

Firestore reads are scoped to only the necessary fields (`start_time`, `deleted`) to keep calls efficient even for large histories.

## Development

Run tests locally with:

```bash
go test ./...
```
