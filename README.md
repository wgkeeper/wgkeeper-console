<h1 align="center">WGKeeper Console</h1>

<p align="center">
  <strong>Web admin console for managing WGKeeper nodes and WireGuard peers.</strong>
</p>

<p align="center">
  Manage your WireGuard infrastructure — nodes, peers, and access — in one place.
</p>

<p align="center">
  <img width="1512" height="868" alt="wgkeeper-nodes-light" src="https://github.com/user-attachments/assets/27ceaf02-2cc0-4c23-91ef-0d31233e1152" />
</p>

<p align="center">
  <em>Nodes overview with real-time status and health indicators</em>
</p>

<p align="center">
  <a href="https://github.com/wgkeeper/wgkeeper-console/actions/workflows/ci.yml">
    <img src="https://github.com/wgkeeper/wgkeeper-console/actions/workflows/ci.yml/badge.svg?branch=main" />
  </a>
  <a href="https://github.com/wgkeeper/wgkeeper-console/blob/main/LICENSE">
    <img src="https://img.shields.io/badge/License-AGPL_v3-blue.svg" />
  </a>
  <a href="https://github.com/wgkeeper/wgkeeper-console/releases/latest">
    <img src="https://img.shields.io/github/v/release/wgkeeper/wgkeeper-console" />
  </a>
  <a href="https://github.com/wgkeeper/wgkeeper-console/pkgs/container/console">
    <img src="https://img.shields.io/badge/dynamic/json?url=https%3A%2F%2Fghcr-badge.elias.eu.org%2Fapi%2Fwgkeeper%2Fwgkeeper-console%2Fconsole&query=%24.downloadCount&label=image%20pulls&color=blue" />
  </a>
</p>

---

WGKeeper Console is the admin panel for WGKeeper. It gives you one place to manage your WireGuard infrastructure.

---

## Features

- 🖥 Manage multiple WireGuard nodes from one UI  
- 📊 Real-time node and peer status  
- 🔐 Built-in access control for the admin console  
- 📦 Download ready-to-use WireGuard configs  
- 🐳 Simple deployment with Docker   

---

## Quick start

### Basic Docker Compose

Run WGKeeper Console with a published image:

Generate a unique `SECRET_KEY` first:

```bash
openssl rand -hex 32
```

```yaml
services:
  wgkeeper-console:
    image: ghcr.io/wgkeeper/console:1.0.0
    container_name: wgkeeper-console
    ports:
      - "8000:8000"
    environment:
      PORT: 8000
      DATABASE_URL: file:/app/data/wgkeeper-console.db
      SECRET_KEY: paste-generated-64-char-hex-key-here
      BOOTSTRAP_ADMIN_PASSWORD: change-me-now
      COOKIE_SECURE: "false"
    volumes:
      - wgkeeper-console-data:/app/data
    restart: unless-stopped

volumes:
  wgkeeper-console-data:
```

Save it as `compose.yaml` and start the app:

```bash
docker compose up -d
```

Then open `http://localhost:8000`.

`SECRET_KEY` should be generated once and kept stable between restarts.
`COOKIE_SECURE=false` is required for plain HTTP. Use secure cookies for HTTPS deployments.

Default bootstrap login:

- username: `admin`
- password: value of `BOOTSTRAP_ADMIN_PASSWORD`

The first login requires a password change.

### Docker Compose with Caddy

If you want WGKeeper Console behind Caddy with automatic HTTPS, use this setup:

`compose.yaml`:

```yaml
services:
  wgkeeper-console:
    image: ghcr.io/wgkeeper/console:1.0.0
    container_name: wgkeeper-console
    environment:
      PORT: 8000
      DATABASE_URL: file:/app/data/wgkeeper-console.db
      SECRET_KEY: paste-generated-64-char-hex-key-here
      BOOTSTRAP_ADMIN_PASSWORD: change-me-now
      COOKIE_SECURE: "true"
    volumes:
      - wgkeeper-console-data:/app/data
    restart: unless-stopped

  caddy:
    image: caddy:2
    container_name: wgkeeper-console-caddy
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./Caddyfile:/etc/caddy/Caddyfile:ro
      - caddy-data:/data
      - caddy-config:/config
    depends_on:
      - wgkeeper-console
    restart: unless-stopped

volumes:
  wgkeeper-console-data:
  caddy-data:
  caddy-config:
```

`Caddyfile`:

```caddy
console.example.com {
  reverse_proxy wgkeeper-console:8000
}
```

Then start it with:

```bash
docker compose up -d
```

Replace `console.example.com` with your real domain pointed at the server. Caddy will provision HTTPS automatically.

## Repository Docker setup

This repository also includes a local [docker-compose.yml](./docker-compose.yml) with Caddy for project development and self-hosted builds.

## Configuration

Most users only need these variables:

| Variable | Default | Purpose |
|----------|---------|---------|
| `SECRET_KEY` | none | Required secret for sessions and stored API keys |
| `BOOTSTRAP_ADMIN_PASSWORD` | none | Initial admin password on first start |
| `DATABASE_URL` | `file:/app/data/wgkeeper-console.db` | SQLite file or PostgreSQL connection URL |
| `PORT` | `8000` | App port |
| `COOKIE_SECURE` | `true` in production | Set to `false` only when serving over plain HTTP |

Example `.env`:

```env
SECRET_KEY=replace-with-generated-64-char-hex-key
BOOTSTRAP_ADMIN_PASSWORD=change-me-now
DATABASE_URL=file:/app/data/wgkeeper-console.db
PORT=8000
COOKIE_SECURE=false
```

PostgreSQL is also supported:

```env
DATABASE_URL=postgres://user:password@postgres:5432/wgkeeper_console
```

A minimal template is available in [.env.example](./.env.example).

Optional variables:

| Variable | Default | Purpose |
|----------|---------|---------|
| `DOCS` | `false` | Enables Swagger UI |
| `DEBUG` | `false` | Enables debug logging |

## API docs

Set `DOCS=true` and start the backend to enable Swagger UI at `http://localhost:8000/docs/index.html`.

More Swagger details live in [backend/SWAGGER.md](./backend/SWAGGER.md).

## Testing

```bash
cd backend && go test ./...
cd frontend && pnpm test && pnpm build
```

## License

AGPL-3.0. See [LICENSE](./LICENSE).
