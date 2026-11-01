#!/usr/bin/env bash
# End-to-end test of the console image against a real wgkeeper-node.
#
# Starts the node and the console in one Docker network, then drives the
# console API the way the UI does: first login and the forced password change,
# adding the node, generating a client config, peers, logout. A WireGuard
# client brings up a tunnel with the config the console generated and must
# complete a handshake with the node, so the test also checks the contract
# between the two projects.
#
# Needs Docker on a Linux kernel with WireGuard (GitHub-hosted Ubuntu runners,
# Docker Desktop), curl and jq.
#
#   E2E_IMAGE=wgkeeper-console:e2e test/e2e/run.sh   # use an image you built
#   E2E_BUILD=1 test/e2e/run.sh                      # build it first
#   E2E_KEEP=1 test/e2e/run.sh                       # keep containers for debugging
set -euo pipefail

IMAGE="${E2E_IMAGE:-wgkeeper-console:e2e}"
# The node release the console is tested against; Dependabot doesn't track
# this one, bump it with each node release.
NODE_IMAGE="ghcr.io/wgkeeper/node:2.0.0@sha256:77a7c2b63bba078df450782e923fe988996bbe3ce4ccfad059bc02a0f6210426"
ALPINE="alpine:3.24@sha256:be2656061b536c1883c4034790e21d0a09c5aaaf51968b89368a3c0371ea2c9a"
NAME=wgkc-e2e
NET_SUBNET=172.31.0.0/24
NODE_IP=172.31.0.10
CONSOLE=http://127.0.0.1:18080
NODE_READY=http://127.0.0.1:18081/readyz

WORK="$(mktemp -d)"
NODE_KEY="$(openssl rand -hex 32)"
ADMIN_PASSWORD="e2e-$(openssl rand -hex 8)"
JAR="$WORK/cookies"
failed=0

cleanup() {
	status=$?
	if [[ "${E2E_KEEP:-0}" == "1" ]]; then
		echo "E2E_KEEP=1: leaving $NAME-* containers and network in place"
		return
	fi
	if [[ "$status" -ne 0 ]] || [[ "$failed" -ne 0 ]]; then
		echo "--- console logs"
		docker logs "$NAME-console" 2>&1 | tail -n 60 || true
		echo "--- node logs"
		docker logs "$NAME-node" 2>&1 | tail -n 40 || true
	fi
	docker rm -f "$NAME-console" "$NAME-node" "$NAME-client" >/dev/null 2>&1 || true
	docker network rm "$NAME" >/dev/null 2>&1 || true
	rm -rf "$WORK"
}
trap cleanup EXIT

pass() { echo "PASS  $1"; }
fail() {
	echo "FAIL  $1"
	failed=1
}
check() { # check NAME CONDITION-COMMAND...
	local name=$1
	shift
	if "$@" >/dev/null 2>&1; then pass "$name"; else fail "$name"; fi
}

# api METHOD PATH [JSON] prints the body; the status goes to $WORK/status.
api() {
	local args=(-sS -o "$WORK/body" -w '%{http_code}' -b "$JAR" -c "$JAR" -X "$1")
	if [[ $# -ge 3 ]]; then args+=(-H "Content-Type: application/json" -d "$3"); fi
	curl "${args[@]}" "$CONSOLE$2" >"$WORK/status"
	cat "$WORK/body"
}
status() { cat "$WORK/status"; }

wait_for() {
	for _ in $(seq 1 60); do
		if curl -fsS "$1" >/dev/null 2>&1; then return 0; fi
		sleep 1
	done
	echo "$1 did not become ready"
	return 1
}

if [[ "${E2E_BUILD:-0}" == "1" ]]; then
	docker build -t "$IMAGE" .
fi

docker network create --subnet "$NET_SUBNET" "$NAME" >/dev/null

# --- node -------------------------------------------------------------------
mkdir -p "$WORK/node-data" "$WORK/node-wg"
cat >"$WORK/config.yaml" <<YAML
auth:
  api_key: "$NODE_KEY"
wireguard:
  subnet: "10.67.0.0/24"
  server_ip: "10.67.0.1"
YAML
docker run -d --name "$NAME-node" \
	--network "$NAME" --ip "$NODE_IP" \
	--cap-add NET_ADMIN --security-opt no-new-privileges:true \
	--sysctl net.ipv4.ip_forward=1 --sysctl net.ipv4.conf.all.src_valid_mark=1 \
	-v "$WORK/config.yaml:/app/config.yaml:ro" \
	-v "$WORK/node-wg:/etc/wireguard" \
	-v "$WORK/node-data:/var/lib/wgkeeper" \
	-p 127.0.0.1:18081:51821 \
	"$NODE_IMAGE" >/dev/null
wait_for "$NODE_READY"
pass "node starts"

# --- console ----------------------------------------------------------------
docker run -d --name "$NAME-console" --network "$NAME" \
	-e SECRET_KEY="$(openssl rand -hex 32)" \
	-e BOOTSTRAP_ADMIN_PASSWORD="$ADMIN_PASSWORD" \
	-e COOKIE_SECURE=false \
	-e LOG_FORMAT=json \
	-p 127.0.0.1:18080:8000 \
	"$IMAGE" >/dev/null
wait_for "$CONSOLE/readyz"
pass "console starts and /readyz answers"
check "Docker health check passes" bash -c "for _ in \$(seq 1 30); do [ \"\$(docker inspect -f '{{.State.Health.Status}}' $NAME-console)\" = healthy ] && exit 0; sleep 2; done; exit 1"

headers="$(curl -sS -D - -o /dev/null "$CONSOLE/")"
check "the UI is served at /" grep -qi '^content-type: text/html' <<<"$headers"
check "security headers are set" grep -qi '^content-security-policy:' <<<"$headers"
check "responses carry X-Request-Id" grep -qi '^x-request-id:' <<<"$headers"

# --- first login ------------------------------------------------------------
api GET /api/me >/dev/null
check "API needs a session" test "$(status)" = 401
login="$(api POST /api/login "{\"username\":\"admin\",\"password\":\"$ADMIN_PASSWORD\"}")"
check "bootstrap login works and asks for a new password" test "$(jq -r .mustChangePassword <<<"$login")" = true
api GET /api/nodes >/dev/null
check "nodes stay closed until the password changes" test "$(status)" = 403
api POST /api/change-password '{"newPassword":"a-much-better-password"}' >/dev/null
check "password change works" test "$(status)" = 200

# --- node -------------------------------------------------------------------
add="{\"address\":\"http://$NODE_IP:51821\",\"apiKey\":\"$NODE_KEY\"}"
api POST /api/nodes/check "{\"address\":\"http://$NODE_IP:51821\",\"apiKey\":\"wrong\"}" >/dev/null
check "a wrong node API key is reported" test "$(status)" = 422
created="$(api POST /api/nodes "$add")"
NODE_ID="$(jq -r .id <<<"$created")"
check "the node is added" test "$(status)" = 200
nodes="$(api GET /api/nodes)"
check "the node is online with its version" test "$(jq -r '.nodes[0].status + " " + .nodes[0].version' <<<"$nodes")" = "online 2.0.0"
api GET "/api/nodes/$NODE_ID/stats" >/dev/null
check "node stats load" test "$(status)" = 200

# --- config and tunnel ------------------------------------------------------
PEER_ID="$(cat /proc/sys/kernel/random/uuid 2>/dev/null || uuidgen | tr '[:upper:]' '[:lower:]')"
api POST "/api/nodes/$NODE_ID/config?peerId=$PEER_ID&addressFamilies=IPv4" >"$WORK/wg0.conf"
check "a client config is generated" test "$(status)" = 200
check "the config points at the node" grep -q "^Endpoint = $NODE_IP:51820$" "$WORK/wg0.conf"

# Use the generated keys and endpoint as is; only narrow the routes to one
# address and drop DNS, so the test doesn't take over the client's networking.
sed -e 's|^AllowedIPs = .*|AllowedIPs = 1.1.1.1/32|' -e '/^DNS = /d' "$WORK/wg0.conf" >"$WORK/client.conf"
docker run -d --name "$NAME-client" --network "$NAME" \
	--cap-add NET_ADMIN --sysctl net.ipv4.conf.all.src_valid_mark=1 \
	"$ALPINE" sleep infinity >/dev/null
docker exec "$NAME-client" apk add --no-cache --quiet wireguard-tools iproute2
docker exec -i "$NAME-client" sh -c "mkdir -p /etc/wireguard && cat >/etc/wireguard/wg0.conf" <"$WORK/client.conf"
docker exec "$NAME-client" wg-quick up wg0 >/dev/null 2>&1
docker exec "$NAME-client" ping -c 3 -W 2 1.1.1.1 >/dev/null 2>&1 || true
handshake="$(docker exec "$NAME-client" wg show wg0 latest-handshakes | awk '{print $2}')"
check "the generated config completes a WireGuard handshake" test "${handshake:-0}" -gt 0

# --- peers ------------------------------------------------------------------
peers="$(api GET "/api/nodes/$NODE_ID/peers")"
check "the new peer is listed" test "$(jq -r --arg id "$PEER_ID" '[.data[] | select(.peerId == $id)] | length' <<<"$peers")" = 1
api GET "/api/nodes/$NODE_ID/peers/$PEER_ID" >/dev/null
check "peer details load" test "$(status)" = 200
deleted="$(api DELETE "/api/nodes/$NODE_ID/peers?peerId=$PEER_ID")"
check "the peer is deleted on the node" test "$(jq -r .deleted <<<"$deleted")" = true

# --- refresh, delete, logout ------------------------------------------------
api POST /api/nodes/refresh >/dev/null
check "status refresh works" test "$(status)" = 200
api DELETE "/api/nodes/$NODE_ID" >/dev/null
check "the node is removed" test "$(status)" = 200
api POST /api/logout >/dev/null
api GET /api/me >/dev/null
check "logout ends the session" test "$(status)" = 401

if [[ "$failed" -ne 0 ]]; then
	echo "end-to-end test failed"
	exit 1
fi
echo "end-to-end test passed"
