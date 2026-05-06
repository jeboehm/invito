# Configuration Reference

Invito is configured entirely via environment variables. There are no configuration files.

## Required Variables

### `INVITO_BASE_URL`

The public base URL at which Invito is reachable, without a trailing slash.

Used to construct:

- OIDC redirect URI (`{INVITO_BASE_URL}/auth/callback`)
- Confirm/reject links in notification emails

```
INVITO_BASE_URL=https://invito.example.com
```

---

### `INVITO_SESSION_SECRET`

A 32-byte value encoded as a 64-character hex string. Used for:

- Signing session cookies (HMAC-SHA256)
- Encrypting CalDAV passwords at rest (AES-256-GCM)

Generate a suitable value:

```bash
openssl rand -hex 32
```

Rotating this value invalidates all active sessions and renders stored CalDAV passwords unreadable. Users will need to reconnect their calendars.

---

### `INVITO_OIDC_ISSUER`

The OIDC issuer URL. Invito performs discovery at `{issuer}/.well-known/openid-configuration`.

```
INVITO_OIDC_ISSUER=https://auth.example.com/realms/main
```

---

### `INVITO_OIDC_CLIENT_ID`

The client ID registered with your OIDC provider.

---

### `INVITO_OIDC_CLIENT_SECRET`

The client secret for the registered OIDC application.

---

### `INVITO_SMTP_HOST`

Hostname of the SMTP server used to send notification emails.

---

### `INVITO_SMTP_USER`

SMTP authentication username.

---

### `INVITO_SMTP_PASSWORD`

SMTP authentication password.

---

### `INVITO_SMTP_FROM`

The `From` address for outgoing emails.

```
INVITO_SMTP_FROM=invito@example.com
```

---

## Optional Variables

### `INVITO_DB_PATH`

Path to the SQLite database file.

- **Default:** `./invito.db`
- Invito creates the file on first run if it does not exist.
- Ensure the parent directory is writable.

```
INVITO_DB_PATH=/data/invito.db
```

---

### `INVITO_LISTEN_ADDR`

TCP address and port for the HTTP server.

- **Default:** `:8080`

```
INVITO_LISTEN_ADDR=:3000
INVITO_LISTEN_ADDR=127.0.0.1:8080
```

---

### `INVITO_SMTP_PORT`

SMTP port number.

- **Default:** `587` (STARTTLS)
- Use `465` for implicit TLS (SMTPS).
- Use `25` for unauthenticated relay (not recommended).

---

### `INVITO_SYNC_INTERVAL`

How often Invito polls CalDAV servers for calendar updates. Accepts Go duration strings.

- **Default:** `15m`
- Minimum recommended: `5m`

```
INVITO_SYNC_INTERVAL=30m
```

---

### `INVITO_BOOKING_TTL`

How long a PENDING booking is held before being automatically cancelled. Accepts Go duration strings.

- **Default:** `24h`

```
INVITO_BOOKING_TTL=48h
```

---

## OIDC Provider Setup

Your OIDC provider must be configured with:

- **Redirect URI:** `{INVITO_BASE_URL}/auth/callback`
- **Scopes:** `openid email profile`
- **Claims used:** `sub`, `email`, `name`, `preferred_username`

The `preferred_username` claim is used to generate the user's public booking URL slug. If your provider does not include this claim, configure it to do so, or users will receive a randomly-generated slug.

### Example: Keycloak

1. Create a new client with type **OpenID Connect**.
2. Set **Valid Redirect URIs** to `{INVITO_BASE_URL}/auth/callback`.
3. Enable **Standard Flow**.
4. Under **Client Scopes**, ensure `profile` is included (it contains `preferred_username`).

### Example: GitHub (via Dex or Authentik)

Configure your bridge to map the GitHub username to the `preferred_username` claim.

---

## Docker Environment File

A complete example `.env` file for Docker deployments:

```env
INVITO_BASE_URL=https://invito.example.com
INVITO_DB_PATH=/data/invito.db
INVITO_SESSION_SECRET=<output of: openssl rand -hex 32>

INVITO_OIDC_ISSUER=https://auth.example.com/realms/main
INVITO_OIDC_CLIENT_ID=invito
INVITO_OIDC_CLIENT_SECRET=your-client-secret

INVITO_SMTP_HOST=smtp.example.com
INVITO_SMTP_PORT=587
INVITO_SMTP_USER=invito@example.com
INVITO_SMTP_PASSWORD=your-smtp-password
INVITO_SMTP_FROM=invito@example.com

INVITO_SYNC_INTERVAL=15m
INVITO_BOOKING_TTL=24h
```
