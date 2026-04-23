# HTTP API Reference

This document describes all HTTP routes exposed by Invito, their parameters, and their expected responses.

Invito is not a JSON API — it returns HTML pages. HTMX partial responses are noted where applicable.

---

## Public Routes

These routes require no authentication.

---

### `GET /`

Landing page. Describes Invito and shows a "Sign in" call-to-action.

**Response:** `200 OK`, HTML page.

---

### `GET /calendar/{username}/`

Lists all active event types for the given user.

**Path parameters:**

- `username` — the user's public slug.

**Response:**

- `200 OK` — HTML page with event type cards.
- `404 Not Found` — no user with this username, or user has no active event types.

---

### `GET /calendar/{username}/{slug}`

Slot picker for a specific event type. Shows a date navigation defaulting to the current week.

**Path parameters:**

- `username` — user's public slug.
- `slug` — event type slug.

**Query parameters:**

- `date` (optional) — `YYYY-MM-DD`. If present, the slot list for this date is shown immediately (used by HTMX).

**Response:**

- `200 OK` — full HTML page (initial load) or HTML partial (HTMX request, identified by `HX-Request` header).
- `404 Not Found` — unknown username or slug.

**HTMX behavior:** When the user navigates to a date, the browser sends `GET /calendar/{username}/{slug}?date=YYYY-MM-DD` with `HX-Request: true`. Invito returns only the slot list fragment, which HTMX swaps into the page.

---

### `POST /calendar/{username}/{slug}/book`

Submits a booking request.

**Path parameters:**

- `username`, `slug` — as above.

**Form fields:**
| Field | Required | Description |
|-------|----------|-------------|
| `guest_name` | yes | Guest's full name |
| `guest_email` | yes | Guest's email address |
| `slot` | yes | ISO 8601 start time in UTC, e.g. `2026-05-15T09:00:00Z` |
| `guest_note` | no | Optional message to the host |
| `csrf_token` | yes | Double-submit CSRF token (set by cookie) |

**Response:**

- `200 OK` — HTML confirmation page ("Your request has been sent").
- `409 Conflict` — slot is no longer available (race condition or stale page).
- `422 Unprocessable Entity` — validation errors; form re-rendered with errors.

---

### `GET /booking/{token}/confirm`

Confirms a PENDING booking. Intended to be clicked from a notification email.

**Path parameters:**

- `token` — UUID token from the booking record.

**Response:**

- `200 OK` — HTML page: "Booking confirmed."
- `200 OK` — HTML page: "This booking has already been confirmed." (idempotent)
- `404 Not Found` — unknown token.
- `410 Gone` — booking was already rejected or cancelled.

---

### `GET /booking/{token}/reject`

Rejects a PENDING booking.

**Path parameters:**

- `token` — UUID token from the booking record.

**Response:**

- `200 OK` — HTML page: "Booking rejected."
- `200 OK` — HTML page: "This booking has already been handled." (idempotent)
- `404 Not Found` — unknown token.
- `410 Gone` — booking was already confirmed or cancelled.

---

## Auth Routes

---

### `GET /auth/login`

Initiates the OIDC authorization code flow. Stores a `state` parameter in a short-lived cookie and redirects to the OIDC provider's authorization endpoint.

**Response:** `302 Found` → OIDC provider.

---

### `GET /auth/callback`

OIDC callback. Validates the `state` parameter, exchanges the code for tokens, extracts claims, and creates or updates the user record. Creates a session cookie.

**Query parameters:**

- `code` — authorization code from the provider.
- `state` — must match the value stored in the login cookie.

**Response:**

- `302 Found` → `/dashboard` on success.
- `400 Bad Request` — state mismatch or missing parameters.
- `500 Internal Server Error` — token exchange or claim extraction failure.

---

### `POST /auth/logout`

Destroys the server-side session and clears the session cookie.

**Response:** `302 Found` → `/`.

---

## Dashboard Routes

All dashboard routes require an authenticated session. Unauthenticated requests are redirected to `/auth/login`.

---

### `GET /dashboard`

Overview page showing upcoming confirmed bookings and count of pending requests.

---

### `GET /dashboard/calendars`

Lists the user's connected CalDAV calendars with sync status.

---

### `POST /dashboard/calendars`

Adds a new CalDAV calendar.

**Form fields:**
| Field | Required | Description |
|-------|----------|-------------|
| `caldav_url` | yes | CalDAV collection URL |
| `username` | yes | CalDAV username |
| `password` | yes | CalDAV password |
| `display_name` | yes | Label shown in the UI |
| `color` | no | Hex color code, default `#6366f1` |

**Behavior:** Invito performs a PROPFIND to validate credentials before saving. Returns an error if the URL is unreachable or credentials are wrong.

---

### `DELETE /dashboard/calendars/{id}`

Removes a calendar and all its cached events.

**Response:** HTMX-friendly; returns an empty `200 OK` body so HTMX can remove the row from the DOM.

---

### `POST /dashboard/calendars/{id}/sync`

Triggers an immediate CalDAV sync for one calendar.

**Response:** `200 OK` with updated sync status partial (HTMX).

---

### `GET /dashboard/availability`

Shows the user's weekly availability rules as a grid.

---

### `POST /dashboard/availability`

Replaces all availability rules for the user. The form submits the entire weekly grid at once.

**Form fields:** Repeated blocks of `weekday`, `start_time`, `end_time` for each active window.

---

### `GET /dashboard/event-types`

Lists all event types with their status (active/inactive) and booking links.

---

### `POST /dashboard/event-types`

Creates a new event type.

**Form fields:**
| Field | Required | Description |
|-------|----------|-------------|
| `title` | yes | Display name |
| `slug` | yes | URL segment (lowercase, hyphens only) |
| `description` | no | Shown on the booking page |
| `duration_minutes` | yes | Fixed duration in minutes |
| `color` | no | Hex color |
| `booking_window_days` | no | Default 60 |

---

### `GET /dashboard/event-types/{id}/edit`

Edit form for an existing event type.

---

### `POST /dashboard/event-types/{id}`

Updates an event type. Same fields as create.

---

### `POST /dashboard/event-types/{id}/toggle`

Toggles the `active` flag.

---

### `GET /dashboard/bookings`

Lists all bookings for the user's event types.

**Query parameters:**

- `status` (optional) — filter by `PENDING`, `CONFIRMED`, `REJECTED`, `CANCELLED`. Default: all.

---

## Error Pages

| Status | Page                                                                        |
| ------ | --------------------------------------------------------------------------- |
| 404    | "Page not found" with link to home                                          |
| 500    | "Something went wrong" — error ID logged server-side, not exposed to client |
