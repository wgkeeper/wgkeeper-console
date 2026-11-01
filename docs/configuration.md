# Configuration

## Requirements

| | Requirement |
|---|-------------|
| Host | Any Linux, macOS or Windows host with Docker; no special capabilities |
| Database | SQLite (default, a file in the data volume) or PostgreSQL 14+ |
| Nodes | [wgkeeper node](https://github.com/wgkeeper/wgkeeper-node) 2.0.0 or newer, reachable from the console over HTTP(S) |

## Environment variables

The console reads its settings from environment variables. With [Docker Compose](deployment.md#docker-compose) they live in `console.env`; [`console.env.example`](../console.env.example) is a starting point.

Invalid values stop the console at startup with a list of every problem, instead of silently falling back to defaults: `SESSION_TTL=7d` is an error, not 24 hours.

### Required

| Variable | Description |
|----------|-------------|
| `SECRET_KEY` | 64 hex characters (`openssl rand -hex 32`). Signs sessions and encrypts stored node API keys. Keep it stable and back it up: if it changes, every session ends and the stored node API keys can't be decrypted. Outside production it may be unset, then a temporary key is used and API keys are stored unencrypted |
| `BOOTSTRAP_ADMIN_PASSWORD` | Password of the first user, created on the first start. You are asked to change it at the first login. Ignored once a user exists |

### Server

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8000` | HTTP port |
| `APP_ENV` | `production` | `production` serves the built UI and requires `SECRET_KEY`; anything else is development mode, where the UI comes from the Vite dev server |
| `TRUSTED_PROXIES` | empty | Comma-separated proxy IPs or CIDRs whose `X-Forwarded-For` is trusted. Needed behind a reverse proxy, see [below](#behind-a-reverse-proxy) |
| `HTTP_READ_TIMEOUT` | `10s` | |
| `HTTP_READ_HEADER_TIMEOUT` | `5s` | |
| `HTTP_WRITE_TIMEOUT` | `30s` | Keep it above 20s: a status refresh of many nodes can take that long |
| `HTTP_IDLE_TIMEOUT` | `60s` | |
| `HTTP_SHUTDOWN_TIMEOUT` | `10s` | How long in-flight requests get to finish on shutdown |
| `CORS_ALLOW_ORIGINS` | none in production | Comma-separated origins allowed to call the API from another site. Not needed when the UI is served by the console itself |
| `DOCS` | `false` | Serve the Swagger UI at `/docs/index.html` |

### Sessions and cookies

| Variable | Default | Description |
|----------|---------|-------------|
| `SESSION_TTL` | `24h` | Session length. A session is refreshed while in use, up to 7 days after login (or `SESSION_TTL`, if longer) |
| `COOKIE_SECURE` | `true` in production | Send the session cookie over HTTPS only. Set `false` only when the console is reached over plain HTTP (its loopback port or an SSH tunnel) |
| `COOKIE_SAMESITE` | `lax` | `lax`, `strict` or `none`. `none` requires `COOKIE_SECURE=true` |
| `COOKIE_DOMAIN` | empty | Cookie domain, a hostname such as `console.example.com` |
| `BOOTSTRAP_ADMIN_USERNAME` | `admin` | Name of the first user |

### Database

| Variable | Default | Description |
|----------|---------|-------------|
| `DATABASE_URL` | `file:./wgkeeper-console.db` | `file:<path>` for SQLite or `postgres://user:password@host:5432/db` for PostgreSQL. The image sets `file:/app/data/wgkeeper-console.db` |
| `DB_MAX_OPEN_CONNS` | `20` | PostgreSQL only; SQLite always uses one connection |
| `DB_MAX_IDLE_CONNS` | `10` | |
| `DB_CONN_MAX_LIFETIME` | `1h` | |
| `DB_CONN_MAX_IDLE_TIME` | `15m` | |

### Logs and metrics

| Variable | Default | Description |
|----------|---------|-------------|
| `LOG_FORMAT` | `text` | `json` for one JSON object per line |
| `DEBUG` | `false` | Debug-level logs, including SQL queries (with placeholders, never values) |
| `METRICS_PORT` | unset (off) | Port of the Prometheus `/metrics` endpoint. See [Metrics](metrics.md) |
| `METRICS_TOKEN` | | Bearer token for `/metrics`, at least 32 characters. Required with `METRICS_PORT` |

## Behind a reverse proxy

By default the console takes the client address from the TCP connection and ignores `X-Forwarded-For`, which is right when nothing sits in front of it. Behind a proxy every request would then come from the proxy, and the login rate limit would treat all users as one client: ten wrong passwords from anyone would lock everyone out. Tell the console which proxies to trust:

| Setup | `TRUSTED_PROXIES` |
|-------|-------------------|
| Proxy on the same host | `127.0.0.1,::1` |
| Proxy in a container on the same Docker network | `172.16.0.0/12,192.168.0.0/16,10.0.0.0/8` (what the [examples](../examples) set), or your network's subnet |
| Load balancer with a known address | that address, for example `203.0.113.10` |
| No proxy | leave it unset |

Only trust addresses that clients can't reach directly, or they can pick their own `X-Forwarded-For`. The console's port must not be published on a public interface.
