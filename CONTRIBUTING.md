# Contributing

Thanks for helping with wgkeeper console. Bug reports, fixes and docs improvements are welcome. For a new feature or a change in behavior, please open an issue first so we can agree on the approach before you write code.

Security issues go through [SECURITY.md](SECURITY.md), not public issues.

## Setup

The repository has two parts: a Go backend in `backend/` and a React frontend in `frontend/`. In production the frontend is built into `backend/public` and served by the backend.

You need:

- Go (the version in `backend/go.mod`);
- Node.js and pnpm (the versions in `engines` in `frontend/package.json`);
- [golangci-lint](https://golangci-lint.run/) (the version pinned in `.github/workflows/ci.yml`) and [lefthook](https://github.com/evilmartians/lefthook).

```bash
cd frontend && pnpm install
lefthook install   # pre-commit: format, lint, types, gofmt, go vet; pre-push: builds, tests, golangci-lint, govulncheck, go mod tidy
```

Run it locally:

```bash
cd backend && APP_ENV=development BOOTSTRAP_ADMIN_PASSWORD=dev-password go run ./cmd/server   # API on :8000
cd frontend && pnpm dev                                                                       # UI on :5173, proxies /api
```

In development the console uses a temporary `SECRET_KEY` when none is set, so sessions end on every restart; set `SECRET_KEY` (`openssl rand -hex 32`) to keep them. Every variable is in [docs/configuration.md](docs/configuration.md).

## Checks

CI runs the same checks; running them locally saves a round trip.

Frontend:

```bash
cd frontend
pnpm format:check
pnpm lint
pnpm exec tsc -b
pnpm test
pnpm build
```

Backend:

```bash
cd backend
go run github.com/swaggo/swag/cmd/swag@v1.16.6 init -g cmd/server/main.go -o ./docs   # once; docs/ is generated
go vet ./...
go test -race ./...
golangci-lint run
govulncheck ./...
go mod tidy -diff
```

`go test` runs the fuzz seed corpus too. CI also fuzzes every `Fuzz*` target for 15 seconds; to dig into one locally:

```bash
go test -run='^$' -fuzz=FuzzVerifyToken -fuzztime=1m ./internal/auth/
```

Benchmarks for `internal/auth`, `internal/middleware`, `internal/models` and `internal/services` run on every pull request, and the comparison with `main` is posted as a comment:

```bash
go test -bench=. -benchmem -run='^$' ./internal/auth/ ./internal/middleware/ ./internal/models/ ./internal/services/
```

The PostgreSQL integration test needs a running server:

```bash
docker run -d --name wgk-pg -e POSTGRES_USER=console -e POSTGRES_PASSWORD=console -e POSTGRES_DB=console -p 5432:5432 postgres:18-alpine
WGKEEPER_TEST_POSTGRES_URL="postgres://console:console@127.0.0.1:5432/console?sslmode=disable" \
  go test -tags=integration -run TestPostgres ./internal/database/
```

The end-to-end test runs the console image against a real wgkeeper-node: it adds the node through the console API, generates a client config and checks that a WireGuard client completes a handshake with it, then goes through peers, refresh and logout. It needs Docker on a kernel with WireGuard (Linux 5.6+ or Docker Desktop), `curl` and `jq`:

```bash
E2E_BUILD=1 test/e2e/run.sh   # build the image, then test it
E2E_KEEP=1 test/e2e/run.sh    # keep the wgkc-e2e-* containers to debug a failure
```

Run it when you touch the Dockerfile, startup, static file serving or how the console talks to nodes. CI runs it on every pull request, and the image is only published if it passes. The node version it tests against is pinned in `test/e2e/run.sh`; bump it with each node release.

## Pull requests

1. Branch from `main` (`feat/…`, `fix/…`, `docs/…`).
2. Keep a pull request to one change, with tests for new behavior and bug fixes.
3. Title it in [Conventional Commits](https://www.conventionalcommits.org/) style: `feat(ui): add peer search`, `fix: refresh node status after restart`. The title becomes the squashed commit on `main`.
4. For UI changes, add a screenshot or a short recording to the pull request.
5. Update `docs/` when you change configuration, deployment or metrics.

`main` only accepts pull requests, and the CI checks must pass before merging.

## Releases (maintainers)

Tag the release commit on `main` as `vX.Y.Z` and push the tag. CI builds and pushes the image to `ghcr.io/wgkeeper/console` as `X.Y.Z`, and as `latest` for stable tags. Tags with a suffix (`v2.0.0-rc.1`) are published without moving `latest`. Every push to `main` also publishes `edge` and `sha-*` images for testing.

## License

By contributing you agree that your contributions are licensed under the [AGPL-3.0](LICENSE), the license of this project.
