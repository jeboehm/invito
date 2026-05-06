# Booking Flow

This document explains how a booking moves from initial guest request to a confirmed (or rejected) meeting.

## Overview

There are four parties involved:

- **Guest** — the person who wants to book time.
- **Host** — the Invito user who owns the booking page.
- **Invito** — the application, running as a server process.
- **CalDAV server** — the host's external calendar store.

## Step-by-Step

### 1. Guest opens a booking link

The host shares one of two link types:

| Link type  | URL                           | Effect                                        |
| ---------- | ----------------------------- | --------------------------------------------- |
| Pre-filled | `/calendar/{username}/{slug}` | Guest lands directly on a specific event type |
| Generic    | `/calendar/{username}/`       | Guest first chooses an event type             |

Both paths eventually land the guest on the slot picker for a specific event type.

### 2. Guest picks a date and time

The slot picker shows a date navigation. When the guest selects a date, an HTMX request fetches available slots for that day:

```
GET /calendar/{username}/{slug}?date=2026-05-15
```

Invito calculates available slots by:

1. Taking the user's `availability_rules` for that weekday.
2. Splitting the available window into slots of `event_type.duration_minutes`.
3. Removing slots that overlap any `calendar_events` (from CalDAV sync).
4. Removing slots where an existing PENDING or CONFIRMED booking exists.

The guest selects one slot.

### 3. Guest fills in the booking form

The form collects:

- Name (required)
- Email address (required)
- Note / message (optional)

On submit, Invito runs the double-booking check inside a transaction (see [data model](data-model.md#double-booking-prevention)) and creates a `Booking` row with:

- `status = PENDING`
- `token = random UUID`
- `reserved_until = now + INVITO_BOOKING_TTL`

The guest sees a confirmation page: "Your request has been sent. The host will confirm within 24 hours."

### 4. Invito sends the host a notification email

The email contains:

- Guest name, email, note
- Date, time, event type
- Two tokenized links:
  - **Confirm:** `{BASE_URL}/booking/{token}/confirm`
  - **Reject:** `{BASE_URL}/booking/{token}/reject`

The host does not need to be logged in to act on these links.

### 5a. Host confirms

Host clicks the confirm link. Invito:

1. Looks up the booking by `token`.
2. Verifies `status = PENDING` (idempotency: already-confirmed bookings return a "already confirmed" page).
3. Sets `status = CONFIRMED` in a transaction.
4. Writes a VEVENT to the host's primary CalDAV calendar.
5. Sends a confirmation email to both host and guest with the meeting details.

### 5b. Host rejects

Host clicks the reject link. Invito:

1. Looks up the booking by `token`.
2. Verifies `status = PENDING`.
3. Sets `status = REJECTED`.
4. Sends a rejection email to the guest.

No CalDAV write occurs.

### 5c. TTL expires (no action)

A background goroutine runs every minute and queries:

```sql
UPDATE bookings
SET status = 'CANCELLED'
WHERE status = 'PENDING' AND reserved_until < CURRENT_TIMESTAMP
```

For each newly-cancelled booking, a cancellation email is sent to the guest.

## State Diagram

```
                 ┌─────────────┐
                 │   PENDING   │
                 └──────┬──────┘
                        │
           ┌────────────┼────────────┐
           │            │            │
      Host confirms  Host rejects  TTL expires
           │            │            │
           ▼            ▼            ▼
      CONFIRMED      REJECTED    CANCELLED
```

All three terminal states are permanent. No transition out of a terminal state is possible.

## Slot Reservation vs. CalDAV

During the PENDING window, the slot is blocked within Invito's own database but not yet written to CalDAV. This means:

- If the host also receives a direct calendar invite externally, they could confirm a booking that then conflicts with an external event (if CalDAV sync hasn't run yet).
- This is an accepted trade-off. The alternative (writing a tentative event to CalDAV immediately) would require cleanup on rejection and is more complex.

On confirmation, the CalDAV write-back is attempted. If it fails (e.g. the CalDAV server is unreachable), the booking status is still set to CONFIRMED and the write-back is retried on the next sync cycle. The booking is not rolled back.
