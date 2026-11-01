# Security policy

wgkeeper console is the admin panel for a fleet of wgkeeper nodes. It stores the API key of every node it manages (encrypted with `SECRET_KEY`), admin accounts and their sessions, and it can create, rotate and delete WireGuard peers on all of those nodes. We treat security issues as the highest priority.

Examples of what we consider a vulnerability:

- logging in, or using the API, without valid credentials; bypassing the forced password change after the first login;
- session fixation, theft or forgery, or a session that survives logout or a password change;
- reading node API keys, peer private keys or preshared keys beyond what an admin is shown on purpose, or recovering stored node API keys without `SECRET_KEY`;
- cross-site scripting, CSRF or clickjacking in the web UI;
- bypassing the per-IP rate limit on login and the API, including by spoofing `X-Forwarded-For` when `TRUSTED_PROXIES` is not set;
- SQL injection or any way to change data you should not reach, on SQLite or PostgreSQL;
- a shipped deployment file (`docker-compose.yml`, `Caddyfile`, the README examples) that exposes the app over plain HTTP or with insecure cookies beyond what it documents;
- a vulnerable dependency or base image that is reachable in the shipped binary, frontend bundle or container.

## Reporting a vulnerability

Report privately through GitHub: **Security → Report a vulnerability** on [wgkeeper/wgkeeper-console](https://github.com/wgkeeper/wgkeeper-console/security/advisories/new). Please do not open a public issue, pull request or discussion.

Include the version or image tag, how the console is deployed (Docker, behind a proxy, SQLite or PostgreSQL), the steps to reproduce and the impact you see. Remove real passwords, `SECRET_KEY`, node API keys and session cookies first.

What to expect:

- an acknowledgement within 3 working days;
- an assessment and a fix plan within 10 working days;
- a patch release and a published GitHub Security Advisory, crediting you unless you prefer otherwise.

## Supported versions

| Version | Supported |
| --- | --- |
| latest release | yes |
| older releases | no, please upgrade |
| `edge` and `sha-*` images | no, built from `main` for testing |
