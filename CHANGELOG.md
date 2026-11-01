# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
Each release's notes on GitHub are taken from its section here.

## [Unreleased]

## [2.0.0] - 2026-11-01

A major release with breaking changes to the deployment files and the API. Read the **Breaking** entries and [Upgrading from 1.x](#upgrading-from-1x) before upgrading.

The console gets a new interface built on shadcn/ui, sessions that really end on logout, a hardened API, health checks and Prometheus metrics, and an install script that sets it up with one command. Deployment now follows wgkeeper-node: a published image, settings in `console.env`, the port on loopback only and the reverse proxy as an example.

### Upgrading from 1.x

1. Back up the database and note your `SECRET_KEY`: the stored node API keys can't be decrypted without it.
2. Stop the old stack with `docker compose down` (the volume is kept).
3. Replace `docker-compose.yml` with the new one and create `console.env` from `console.env.example`. Put your `SECRET_KEY` there, and move any other settings you passed to the old `app` service.
4. Copy the database into the new volume. Volume names start with the Compose project name, by default the folder name; `docker volume ls` shows them:

   ```bash
   docker volume create <project>_console_data
   docker run --rm -v <project>_wg-keeper-data:/from -v <project>_console_data:/to alpine cp -a /from/. /to/
   ```

5. If you used the bundled Caddy, move your edits from `Caddyfile` to `examples/caddy/Caddyfile` and start with `docker compose -f docker-compose.yml -f examples/caddy/compose.yml up -d`. Otherwise run `docker compose up -d` and point your proxy at `127.0.0.1:8000`.
6. Sign in again: sessions from 1.x are not valid in 2.0.

### Added

- `scripts/install.sh` installs the console on a Linux server with one command: it installs Docker if needed, writes `console.env` with a new `SECRET_KEY` and admin password, pins the image to a release and, with `--domain`, adds Caddy with HTTPS. See `docs/deployment.md`.
- `GET /healthz` (the process is up) and `GET /readyz` (the database answers), and a health check in the image and in `docker-compose.yml`.
- Prometheus metrics on a separate port (`METRICS_PORT`, protected by `METRICS_TOKEN`): API requests and latency, and requests to nodes and the release check by operation and result. See `docs/metrics.md` and the alerting rules in `examples/prometheus/alerts.yml`.
- Every request gets an ID, returned in `X-Request-Id` and logged with every line about that request. `LOG_FORMAT=json` switches logs to JSON.
- `GET /api/nodes/:id` returns one node, and `GET /api/nodes` returns `summary` with online, offline and outdated counts for the whole fleet.
- The interface:
  - command menu (`⌘K` / `Ctrl K`) to jump to any node or run an action;
  - a sidebar that lists the fleet with each node's status;
  - status counts for the whole fleet;
  - search and filters on the node and peer lists;
  - a peer details panel with traffic and the exact dates;
  - downloading a generated config as a `.conf` file;
  - light, dark and system themes.
- Page, filters, the open tab and the open peer are kept in the URL, so reload, the back button and a shared link keep them.
- `docs/` with configuration, deployment, security and metrics references, and `examples/` for Caddy, nginx, PostgreSQL and Prometheus.

### Changed

- **Breaking:** `docker-compose.yml` is rewritten:
  - it runs the published `ghcr.io/wgkeeper/console` image instead of building one;
  - the service is `console` instead of `app`, and the volume is `console_data` instead of `wg-keeper-data`;
  - settings are read from `console.env`; `.env.example` is now `console.env.example`;
  - the port is published on `127.0.0.1:8000` only.

  The bundled Caddy moved to `examples/caddy/`, next to an nginx example. See [Upgrading from 1.x](#upgrading-from-1x).
- **Breaking:** generating a config is `POST /api/nodes/:id/config` instead of `GET`, since it creates or rotates the peer on the node.
- **Breaking:** invalid values in environment variables stop the console at startup with an error naming the variable, instead of silently falling back to the default (`SESSION_TTL=7d` used to become 24 hours).
- **Breaking:** sessions from 1.x are not accepted; everyone signs in again once.
- The interface is rebuilt on shadcn/ui, with the lowercase wgkeeper wordmark. Rotating a peer's keys asks for confirmation and says that the old config stops working.
- Errors from nodes keep their meaning: a missing node is `404 node_not_found`, a node that rejects the key, fails or can't be reached is `502` with the node's status in the body.
- Refreshing node status runs once at a time, gives up after 20 seconds, and keeps a node's last known status when its check was cut off instead of marking it offline.
- The latest wgkeeper-node release is checked at most once per period, with a back-off after failures, instead of on every node list request.
- Built with Go 1.27 and Node.js 26; the runtime image is based on Alpine 3.24; dependencies updated.

### Removed

- The Swagger-related `backend/Makefile`, `backend/SWAGGER.md` and `backend/.swagignore`. The Swagger UI itself stays: set `DOCS=true` and open `/docs/index.html`.

### Fixed

- Generating a config or changing the password no longer hangs on a network error.
- Moving from one node to another no longer shows the previous node's stats, peers or generated config.
- A late response for a page you already left no longer replaces the one you're looking at, in the node list and the peer list.
- A direct link to a node that isn't on the first page of the node list works.
- Expiry dates entered without a time mean the end of that day in local time, like dates with a time, instead of UTC.

### Security

- Logging out or changing the password ends every session of the user, including copies of the cookie. Sessions also end 7 days after login however often they are refreshed.
- Until the bootstrap password is changed, the API refuses everything except the password change; before, only the interface enforced it.
- State-changing requests are refused when the browser says they come from another site (`Sec-Fetch-Site`, or `Origin` against `Host`), and request bodies must be JSON.
- A login for an unknown user takes as long as one with a wrong password, so usernames can't be guessed by timing. Login attempts are reserved before the password check, so parallel requests can't get around the rate limit, and IPv6 clients are counted per /64.
- Requests to nodes:
  - redirects are not followed, so the API key can't be sent to another host;
  - responses are size-limited;
  - node addresses with credentials in the URL are refused;
  - the console never connects to link-local or cloud metadata addresses (`169.254.169.254`, `fd00:ec2::254`), checked after DNS resolution.
- Every API response has `Cache-Control: no-store`, so peer private keys in generated configs don't stay in browser or proxy caches.
- Node API keys stored by older versions without encryption are encrypted at startup.

## Earlier releases

Notes for 1.2.0 and earlier are on [GitHub Releases](https://github.com/wgkeeper/wgkeeper-console/releases).

[Unreleased]: https://github.com/wgkeeper/wgkeeper-console/compare/v2.0.0...HEAD
[2.0.0]: https://github.com/wgkeeper/wgkeeper-console/compare/v1.2.0...v2.0.0
