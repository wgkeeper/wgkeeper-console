# Stage 1: Build frontend
FROM node:26-alpine@sha256:c6ad9e9c1d80f6eb607c26b4b0a132ff7a2344fad1c78ea3baa2a7d4c8de6d94 AS frontend-builder

# Node 25+ no longer ships corepack, so install pnpm with npm.
RUN npm install -g pnpm@12.16.1

WORKDIR /app
COPY frontend/package.json frontend/pnpm-lock.yaml ./frontend/
RUN cd frontend && pnpm install --frozen-lockfile

COPY frontend/ ./frontend/
RUN mkdir -p backend/public && cd frontend && pnpm run build

# Stage 2: Build backend
FROM golang:1.27-alpine@sha256:d200d2dd76b03a99b52651fd1325aff7dcd35422c35c01bac56571b0507f3deb AS backend-builder

WORKDIR /app

RUN apk add --no-cache git gcc musl-dev

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ .

RUN go run github.com/swaggo/swag/cmd/swag@v1.16.6 init -g cmd/server/main.go -o ./docs

RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-s -w" -o server ./cmd/server

# Stage 3: Final image
FROM alpine:3.24@sha256:be2656061b536c1883c4034790e21d0a09c5aaaf51968b89368a3c0371ea2c9a

WORKDIR /app

RUN apk add --no-cache sqlite ca-certificates

RUN addgroup -S appgroup && adduser -S appuser -G appgroup

COPY --from=backend-builder /app/server .
COPY --from=frontend-builder /app/backend/public ./public

RUN mkdir -p /app/data && chown -R appuser:appgroup /app

USER appuser

ENV PORT=8000
ENV DATABASE_URL=file:/app/data/wgkeeper-console.db

EXPOSE 8000

CMD ["./server"]
