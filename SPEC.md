# Invito — Technical Specification

**Version:** 0.1 (pre-implementation)
**Last updated:** 2026-04-23

---

## 1. Product Vision

Invito is a lightweight, self-hosted scheduling tool for individuals and small teams who want Calendly-like functionality without sending their calendar data to a third party.

**Target users:** Freelancers, consultants, small-team operators, privacy-conscious individuals who already self-host infrastructure.

**Core promise:**

- One binary, one SQLite file, one config.
- Reads your existing CalDAV calendars — no second source of truth for your schedule.
- Guests book; you confirm; both parties get an email. Nothing else.

**What Invito is not:**

- A full calendar application (no event creation beyond bookings).
- A team scheduling tool (no round-robin, no collective availability).
- A payment or CRM platform.

---

## 2. User Stories

### User

| ID   | Story                                                                                                                                  |
| ---- | -------------------------------------------------------------------------------------------------------------------------------------- |
| U-01 | As a user I want to connect my private and work calendars via CalDAV so that Invito knows which time slots are available.              |
| U-02 | As a user I want to publish my calendar so that people I give the link to can independently book appointments in my free slots.        |
| U-03 | As a user I want to configure which time windows are available for booking, how long appointments last, and what their purpose is.     |
| U-04 | As a user I want to provide guests with both pre-filled links (event type fixed) and generic links (guest chooses the event type).     |
| U-05 | As a user I want each event type to have a fixed duration.                                                                             |
| U-06 | As a user I want to receive an email when a guest requests an appointment. I want the slot to be reserved for 24 hours while I decide. |
| U-07 | As a user I want to accept or reject booking requests by email.                                                                        |

### Administrator

| ID   | Story                                                                                                                  |
| ---- | ---------------------------------------------------------------------------------------------------------------------- |
| A-01 | As an administrator I want users to authenticate via OIDC so that I don't have to run a separate login infrastructure. |
| A-02 | As an administrator I want Invito to be written in Go so that I don't have to worry about many runtime dependencies.   |

### Product Owner

| ID   | Story                                                                                                           |
| ---- | --------------------------------------------------------------------------------------------------------------- |
| P-01 | As a product owner I want the Invito landing page to concisely describe the product and invite users to log in. |
| P-02 | As a product owner I want Invito to be an open source product.                                                  |
| P-03 | As a product owner I want Invito's documentation to follow the Diátaxis framework.                              |

---

## 3. System Architecture

### Technology Stack

| Layer         | Choice                          | Rationale                                          |
| ------------- | ------------------------------- | -------------------------------------------------- |
| Language      | Go 1.22+                        | Single binary, low operational overhead (A-02)     |
| Database      | SQLite via `modernc.org/sqlite` | No cgo required, embedded, single file             |
| Templates     | `html/template` (Go stdlib)     | Zero JS framework dependency                       |
| Interactivity | HTMX                            | Minimal JS, progressive enhancement, no build step |
| Auth          | OIDC (coreos/go-oidc)           | Provider-agnostic, no user DB required (A-01)      |
| CalDAV        | `github.com/emersion/go-webdav` | Mature Go CalDAV client                            |
| Email         | SMTP via `net/smtp` (stdlib)    | No vendor lock-in                                  |

### Component Diagram

```
┌─────────────────────────────────────────────────────┐
│                    Invito Process                   │
│                                                     │
│  ┌──────────┐  ┌──────────┐  ┌───────────────────┐ │
│  │ HTTP     │  │ Scheduler│  │ Background Jobs   │ │
│  │ Server   │  │ (ticker) │  │ - CalDAV sync     │ │
│  │          │  │          │  │ - Booking TTL GC  │ │
│  └────┬─────┘  └────┬─────┘  └────────┬──────────┘ │
│       │              │                 │             │
│  ┌────▼─────────────▼─────────────────▼──────────┐  │
│  │                 Service Layer                  │  │
│  │  BookingService  CalendarService  AuthService  │  │
│  └────────────────────┬───────────────────────────┘  │
│                       │                              │
│  ┌────────────────────▼───────────────────────────┐  │
│  │              SQLite (single file)              │  │
│  └────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────┘
         │                    │              │
    OIDC Provider         CalDAV Server   SMTP Server
```

### Multi-User Model

Each OIDC-authenticated account becomes a user. Each user has an independent profile with their own calendars, event types, and availability rules. Public booking URLs are scoped to the user:

```
/{username}/                    → lists the user's active event types
/{username}/{event-type-slug}   → slot picker for a specific event type
```

`username` is derived from the OIDC `preferred_username` claim, lowercased and slugified at first login.

### Deployment

- **Binary**: Single statically-linked executable (CGO disabled via `modernc.org/sqlite`).
- **Docker**: Official image with non-root user, volume at `/data`.
- **Config**: 100% via environment variables (see §6).
- **No migrations framework**: Schema is applied via embedded SQL on startup; additive changes only.

---

## 4. Data Model

See [docs/explanation/data-model.md](docs/explanation/data-model.md) for the full entity-relationship description. SQL DDL follows:

### users

```sql
CREATE TABLE users (
    id          INTEGER PRIMARY KEY,
    oidc_sub    TEXT NOT NULL UNIQUE,
    email       TEXT NOT NULL,
    name        TEXT NOT NULL,
    username    TEXT NOT NULL UNIQUE,  -- URL-safe slug, derived from preferred_username
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

### calendars

```sql
CREATE TABLE calendars (
    id           INTEGER PRIMARY KEY,
    user_id      INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    caldav_url   TEXT NOT NULL,
    username     TEXT NOT NULL,
    password     TEXT NOT NULL,         -- stored encrypted at rest
    display_name TEXT NOT NULL,
    color        TEXT NOT NULL DEFAULT '#6366f1',
    sync_enabled INTEGER NOT NULL DEFAULT 1,
    last_synced  DATETIME,
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

### calendar_events

```sql
CREATE TABLE calendar_events (
    id          INTEGER PRIMARY KEY,
    calendar_id INTEGER NOT NULL REFERENCES calendars(id) ON DELETE CASCADE,
    uid         TEXT NOT NULL,
    start_at    DATETIME NOT NULL,
    end_at      DATETIME NOT NULL,
    summary     TEXT,
    synced_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(calendar_id, uid)
);
```

### event_types

```sql
CREATE TABLE event_types (
    id                  INTEGER PRIMARY KEY,
    user_id             INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    slug                TEXT NOT NULL,
    title               TEXT NOT NULL,
    description         TEXT,
    duration_minutes    INTEGER NOT NULL,
    color               TEXT NOT NULL DEFAULT '#6366f1',
    booking_window_days INTEGER NOT NULL DEFAULT 60,
    active              INTEGER NOT NULL DEFAULT 1,
    created_at          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, slug)
);
```

### availability_rules

```sql
CREATE TABLE availability_rules (
    id          INTEGER PRIMARY KEY,
    user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    weekday     INTEGER NOT NULL CHECK(weekday BETWEEN 0 AND 6),  -- 0=Sunday
    start_time  TEXT NOT NULL,  -- HH:MM, local time
    end_time    TEXT NOT NULL,  -- HH:MM, local time
    active      INTEGER NOT NULL DEFAULT 1
);
```

### bookings

```sql
CREATE TABLE bookings (
    id              INTEGER PRIMARY KEY,
    event_type_id   INTEGER NOT NULL REFERENCES event_types(id),
    guest_name      TEXT NOT NULL,
    guest_email     TEXT NOT NULL,
    guest_note      TEXT,
    start_at        DATETIME NOT NULL,
    end_at          DATETIME NOT NULL,
    status          TEXT NOT NULL DEFAULT 'PENDING'
                        CHECK(status IN ('PENDING','CONFIRMED','REJECTED','CANCELLED')),
    token           TEXT NOT NULL UNIQUE,   -- UUID v4, used in email confirm/reject links
    reserved_until  DATETIME NOT NULL,      -- start_at + booking_ttl
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

---

## 5. Feature Specifications

### 5.1 CalDAV Integration

**Adding a calendar:**

- User provides: CalDAV URL, username, password, display name.
- Invito performs a PROPFIND to verify credentials before saving.
- Password is stored AES-256-GCM encrypted using the session secret as key material.

**Sync:**

- Background goroutine runs on a configurable interval (default: 15 min).
- VEVENT components within the booking window are fetched and upserted into `calendar_events`.
- Events outside `now – 1h` to `now + booking_window_days` are ignored.

**Free slot calculation:**

1. For a given user, date, and event type: enumerate candidate slots at 30-min boundaries within `availability_rules`.
2. Remove slots that overlap any `calendar_events` row (padding: 0 min — no buffer by default).
3. Remove slots where an existing `PENDING` or `CONFIRMED` booking exists.
4. Return remaining slots in the user's local timezone.

**Write-back on confirmation:**

- A VCALENDAR/VEVENT is created via CalDAV PUT on the user's primary calendar (first synced calendar, or user-configurable).
- SUMMARY: `{EventType.title} with {guest_name}`
- DESCRIPTION: guest note if provided.

### 5.2 Public Booking Pages

| Route                                    | Behavior                                                |
| ---------------------------------------- | ------------------------------------------------------- |
| `GET /{username}/`                       | Lists all active EventTypes. No auth required.          |
| `GET /{username}/{slug}`                 | Shows a date picker. Default: current week.             |
| `GET /{username}/{slug}?date=YYYY-MM-DD` | Shows time slots for the given date (HTMX target).      |
| `POST /{username}/{slug}/book`           | Creates a Booking (PENDING). Returns confirmation page. |

The booking form collects: guest name, guest email, optional note. CSRF protection via double-submit cookie.

### 5.3 Booking Flow

See [docs/explanation/booking-flow.md](docs/explanation/booking-flow.md) for the full state diagram.

```
Guest submits form
       │
       ▼
  [PENDING] ──── 24h TTL expires ────► [CANCELLED] ──► Email to guest
       │
       ├── User clicks "Confirm" link in email
       │          │
       │          ▼
       │     [CONFIRMED] ──► CalDAV write-back
       │                ──► Confirmation email to user + guest
       │
       └── User clicks "Reject" link in email
                  │
                  ▼
             [REJECTED] ──► Rejection email to guest
```

Confirm/reject links contain the booking `token` and are single-use (status is checked before applying).

### 5.4 Email Notifications

All emails are sent as MIME multipart (text/plain + text/html).

| Trigger                   | Recipient    | Subject                                             |
| ------------------------- | ------------ | --------------------------------------------------- |
| Booking created (PENDING) | User         | `New booking request: {title} with {guest_name}`    |
| Booking confirmed         | User + Guest | `Confirmed: {title} on {date}`                      |
| Booking rejected          | Guest        | `Your booking request for {title} was not accepted` |
| Booking cancelled (TTL)   | Guest        | `Your booking request for {title} has expired`      |

The pending notification includes two tokenized links:

- `{BASE_URL}/booking/{token}/confirm`
- `{BASE_URL}/booking/{token}/reject`

### 5.5 OIDC Authentication

- Discovery via `{issuer}/.well-known/openid-configuration`.
- Scopes requested: `openid email profile`.
- Claims used: `sub` (identity), `email`, `name`, `preferred_username` (→ username slug).
- Session stored server-side in SQLite; cookie holds a random session ID (HttpOnly, Secure, SameSite=Lax).
- Session lifetime: 24 hours, refreshed on activity.
- On first login: user row is created. On subsequent logins: email and name are updated from claims.

### 5.6 Dashboard

All routes under `/dashboard` require an authenticated session.

| Page                      | Purpose                                                       |
| ------------------------- | ------------------------------------------------------------- |
| `/dashboard`              | Overview: upcoming confirmed bookings, pending requests count |
| `/dashboard/calendars`    | List, add, remove CalDAV calendars; trigger manual sync       |
| `/dashboard/availability` | Set weekly availability windows                               |
| `/dashboard/event-types`  | Create, edit, toggle event types                              |
| `/dashboard/bookings`     | Full booking history with status filter                       |

---

## 6. Configuration Reference

All configuration is via environment variables. See [docs/reference/configuration.md](docs/reference/configuration.md) for full descriptions and defaults.

| Variable                    | Required | Default       | Description                                             |
| --------------------------- | -------- | ------------- | ------------------------------------------------------- |
| `INVITO_BASE_URL`           | yes      | —             | Public base URL (no trailing slash)                     |
| `INVITO_DB_PATH`            | no       | `./invito.db` | Path to SQLite file                                     |
| `INVITO_SESSION_SECRET`     | yes      | —             | 32-byte hex string for cookie + encryption key          |
| `INVITO_OIDC_ISSUER`        | yes      | —             | OIDC issuer URL                                         |
| `INVITO_OIDC_CLIENT_ID`     | yes      | —             | OIDC client ID                                          |
| `INVITO_OIDC_CLIENT_SECRET` | yes      | —             | OIDC client secret                                      |
| `INVITO_SMTP_HOST`          | yes      | —             | SMTP hostname                                           |
| `INVITO_SMTP_PORT`          | no       | `587`         | SMTP port                                               |
| `INVITO_SMTP_USER`          | yes      | —             | SMTP username                                           |
| `INVITO_SMTP_PASSWORD`      | yes      | —             | SMTP password                                           |
| `INVITO_SMTP_FROM`          | yes      | —             | From address for outgoing mail                          |
| `INVITO_SYNC_INTERVAL`      | no       | `15m`         | CalDAV sync interval (Go duration string)               |
| `INVITO_BOOKING_TTL`        | no       | `24h`         | How long PENDING bookings are held (Go duration string) |
| `INVITO_LISTEN_ADDR`        | no       | `:8080`       | TCP address to listen on                                |

---

## 7. HTTP Routes

See [docs/reference/api.md](docs/reference/api.md) for detailed request/response documentation.

### Public

| Method | Path                                 | Description                           |
| ------ | ------------------------------------ | ------------------------------------- |
| GET    | `/`                                  | Landing page                          |
| GET    | `/{username}/`                       | User's booking page — event type list |
| GET    | `/{username}/{slug}`                 | Event type booking page — slot picker |
| GET    | `/{username}/{slug}?date=YYYY-MM-DD` | HTMX partial: slot list for a date    |
| POST   | `/{username}/{slug}/book`            | Submit booking request                |
| GET    | `/booking/{token}/confirm`           | Confirm a pending booking             |
| GET    | `/booking/{token}/reject`            | Reject a pending booking              |

### Auth

| Method | Path             | Description                        |
| ------ | ---------------- | ---------------------------------- |
| GET    | `/auth/login`    | Redirect to OIDC provider          |
| GET    | `/auth/callback` | OIDC callback, creates session     |
| POST   | `/auth/logout`   | Destroys session, redirects to `/` |

### Dashboard (requires auth)

| Method | Path                                 | Description             |
| ------ | ------------------------------------ | ----------------------- |
| GET    | `/dashboard`                         | Overview                |
| GET    | `/dashboard/calendars`               | Calendar list           |
| POST   | `/dashboard/calendars`               | Add calendar            |
| DELETE | `/dashboard/calendars/{id}`          | Remove calendar         |
| POST   | `/dashboard/calendars/{id}/sync`     | Trigger manual sync     |
| GET    | `/dashboard/availability`            | Availability rules      |
| POST   | `/dashboard/availability`            | Save availability rules |
| GET    | `/dashboard/event-types`             | Event type list         |
| POST   | `/dashboard/event-types`             | Create event type       |
| GET    | `/dashboard/event-types/{id}/edit`   | Edit form               |
| POST   | `/dashboard/event-types/{id}`        | Update event type       |
| POST   | `/dashboard/event-types/{id}/toggle` | Toggle active state     |
| GET    | `/dashboard/bookings`                | Booking history         |

---

## 8. Open Source

- **License:** MIT
- **Repository:** GitHub (public)
- **Versioning:** Semantic Versioning (`MAJOR.MINOR.PATCH`)
- **Releases:** GitHub Releases with pre-built binaries (linux/amd64, linux/arm64, darwin/arm64) and Docker image
- **Documentation:** Diátaxis framework (tutorials, how-to guides, explanation, reference)
- **Contributing:** See `CONTRIBUTING.md`

---

## Appendix: Glossary

| Term           | Definition                                                                                        |
| -------------- | ------------------------------------------------------------------------------------------------- |
| **Event Type** | A named, fixed-duration meeting kind created by the user (e.g. "30-min intro call").              |
| **Booking**    | A guest's request to meet at a specific slot for a specific event type.                           |
| **Slot**       | A candidate time window derived from availability rules minus existing calendar events.           |
| **CalDAV**     | Calendar extension of WebDAV; the protocol used to read/write calendar data.                      |
| **OIDC**       | OpenID Connect; the identity protocol used for user login.                                        |
| **TTL**        | Time-to-live; the 24-hour window during which a PENDING booking is held before auto-cancellation. |
