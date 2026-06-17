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
    OPENAI_API_STYLE=responses \
    EZVIZ_ACCOUNTS_JSON='[{"name":"华北","account_name":"mjdox1","app_key":"675f5728fba047d4a4e96f655d14976a","app_secret":"2e71a11cc5cc69140d7bbd8904a8e816","access_token":"at.2q498utw1h4zk6uc92m015c0572591sg-1761swc8y5-08wmtqu-geq4xscpc"},{"name":"华南","account_name":"l1yv12","app_key":"5e7be7eb1b0b473cb672bc7cb85aeb78","app_secret":"d3212d0c126850dcc1fd57115779e23a","access_token":"at.9mdsh0tjdel19y9l3x9jbeis6swylb43-7p49m7qsow-0x1l3vf-sasi6lssp"},{"name":"华东","account_name":"clpeiu","app_key":"203e82f057554295af6b4804a11d1822","app_secret":"d92b523f469b5c1d62e62be739020171","access_token":"at.cun1idpi8rwsvk870sub6qo6csavlw8a-8dxj68g3c0-15uwgbj-7nahw89td"},{"name":"华中","account_name":"l5e8ry","app_key":"f469a26ecead46709d06fe752554d764","app_secret":"4ee293d5f26dc9b8b5442c898bd71b67","access_token":"at.7x1325jt08eoggmn43dsopch8rz6nc7i-41ypyp27y8-1szpb9w-0tuw1vvgi"}]'

EXPOSE 18080

ENTRYPOINT ["/app/erzhuang-project"]
