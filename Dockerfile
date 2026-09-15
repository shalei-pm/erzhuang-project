# syntax=docker/dockerfile:1

FROM soyoung-registry-vpc.cn-beijing.cr.aliyuncs.com/sy-system/exec-node:23.11.1-alpine AS frontend-builder
WORKDIR /src/frontend

ARG GIT_VERSION=container
ARG VITE_APP_VERSION=

COPY frontend/package*.json ./
RUN npm ci

COPY VERSION /src/VERSION
COPY testdata/ /src/testdata/
COPY frontend/ ./
RUN PRODUCT_VERSION="$(cat /src/VERSION)" \
    && APP_VERSION="${VITE_APP_VERSION:-${PRODUCT_VERSION} (${GIT_VERSION})}" \
    && VITE_APP_VERSION="${APP_VERSION}" npm run build

FROM soyoung-registry-vpc.cn-beijing.cr.aliyuncs.com/sy-system/exec-go:1.22-bullseye AS go-builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . ./
RUN go test ./...
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o /out/erzhuang-project ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/nvr-snapshot-backfill ./cmd/nvr-snapshot-backfill
RUN mkdir -p /out/lib \
    && qconf_path="$(ldd /out/erzhuang-project | awk '/libqconf/{print $3; exit}')" \
    && if [ -n "${qconf_path}" ]; then cp -L "${qconf_path}" /out/lib/; fi

FROM soyoung-registry-vpc.cn-beijing.cr.aliyuncs.com/sy-system/minidocks/poppler:latest AS runtime
WORKDIR /app

ENV APP_BASE_PATH=/erzhuang-project
ENV FRONTEND_DIR=/app/frontend/dist

RUN command -v pdftoppm

COPY --from=go-builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=go-builder /out/erzhuang-project /app/erzhuang-project
COPY --from=go-builder /out/nvr-snapshot-backfill /app/nvr-snapshot-backfill
COPY --from=go-builder /out/lib /app/lib
COPY --from=frontend-builder /src/frontend/dist /app/frontend/dist

ENV LD_LIBRARY_PATH=/app/lib

EXPOSE 18080

ENTRYPOINT ["/app/erzhuang-project"]
