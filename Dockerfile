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
    UPLOAD_DIR=/app/uploads/design-plan \
    DATABASE_URL=postgresql://postgres.aytcxqnlctmhtpwxpfvf:VEOvgITpUXFMeE3A@aws-1-ap-south-1.pooler.supabase.com:5432/postgres \
    OPENAI_API_KEY=sk-82c021b5f90f6a06a7bc7403337b475f1464b4ac59104cfe6f57993dc7880f83 \
    OPENAI_BASE_URL=https://vibe.soyoung.com/ \
    OPENAI_MODEL=gpt-5.5 \
    OPENAI_API_STYLE=responses

EXPOSE 18080

ENTRYPOINT ["/app/erzhuang-project"]
