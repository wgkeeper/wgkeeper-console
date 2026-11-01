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
cp .env.example .env                       # set SECRET_KEY: openssl rand -hex 32
cd backend && go run ./cmd/server          # API on :8000
cd frontend && pnpm dev                    # UI with hot reload
```

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

## Pull requests

1. Branch from `main` (`feat/…`, `fix/…`, `docs/…`).
2. Keep a pull request to one change, with tests for new behavior and bug fixes.
3. Title it in [Conventional Commits](https://www.conventionalcommits.org/) style: `feat(ui): add peer search`, `fix: refresh node status after restart`. The title becomes the squashed commit on `main`.
4. For UI changes, add a screenshot or a short recording to the pull request.
5. Update the README when you change configuration, deployment or the API.

`main` only accepts pull requests, and `Frontend Checks` and `Backend Checks` must pass before merging.

## Releases (maintainers)

Tag the release commit on `main` as `vX.Y.Z` and push the tag. CI builds and pushes the image to `ghcr.io/wgkeeper/console` as `X.Y.Z`, and as `latest` for stable tags. Tags with a suffix (`v2.0.0-rc.1`) are published without moving `latest`. Every push to `main` also publishes `edge` and `sha-*` images for testing.

## License

By contributing you agree that your contributions are licensed under the [AGPL-3.0](LICENSE), the license of this project.
