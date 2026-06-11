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

# TODO: 运维同步可用的 poppler runtime 镜像后，再切回包含 pdftoppm 的基础镜像。
# 当前 DaoCloud 代理地址 m.daocloud.io/docker.io/minidocks/poppler:latest 不在白名单，
# 先使用公司 Debian runtime 跑通 Wharf -> Kaniko -> 镜像仓库 -> K8s 发布链路。
# FROM m.daocloud.io/docker.io/minidocks/poppler:latest AS runtime
FROM soyoung-registry-vpc.cn-beijing.cr.aliyuncs.com/sy-ops/debian:bookworm-slim AS runtime
WORKDIR /app

# TODO: 恢复 poppler runtime 后打开校验；当前临时镜像不包含 pdftoppm。
# RUN command -v pdftoppm

COPY --from=go-builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=go-builder /out/erzhuang-project /app/erzhuang-project
COPY --from=frontend-builder /src/frontend/dist /app/frontend/dist

ENV ADDR=0.0.0.0:18080 \
    FRONTEND_DIR=/app/frontend/dist \
    UPLOAD_DIR=/app/uploads/design-plan

EXPOSE 18080

ENTRYPOINT ["/app/erzhuang-project"]
