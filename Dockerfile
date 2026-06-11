# syntax=docker/dockerfile:1

FROM node:22-bookworm-slim AS frontend-builder
WORKDIR /src/frontend

COPY frontend/package*.json ./
RUN npm ci

COPY frontend/ ./
RUN npm run build

FROM golang:1.22-bookworm AS go-builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . ./
RUN go test ./...
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/erzhuang-project ./cmd/server

FROM debian:bookworm-slim AS runtime
WORKDIR /app

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates poppler-utils \
    && rm -rf /var/lib/apt/lists/*

COPY --from=go-builder /out/erzhuang-project /app/erzhuang-project
COPY --from=frontend-builder /src/frontend/dist /app/frontend/dist

ENV ADDR=0.0.0.0:18080 \
    FRONTEND_DIR=/app/frontend/dist \
    UPLOAD_DIR=/app/uploads/design-plan

EXPOSE 18080

ENTRYPOINT ["/app/erzhuang-project"]
