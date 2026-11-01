# Metrics and logs

## Health checks

| Endpoint | Answers |
|----------|---------|
| `GET /healthz` | `200` while the process is up. The Docker image uses it as its `HEALTHCHECK` |
| `GET /readyz` | `200` when the database is reachable, `503` when it isn't. The Compose file checks this one |

Both sit outside authentication and rate limits, and are not written to the request log.

## Logs

The console logs to stdout: one line per request, plus a line for every error. `LOG_FORMAT=json` writes one JSON object per line for log collectors.

Every response carries an `X-Request-Id` header, and every log line written while handling that request has the same `request_id`. A failed request in the browser's network tab leads straight to its log lines. A client-supplied `X-Request-Id` is kept only if it is a UUID v4.

Request lines are logged at error level for `5xx`, warning for `4xx` (except `404` for paths the console doesn't have, which is scanner noise), and info otherwise. Static files that load fine are debug.

## Prometheus metrics

The console can serve Prometheus metrics at `/metrics` on a separate port. It is off until `METRICS_PORT` is set. The endpoint requires a bearer token that has nothing to do with the console's logins and is rate limited to 5 requests per second:

```bash
METRICS_PORT=9100
METRICS_TOKEN=<at least 32 characters, e.g. openssl rand -hex 32>
```

With `METRICS_PORT` set, a token of at least 32 characters is required, or the console refuses to start.

| Metric | Type | Labels | Use |
|--------|------|--------|-----|
| `wgkeeper_console_nodes` | gauge | `status` = `online`, `offline` | Stored nodes by their last known status |
| `wgkeeper_console_node_requests_total` | counter | `op`, `result` | Calls to nodes and the release check. Alert on `error_unreachable` and `error_unauthorized` |
| `wgkeeper_console_node_request_duration_seconds` | histogram | `op` | How fast nodes answer |
| `wgkeeper_console_http_requests_total` | counter | `method`, `path`, `status_class` | API traffic and error rate |
| `wgkeeper_console_http_request_duration_seconds` | histogram | `method`, `path` | API latency |
| `process_*`, `go_*` | | | Standard process and Go runtime metrics |

`op` is one of `health`, `stats`, `list_peers`, `get_peer`, `create_peer`, `delete_peer`, `release_check`; `result` is `success`, `error_unauthorized`, `error_upstream` or `error_unreachable`. The `path` label is the route template, such as `/api/nodes/:id/peers`, so node and peer IDs don't create new series; unknown routes are counted as `path="unmatched"`.

Ready-made configs are in [`examples/prometheus/`](../examples/prometheus): a [scrape config](../examples/prometheus/prometheus.yml) and [alerting rules](../examples/prometheus/alerts.yml) for a down console, offline nodes, a rejected node API key, unreachable nodes and server errors.

For the nodes' own metrics (peers, traffic, handshakes), scrape the nodes directly; see the node's metrics docs.

## Keep the metrics port private

The metrics endpoint listens on all interfaces inside the container. Don't publish it on the host: run Prometheus in the same Docker network and scrape `console:9100`. If Prometheus runs elsewhere, put a separate proxy site with client-certificate (mTLS) auth in front of it.
