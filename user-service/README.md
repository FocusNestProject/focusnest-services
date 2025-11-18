# FocusNest User Service

Manages authenticated profile data for the FocusNest ecosystem. Profiles are stored in Firestore under the `profiles/{userID}` document path and enriched at read time with derived productivity metadata.

## API surface

All routes require a valid access token handled by the shared auth middleware. The HTTP handler also expects the upstream gateway to inject the `X-User-ID` header for the authenticated subject.

### `GET /v1/users/me`

Returns the caller's profile and derived metadata. If a profile document has not been created yet, the service responds with default values (empty strings, `null` birthdate) while still calculating metadata counters based on the user's `productivities` sub-collection.

```jsonc
{
  "user_id": "abc123",
  "full_name": "Focus Nest",
  "username": "focusnest",
  "bio": "",
  "birthdate": "1996-09-14",
  "metadata": {
    "longest_streak": 12,
    "total_productivities": 48,
    "total_sessions": 48
  },
  "created_at": "2025-11-19T09:10:11Z",
  "updated_at": "2025-11-19T09:10:11Z"
}
```

### `PATCH /v1/users/me`

Allows partial profile updates. The payload may include any combination of `full_name`, `username`, `bio`, and `birthdate`. Fields are trimmed server-side; `birthdate` must be an ISO `YYYY-MM-DD` string or explicit `null` to clear the value. Requests larger than 64KB are rejected.

```json
{
  "full_name": "Focus Nest",
  "username": "focusnest",
  "bio": "building calm productivity",
  "birthdate": "1996-09-14"
}
```

The response mirrors the `GET` payload with the updated data and live metadata snapshot.

## Metadata derivation

Metadata fields are computed on each request to guarantee freshness:

- **total_productivities / total_sessions**: number of non-deleted documents inside `users/{id}/productivities`.
- **longest_streak**: the longest run of consecutive days (Asia/Jakarta timezone) with at least one productivity entry. Duplicate entries within the same day are deduplicated to prevent streak inflation.

Firestore reads are scoped to only the necessary fields (`start_time`, `deleted`) to keep calls efficient even for large histories.

## Development

Run tests locally with:

```bash
go test ./...
```
