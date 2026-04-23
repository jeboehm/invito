# Data Model

This document explains Invito's entities, their relationships, and the reasoning behind key design decisions.

## Entity Overview

```
users
 ├── calendars
 │    └── calendar_events
 ├── availability_rules
 └── event_types
      └── bookings
```

## users

A user is created on first OIDC login. The `oidc_sub` (subject claim) is the stable identifier from the OIDC provider — it does not change if the user's email or name changes.

`username` is a URL-safe slug derived from the OIDC `preferred_username` claim at first login. It is used in public booking URLs and is immutable after creation to avoid breaking shared links.

## calendars

A calendar represents one CalDAV collection. A user may connect multiple calendars (e.g. a personal Nextcloud calendar and a work Exchange calendar via a DAV bridge).

All connected calendars contribute to free/busy calculation. A slot is considered free only if no event in _any_ of the user's synced calendars overlaps it.

`password` is stored AES-256-GCM encrypted. The encryption key is derived from `INVITO_SESSION_SECRET`. Losing the session secret means losing the ability to decrypt CalDAV credentials — re-connection will be required.

## calendar_events

A local cache of VEVENT components fetched from CalDAV. This table is the source of truth for free/busy calculation. It is not the authoritative event store — that remains on the CalDAV server.

Events are identified by `(calendar_id, uid)`. On each sync, existing rows are updated and new ones are inserted. Events outside the booking window are purged to keep the table small.

`start_at` and `end_at` are stored in UTC.

## event_types

An event type is a named, fixed-duration meeting kind that a user offers. Examples: "30-min intro call", "1-hour workshop", "15-min check-in".

`slug` is the URL segment used in booking links: `/calendar/{username}/{slug}`. It is user-defined and must be unique per user. Changing a slug breaks any links that have been shared.

`booking_window_days` controls how far into the future guests can book. Slots beyond this window are not shown.

`active` allows a user to disable an event type without deleting it (and its booking history).

## availability_rules

A weekly repeating schedule that defines when the user is _in principle_ available. Each row covers one block on one weekday.

`start_time` and `end_time` are stored as `HH:MM` strings in the user's local timezone. Timezone handling is done at query time using the user's configured timezone (future feature; initial version uses server timezone).

Example: a user available Mon–Fri 09:00–12:00 and 13:00–17:00 would have 10 rows.

## bookings

A booking is a guest's request to occupy a specific time slot for a specific event type.

### Status Lifecycle

```
PENDING ──► CONFIRMED
        ──► REJECTED
        ──► CANCELLED  (by TTL expiry)
```

`token` is a random UUID v4 used in email links to confirm or reject without requiring the host to be logged in. It is single-use: once a terminal state is reached, subsequent requests with the same token are a no-op.

`reserved_until` is set to `created_at + INVITO_BOOKING_TTL`. The background GC job queries for PENDING bookings where `reserved_until < now` and marks them CANCELLED.

`start_at` and `end_at` are stored in UTC and reflect the exact slot the guest selected.

### Double-Booking Prevention

When a booking is submitted, the following check runs inside a SQLite transaction:

1. Re-fetch all `calendar_events` and existing `PENDING`/`CONFIRMED` bookings for the target time range.
2. If any overlap is found, the booking is rejected with a "slot no longer available" message.
3. Otherwise, the booking row is inserted.

This serialized write (SQLite's default) prevents two simultaneous requests from double-booking the same slot.
