#!/usr/bin/env bash
# Installs or upgrades wgkeeper console with Docker Compose on a Linux host.
#
#   curl -fsSL https://raw.githubusercontent.com/wgkeeper/wgkeeper-console/main/scripts/install.sh | sudo bash
#   curl -fsSL https://raw.githubusercontent.com/wgkeeper/wgkeeper-console/main/scripts/install.sh | sudo bash -s -- --domain console.example.com
#
# Options:
#   --version X.Y.Z   release to install (default: the latest stable release)
#   --domain NAME     put Caddy with an automatic HTTPS certificate in front of the console
#   --dir PATH        install directory (default: /opt/wgkeeper-console)
#
# Running it again upgrades the console to the given version. An existing
# console.env (SECRET_KEY, admin password) and the database are never touched.
#
# For tests: WGKEEPER_IMAGE replaces the image, WGKEEPER_FILES copies the
# Compose files from a local checkout instead of downloading them.
set -euo pipefail

REPO=wgkeeper/wgkeeper-console
IMAGE=ghcr.io/wgkeeper/console
VERSION=""
DOMAIN=""
DIR=/opt/wgkeeper-console

log() { echo "==> $*"; }
die() {
	echo "error: $*" >&2
	exit 1
}

usage() {
	cat <<'USAGE'
Usage: install.sh [--version X.Y.Z] [--domain NAME] [--dir PATH]

  --version X.Y.Z   release to install (default: the latest stable release)
  --domain NAME     put Caddy with an automatic HTTPS certificate in front of the console
  --dir PATH        install directory (default: /opt/wgkeeper-console)
USAGE
	exit "${1:-0}"
}

while [[ $# -gt 0 ]]; do
	case "$1" in
	--version) VERSION="${2#v}" && shift 2 ;;
	--domain) DOMAIN="$2" && shift 2 ;;
	--dir) DIR="$2" && shift 2 ;;
	-h | --help) usage 0 ;;
	*) die "unknown option: $1 (see --help)" ;;
	esac
done

[[ "$(uname -s)" == "Linux" ]] || die "the install script supports Linux only; elsewhere use docker-compose.yml directly"
[[ "$(id -u)" -eq 0 ]] || die "run as root (sudo bash ...)"
command -v curl >/dev/null || die "curl is required"
if [[ -n "$DOMAIN" && ! "$DOMAIN" =~ ^[A-Za-z0-9.-]+$ ]]; then
	die "invalid domain: $DOMAIN"
fi

random_hex() { openssl rand -hex "$1" 2>/dev/null || head -c "$1" /dev/urandom | od -An -tx1 | tr -d ' \n'; }

# --- Docker -----------------------------------------------------------------
if ! command -v docker >/dev/null; then
	log "Installing Docker"
	curl -fsSL https://get.docker.com | sh
fi
docker compose version >/dev/null 2>&1 || die "Docker Compose v2 is required (docker compose)"

# --- Version ----------------------------------------------------------------
if [[ -z "$VERSION" ]]; then
	VERSION="$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" |
		sed -n 's/.*"tag_name": *"v\{0,1\}\([^"]*\)".*/\1/p' | head -n 1)"
	[[ -n "$VERSION" ]] || die "could not find the latest release; pass --version"
fi
[[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?$ ]] || die "invalid version: $VERSION"
log "Installing wgkeeper console $VERSION into $DIR"

# --- Files ------------------------------------------------------------------
# fetch PATH copies PATH from the release (or WGKEEPER_FILES) into $DIR.
fetch() {
	local path=$1
	mkdir -p "$(dirname "$DIR/$path")"
	if [[ -n "${WGKEEPER_FILES:-}" ]]; then
		cp "$WGKEEPER_FILES/$path" "$DIR/$path"
	else
		curl -fsSL "https://raw.githubusercontent.com/$REPO/v$VERSION/$path" -o "$DIR/$path" ||
			die "could not download $path for $VERSION (the installer supports 2.0.0 and newer)"
	fi
}

mkdir -p "$DIR"
cd "$DIR"

fetch docker-compose.yml
image="${WGKEEPER_IMAGE:-$IMAGE:$VERSION}"
sed -i "s|^\([[:space:]]*image:\) $IMAGE\(:.*\)\{0,1\}$|\1 $image|" docker-compose.yml
grep -q "image: $image$" docker-compose.yml || die "could not set the image in docker-compose.yml"

admin_password=""
if [[ -f console.env ]]; then
	log "Keeping the existing console.env"
else
	log "Writing console.env with a new secret key and admin password"
	admin_password="$(random_hex 12)"
	umask 077
	cat >console.env <<ENV
# wgkeeper console settings. All variables: https://github.com/$REPO/blob/v$VERSION/docs/configuration.md

# Signs sessions and encrypts stored node API keys. Back it up with the
# database: if it changes, the stored node API keys can't be decrypted.
SECRET_KEY=$(random_hex 32)

# Password of the "admin" user on the first start; changed at the first login.
BOOTSTRAP_ADMIN_PASSWORD=$admin_password

# false while the console is reached over plain HTTP (loopback or SSH tunnel);
# the Caddy layer turns it on.
COOKIE_SECURE=false
ENV
	umask 022
fi

compose_files=docker-compose.yml
if [[ -n "$DOMAIN" || -f examples/caddy/compose.yml ]]; then
	fetch examples/caddy/compose.yml
	if [[ ! -f examples/caddy/Caddyfile ]]; then
		[[ -n "$DOMAIN" ]] || die "examples/caddy/Caddyfile is missing; pass --domain"
		fetch examples/caddy/Caddyfile
		sed -i "s/console\.example\.com/$DOMAIN/" examples/caddy/Caddyfile
	fi
	compose_files="$compose_files:examples/caddy/compose.yml"
fi
# Your own changes go in docker-compose.override.yml; this script never touches it.
if [[ -f docker-compose.override.yml ]]; then
	compose_files="$compose_files:docker-compose.override.yml"
fi
# Keep other settings in .env (for example POSTGRES_PASSWORD); only COMPOSE_FILE is ours.
touch .env
sed -i '/^COMPOSE_FILE=/d' .env
echo "COMPOSE_FILE=$compose_files" >>.env

# --- Firewall ---------------------------------------------------------------
if [[ "$compose_files" == *caddy* ]] && command -v ufw >/dev/null && ufw status 2>/dev/null | grep -q "Status: active"; then
	log "Opening 80/tcp and 443/tcp in ufw"
	ufw allow 80/tcp >/dev/null
	ufw allow 443/tcp >/dev/null
fi

# --- Start ------------------------------------------------------------------
log "Starting the console"
docker compose pull --quiet --ignore-buildable 2>/dev/null || true
docker compose up -d --remove-orphans

for _ in $(seq 1 60); do
	if curl -fsS http://127.0.0.1:8000/readyz >/dev/null 2>&1; then
		break
	fi
	sleep 1
done
curl -fsS http://127.0.0.1:8000/readyz >/dev/null 2>&1 ||
	die "the console did not become ready; see: cd $DIR && docker compose logs console"

if [[ "$compose_files" == *caddy* ]]; then
	address="https://$(sed -n 's/^\([A-Za-z0-9.-]*\) {$/\1/p' examples/caddy/Caddyfile | head -n 1)"
else
	address="http://127.0.0.1:8000 (this host only: ssh -L 8000:127.0.0.1:8000 <server>, or use --domain)"
fi
if [[ -n "$admin_password" ]]; then
	login="admin / $admin_password  (you'll be asked to change it)"
else
	login="unchanged"
fi

cat <<DONE

wgkeeper console $VERSION is running in $DIR.

  Address:  $address
  Login:    $login

Settings and the secret key are in $DIR/console.env; back it up with the database.
Manage it with: cd $DIR && docker compose ps | logs | down
DONE
