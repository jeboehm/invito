PRAGMA journal_mode=WAL;

CREATE TABLE IF NOT EXISTS users (
    id         INTEGER PRIMARY KEY,
    oidc_sub   TEXT NOT NULL UNIQUE,
    email      TEXT NOT NULL,
    name       TEXT NOT NULL,
    username   TEXT NOT NULL UNIQUE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS sessions (
    id         TEXT PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS calendars (
    id           INTEGER PRIMARY KEY,
    user_id      INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    caldav_url   TEXT NOT NULL,
    username     TEXT NOT NULL,
    password     TEXT NOT NULL,
    display_name TEXT NOT NULL,
    color        TEXT NOT NULL DEFAULT '#6366f1',
    sync_enabled INTEGER NOT NULL DEFAULT 1,
    last_synced  DATETIME,
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS calendar_events (
    id          INTEGER PRIMARY KEY,
    calendar_id INTEGER NOT NULL REFERENCES calendars(id) ON DELETE CASCADE,
    uid         TEXT NOT NULL,
    start_at    DATETIME NOT NULL,
    end_at      DATETIME NOT NULL,
    summary     TEXT,
    synced_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(calendar_id, uid)
);

CREATE TABLE IF NOT EXISTS event_types (
    id                  INTEGER PRIMARY KEY,
    user_id             INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    slug                TEXT NOT NULL,
    title               TEXT NOT NULL,
    description         TEXT,
    confirmed_message   TEXT,
    rejected_message    TEXT,
    duration_minutes    INTEGER NOT NULL,
    color               TEXT NOT NULL DEFAULT '#6366f1',
    booking_window_days INTEGER NOT NULL DEFAULT 60,
    active              INTEGER NOT NULL DEFAULT 1,
    created_at          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, slug)
);

CREATE TABLE IF NOT EXISTS availability_rules (
    id         INTEGER PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    weekday    INTEGER NOT NULL CHECK(weekday BETWEEN 0 AND 6),
    start_time TEXT NOT NULL,
    end_time   TEXT NOT NULL,
    active     INTEGER NOT NULL DEFAULT 1
);

CREATE TABLE IF NOT EXISTS bookings (
    id             INTEGER PRIMARY KEY,
    event_type_id  INTEGER NOT NULL REFERENCES event_types(id),
    guest_name     TEXT NOT NULL,
    guest_email    TEXT NOT NULL,
    guest_note     TEXT,
    start_at       DATETIME NOT NULL,
    end_at         DATETIME NOT NULL,
    status         TEXT NOT NULL DEFAULT 'PENDING'
                       CHECK(status IN ('PENDING','CONFIRMED','REJECTED','CANCELLED')),
    token          TEXT NOT NULL UNIQUE,
    reserved_until DATETIME NOT NULL,
    created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
