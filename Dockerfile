# Stage 1: Build frontend
FROM node:24-alpine AS frontend-builder

RUN corepack enable && corepack prepare pnpm@10 --activate

WORKDIR /app
COPY frontend/package.json frontend/pnpm-lock.yaml ./frontend/
RUN cd frontend && pnpm install --frozen-lockfile

COPY frontend/ ./frontend/
RUN mkdir -p backend/public && cd frontend && pnpm run build

# Stage 2: Build backend
FROM golang:1.26-alpine AS backend-builder

WORKDIR /app

RUN apk add --no-cache git gcc musl-dev

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ .

RUN go run github.com/swaggo/swag/cmd/swag@v1.16.6 init -g cmd/server/main.go -o ./docs

RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-s -w" -o server ./cmd/server

# Stage 3: Final image
FROM alpine:3.23

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
