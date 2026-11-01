# Security

To report a vulnerability, see [SECURITY.md](../SECURITY.md). Please don't open a public issue.

The console holds the API key of every node it manages and can create, rotate and delete peers on all of them. Treat access to it like access to the VPN itself.

| Mechanism | Details |
|-----------|---------|
| Passwords | bcrypt. The bootstrap password must be changed at the first login, and until then the API refuses everything except the password change. A login for an unknown user takes as long as one with a wrong password, so usernames can't be guessed by timing |
| Sessions | An HMAC-signed token in an `HttpOnly` cookie, valid for `SESSION_TTL` and refreshed while in use, never beyond 7 days after login. Logging out or changing the password ends every session of the user, including copies of the cookie |
| Cookies | `Secure` in production, `SameSite=Lax` by default. The console refuses to start with `SameSite=None` without `Secure` |
| Cross-site requests | On top of `SameSite`, state-changing requests are refused when the browser says they come from another site (`Sec-Fetch-Site`, or `Origin` against `Host`), and request bodies must be JSON. Generating a config is a `POST`, since it creates or rotates the peer on the node |
| Login rate limit | 10 failed logins per client in 15 minutes, then a 15-minute block. Attempts are reserved before the password is checked, so parallel requests can't get around it; IPv6 clients are counted per /64. Behind a proxy, see [`TRUSTED_PROXIES`](configuration.md#behind-a-reverse-proxy) |
| API rate limit | 120 requests per minute per client on authenticated routes |
| Node API keys | Encrypted with AES-256-GCM, with a key derived from `SECRET_KEY`, on every write. Never returned by the API |
| Requests to nodes | Redirects are never followed, so a node or proxy can't send the API key to another host. Responses are size-limited. Addresses with credentials (`https://user:pass@...`) are refused. The console never connects to link-local addresses (`169.254.0.0/16`, `fe80::/10`), cloud metadata endpoints (`169.254.169.254`, `fd00:ec2::254`, `100.100.100.200`), multicast or unspecified addresses, checked after DNS resolution, so a stolen admin session can't reach the host's cloud credentials through it. Loopback and private networks stay allowed. With `HTTP_PROXY` set, the proxy decides where requests may go |
| Secrets in responses | Peer private keys are returned only in the generated config; every API response has `Cache-Control: no-store` |
| Error details | Logged with the request ID, never returned to the client. Logs never contain request headers, cookies or SQL values |
| Security headers | Content-Security-Policy, `X-Frame-Options: DENY`, `X-Content-Type-Options`, `Referrer-Policy`, `Permissions-Policy`, and HSTS in production |
| Body limit | 1 MB; larger requests are refused |
| Container | Runs as a non-root user with `no-new-privileges`; the port is published on loopback only |
| Files on disk | The SQLite directory is created with mode `0750` |

## Running it safely

- **Use HTTPS** for anything but a loopback or SSH-tunnel connection, and set `COOKIE_SECURE=true` (the [proxy examples](deployment.md#behind-a-reverse-proxy) do).
- **Keep it private.** The console doesn't need to be public: put it behind a VPN or an IP allowlist in the proxy.
- **Back up `SECRET_KEY`** together with the database. A new key can't decrypt the stored node API keys.
- **Plain-HTTP nodes:** the console accepts `http://` node addresses, since a node's API is plain HTTP by default. Over an untrusted network, the node API key and peer private keys then travel in the clear; put the node behind HTTPS (see the node's deployment docs) or reach it over a private network.
