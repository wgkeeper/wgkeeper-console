# Deployment

The console is a single container: the API and the built UI on one HTTP port, and a database. Check the [requirements](configuration.md#requirements) first.

## Install script

[`scripts/install.sh`](../scripts/install.sh) sets up the [Docker Compose](#docker-compose) deployment below on a Linux server:

```bash
curl -fsSL https://raw.githubusercontent.com/wgkeeper/wgkeeper-console/main/scripts/install.sh | sudo bash
```

It installs Docker if it's missing, downloads the Compose file of the release into `/opt/wgkeeper-console`, pins the image to that release, writes `console.env` with a new `SECRET_KEY` and a random admin password, starts the console and prints the address and the password.

Without `--domain` the console listens on `127.0.0.1:8000` of the server only. Reach it through an SSH tunnel:

```bash
ssh -L 8000:127.0.0.1:8000 your-server   # then open http://127.0.0.1:8000
```

Pass options after `bash -s --`:

| Option | Default | Description |
|--------|---------|-------------|
| `--version X.Y.Z` | latest stable release | Release to install, for example `2.0.0` |
| `--domain NAME` | | Add Caddy with an automatic Let's Encrypt certificate for this domain. Point the domain at the server first; ports 80 and 443 must be reachable |
| `--dir PATH` | `/opt/wgkeeper-console` | Install directory |

```bash
curl -fsSL https://raw.githubusercontent.com/wgkeeper/wgkeeper-console/main/scripts/install.sh | sudo bash -s -- --domain console.example.com
```

The console manages your VPN, so keep it off the open internet if you can. With a fixed office or VPN address, let only it through: uncomment the `remote_ip` block in `/opt/wgkeeper-console/examples/caddy/Caddyfile`, set the address, and run `docker compose exec caddy caddy reload --config /etc/caddy/Caddyfile` in `/opt/wgkeeper-console`.

The script never overwrites `console.env` or the data volume. It replaces `docker-compose.yml` with the one from the release, so put your own Compose changes in `docker-compose.override.yml` next to it; the script includes that file when it exists.

### Updating

Run the script again. Without `--version` it updates to the latest stable release; with `--version` it installs that release:

```bash
curl -fsSL https://raw.githubusercontent.com/wgkeeper/wgkeeper-console/main/scripts/install.sh | sudo bash -s -- --version 2.0.1
```

It keeps the settings, the secret key and the database, pulls the new image and restarts the console. Database migrations run on start.

## Docker Compose

[`docker-compose.yml`](../docker-compose.yml) runs `ghcr.io/wgkeeper/console` with `no-new-privileges`, reads settings from `console.env`, keeps the database in the `console_data` volume, publishes the port on `127.0.0.1:8000` only, checks `/readyz` and rotates the container logs (three files of 10 MB). Use the `edge` tag for the latest build from `main`.

```bash
cp console.env.example console.env   # set SECRET_KEY and BOOTSTRAP_ADMIN_PASSWORD
chmod 600 console.env
docker compose up -d
```

The console is reachable from this host only. For access from other machines, put a reverse proxy in front, or use an SSH tunnel.

## Behind a reverse proxy

The console speaks plain HTTP. Use whatever proxy you already run (Caddy, nginx, Traefik, a cloud load balancer), on the host or in Docker. It must:

1. Terminate TLS. Then set `COOKIE_SECURE=true`.
2. Forward to the console over loopback or a private network: `127.0.0.1:8000` from the host, `console:8000` from a container in the same Compose project. Never publish the console's port on a public interface.
3. Set `X-Forwarded-For` to the client address, overwriting any value the client sent, and list the proxy in [`TRUSTED_PROXIES`](configuration.md#behind-a-reverse-proxy).
4. Allow at least 30s per request and bodies up to 1 MB.
5. Pass `Host` through. The console checks that state-changing requests come from its own site.
6. Not route the metrics port through the public site.

The repository has two examples. Each has a proxy config and a Compose file that adds the proxy next to the console and sets `COOKIE_SECURE` and `TRUSTED_PROXIES`:

- [`examples/caddy/`](../examples/caddy): Caddy with an automatic Let's Encrypt certificate. Edit the domain in the `Caddyfile`.
- [`examples/nginx/`](../examples/nginx): nginx with an optional IP allowlist and rate limit. nginx doesn't get certificates itself; `compose.yml` shows how to get one with certbot on the host. Edit `server_name` and the certificate paths in `nginx.conf`.

Run from the directory with `docker-compose.yml`, with `caddy` or `nginx`:

```bash
docker compose -f docker-compose.yml -f examples/caddy/compose.yml up -d
```

To have plain `docker compose` commands include the proxy, put `COMPOSE_FILE=docker-compose.yml:examples/caddy/compose.yml` in `.env`.

## PostgreSQL

SQLite is enough for one console. To use PostgreSQL instead, add [`examples/postgres/compose.yml`](../examples/postgres/compose.yml): it runs PostgreSQL next to the console and sets `DATABASE_URL`. Put `POSTGRES_PASSWORD` in `.env` first:

```bash
echo "POSTGRES_PASSWORD=$(openssl rand -hex 24)" >> .env
docker compose -f docker-compose.yml -f examples/postgres/compose.yml up -d
```

For an existing PostgreSQL server, set `DATABASE_URL=postgres://user:password@host:5432/db` in `console.env` instead. Switching doesn't move data: add the nodes again afterwards.

## Backups

Back up the database and `SECRET_KEY` together. The node API keys in the database are encrypted with a key derived from `SECRET_KEY`; one without the other is useless.

With the default SQLite volume, from the directory with `docker-compose.yml`:

```bash
docker compose stop console
docker run --rm --volumes-from "$(docker compose ps -aq console)" -v "$PWD":/backup alpine \
  tar czf "/backup/console-data-$(date +%F).tgz" -C /app/data .
docker compose start console
```

To restore, stop the console, unpack the archive into `/app/data` the same way (`tar xzf ... -C /app/data`) and start it again with the same `SECRET_KEY`.

Losing the console doesn't affect the VPN: peers live on the nodes and keep working. You can add the nodes to a fresh console with their API keys.

## From source

Requires Go and Node.js with pnpm (the versions in `backend/go.mod` and `frontend/package.json`); see [CONTRIBUTING.md](../CONTRIBUTING.md). To build the image yourself:

```bash
docker build -t ghcr.io/wgkeeper/console:local .
```

and set `image: ghcr.io/wgkeeper/console:local` in `docker-compose.override.yml`.
