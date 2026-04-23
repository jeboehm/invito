# Invito

A lightweight, self-hosted scheduling tool. Give guests a link — they pick a time that works for both of you.

Invito connects to your existing CalDAV calendars to find open slots and blocks time when a booking is confirmed. No cloud lock-in, no monthly fee.

## Features

- **CalDAV integration** — connects to any CalDAV server (Nextcloud, iCloud, Google Calendar via DAV, etc.)
- **Multiple event types** — define different meeting kinds with fixed durations (e.g. "30-min intro call", "1-hour consultation")
- **Public booking pages** — share a link; guests book without needing an account
- **Pending approval** — every booking request waits for your confirmation for up to 24 hours
- **Email notifications** — accept or reject bookings directly from your inbox
- **OIDC login** — no separate user database; plug in your existing identity provider
- **Single binary** — deploy with one file and an SQLite database

## Quick Start

### Requirements

- Go 1.22+
- An OIDC provider (Keycloak, Authentik, Dex, GitHub, Google, …)
- A CalDAV server
- An SMTP server

### Run with Docker

```bash
docker run -d \
  -e INVITO_BASE_URL=https://invito.example.com \
  -e INVITO_OIDC_ISSUER=https://auth.example.com/realms/main \
  -e INVITO_OIDC_CLIENT_ID=invito \
  -e INVITO_OIDC_CLIENT_SECRET=secret \
  -e INVITO_SMTP_HOST=smtp.example.com \
  -e INVITO_SMTP_FROM=invito@example.com \
  -e INVITO_SESSION_SECRET=replace-with-32-byte-hex \
  -v invito-data:/data \
  ghcr.io/YOUR_ORG/invito:latest
```

### Build from source

```bash
git clone https://github.com/YOUR_ORG/invito.git
cd invito
go build -o invito ./cmd/invito
INVITO_BASE_URL=http://localhost:8080 ./invito
```

See [Getting Started](docs/tutorials/getting-started.md) for a full walkthrough.

## Documentation

Invito's documentation follows the [Diátaxis framework](https://diataxis.fr/):

| Type                                                              | Content                                               |
| ----------------------------------------------------------------- | ----------------------------------------------------- |
| [Tutorial](docs/tutorials/getting-started.md)                     | Step-by-step: from install to first confirmed booking |
| [How-to: Add a calendar](docs/how-to/add-calendar.md)             | Connect a CalDAV calendar                             |
| [How-to: Create an event type](docs/how-to/create-event-type.md)  | Define a new meeting kind                             |
| [How-to: Share a booking link](docs/how-to/share-booking-link.md) | Send guests a link                                    |
| [Explanation: Architecture](docs/explanation/architecture.md)     | Design decisions and system overview                  |
| [Explanation: Data model](docs/explanation/data-model.md)         | Entities and relationships                            |
| [Explanation: Booking flow](docs/explanation/booking-flow.md)     | How a booking moves from request to confirmation      |
| [Reference: Configuration](docs/reference/configuration.md)       | All environment variables                             |
| [Reference: HTTP API](docs/reference/api.md)                      | All routes and their behavior                         |

The full technical specification lives in [SPEC.md](SPEC.md).

## Contributing

Contributions are welcome. Please read [CONTRIBUTING.md](CONTRIBUTING.md) before opening a pull request.

## License

MIT — see [LICENSE](LICENSE).
