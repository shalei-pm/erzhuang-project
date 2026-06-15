# syntax=docker/dockerfile:1

FROM soyoung-registry-vpc.cn-beijing.cr.aliyuncs.com/sy-system/exec-node:23.11.1-alpine AS frontend-builder
WORKDIR /src/frontend

COPY frontend/package*.json ./
RUN npm ci

COPY testdata/ /src/testdata/
COPY frontend/ ./
RUN npm run build

FROM soyoung-registry-vpc.cn-beijing.cr.aliyuncs.com/sy-system/exec-go:1.22-bullseye AS go-builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . ./
RUN go test ./...
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/erzhuang-project ./cmd/server

FROM soyoung-registry-vpc.cn-beijing.cr.aliyuncs.com/sy-system/minidocks/poppler:latest AS runtime
WORKDIR /app

RUN command -v pdftoppm

COPY --from=go-builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=go-builder /out/erzhuang-project /app/erzhuang-project
COPY --from=frontend-builder /src/frontend/dist /app/frontend/dist

ENV ADDR=0.0.0.0:18080 \
    APP_BASE_PATH=/erzhuang-project \
    FRONTEND_DIR=/app/frontend/dist \
    UPLOAD_DIR=/app/uploads/design-plan

EXPOSE 18080

ENTRYPOINT ["/app/erzhuang-project"]
