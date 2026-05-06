# Architecture

This document explains the design decisions behind Invito — why the system is shaped the way it is, and what trade-offs were made.

## Design Goals

1. **Minimal operational footprint.** One process, one file, one config. No external queue, no Redis, no PostgreSQL.
2. **No cloud dependency.** All scheduling data stays in your infrastructure.
3. **Pluggable identity.** OIDC means you can reuse whatever auth you already have (Keycloak, Authentik, GitHub, Google, etc.).
4. **Correctness over features.** A booking that is confirmed must not double-book. This is the only hard invariant.

## Why Go?

Go produces a statically-linked binary with no runtime dependencies. Combined with `modernc.org/sqlite` (a CGO-free SQLite port), the entire application ships as a single executable. There is no Node.js version mismatch, no Python virtualenv, no JVM to tune.

Go's standard library covers HTTP, TLS, templating, and SMTP. The number of third-party dependencies is kept intentionally small.

## Why SQLite?

SQLite is appropriate here because:

- **Write concurrency is low.** Bookings arrive one at a time; CalDAV sync writes in short bursts.
- **Single-instance deployment.** Invito is not designed for horizontal scaling. If you need HA, run a reverse proxy in front.
- **Backups are trivial.** `cp invito.db invito.db.bak` or use Litestream for continuous replication.

The schema uses `UNIQUE` constraints and SQLite's serialized write mode to prevent double-bookings without application-level locking.

## Why HTMX?

The booking flow involves one dynamic interaction: selecting a date and seeing available slots update without a full page reload. HTMX handles this with a single `hx-get` attribute — no build step, no bundler, no JavaScript framework.

The rest of the UI is plain HTML forms. This keeps the template code simple and testable with `html/template`.

## Single-Process Architecture

Invito runs as a single OS process with three internal goroutines:

1. **HTTP server** — handles all web traffic.
2. **CalDAV sync loop** — polls connected calendars every `INVITO_SYNC_INTERVAL`.
3. **Booking GC loop** — marks PENDING bookings as CANCELLED after their TTL expires.

There is no message broker between these goroutines. The HTTP server reads from the database; the background goroutines write to it. SQLite's WAL mode allows concurrent reads during writes.

## Multi-User Model

Each OIDC identity maps to one user row. Users are isolated: a user's calendars, event types, and bookings are not visible to or accessible by other users. There is no concept of "admin" within the application — the OIDC provider controls who can log in.

Public booking pages are keyed by `username` (a URL-safe slug derived from the OIDC `preferred_username` claim). A user can change their username in their profile settings, but doing so invalidates all previously shared booking links.

## Dual HTTP Multiplexer

Invito uses two separate HTTP multiplexers:

1. **Main mux** — handles all routes except `/widget/`. Applies CSRF double-submit cookie protection and sets `X-Frame-Options: DENY` to prevent the UI from being embedded in iframes.
2. **Widget mux** — handles `/widget/{username}/{slug}` routes only. No CSRF middleware (the widget is stateless and does not use cookies), and `X-Frame-Options` is set to `ALLOWALL` to permit iframe embedding on external sites.

The booking logic (slot calculation, conflict detection, email notifications) is shared between both multiplexers. The split is purely about transport-level security policy.

## Security Considerations

| Concern            | Mitigation                                                                                                                             |
| ------------------ | -------------------------------------------------------------------------------------------------------------------------------------- |
| Double-booking     | `UNIQUE` constraint on (event_type_id, start_at) for CONFIRMED bookings; slot availability re-checked in a transaction at booking time |
| CSRF               | Double-submit cookie on all state-changing forms                                                                                       |
| CalDAV credentials | AES-256-GCM encrypted at rest; key derived from `INVITO_SESSION_SECRET`                                                                |
| Session fixation   | Session ID regenerated on login                                                                                                        |
| Email token replay | Confirm/reject checks booking status; only PENDING transitions are applied                                                             |
| Open redirect      | OIDC `state` parameter validated; redirect target whitelist enforced                                                                   |

## What Is Out of Scope

- **Recurring availability exceptions** (e.g. holidays, one-off blocked days). The current model only supports weekly rules.
- **Buffer time between meetings.** Slots are calculated without padding.
- **Group bookings / multiple attendees per slot.**
- **Payments.**
- **Webhooks.**
- **SMS notifications.**

These may be addressed in future versions.
