# 单镜像容器化说明

本项目提供一个基础 `Dockerfile`，用于把前端构建产物和 Go 后端打包进同一个运行镜像，方便后续在本地 Docker 或 K8s 中验证。本文不包含真实公司环境、密钥或镜像仓库地址。

## 镜像内容

- `frontend-builder` 阶段：使用 Node 构建 `frontend/`，产物为 `frontend/dist`；构建时会复制 `testdata/`，因为当前前端 mock 数据引用了 `testdata/design-plans/generated/sample-store-floor-plan.png`。
- `go-builder` 阶段：执行 `go test ./...`，并构建 `./cmd/server`。
- `runtime` 阶段：当前临时基于公司内网镜像 `soyoung-registry-vpc.cn-beijing.cr.aliyuncs.com/sy-ops/debian:bookworm-slim`，用于先跑通 Wharf -> Kaniko -> 镜像仓库 -> K8s 发布链路。
- PDF 转图片依赖 `pdftoppm`；当前 runtime 临时镜像不包含该工具，等运维同步可用的 poppler runtime 镜像后再恢复。
- 运行镜像内路径：
  - 后端二进制：`/app/erzhuang-project`
  - 前端产物：`/app/frontend/dist`
  - 默认上传目录：`/app/uploads/design-plan`
- Go 服务会读取 `APP_BASE_PATH` 和 `FRONTEND_DIR`，默认把前端产物挂到 `/erzhuang-project`，并兼容 `/erzhuang-project/api/...` 到后端 `/api/...`、`/erzhuang-project/health` 到后端 `/health` 的转发前缀。
- Dockerfile 中构建阶段基础镜像使用公司内网镜像地址：`soyoung-registry-vpc.cn-beijing.cr.aliyuncs.com/sy-system/exec-node:23.11.1-alpine`、`soyoung-registry-vpc.cn-beijing.cr.aliyuncs.com/sy-system/exec-go:1.22-bullseye`。
- 运行阶段暂用公司 Debian 镜像，不在 Kaniko 构建时访问 Debian apt 源安装 `poppler-utils`。
- 已尝试 Docker Hub 镜像 `minidocks/poppler:latest` 的 DaoCloud 大陆代理地址 `m.daocloud.io/docker.io/minidocks/poppler:latest`，但 DaoCloud 返回 `this image is not in the allowlist`，需等运维同步到公司内网镜像或配置可用白名单后再切换。

## 本地构建

默认镜像名为 `erzhuang-project`，默认标签为当前 Git short sha：

```sh
./scripts/build-image.sh
```

自定义镜像名、标签、Dockerfile 或构建上下文：

```sh
IMAGE_NAME=erzhuang-project IMAGE_TAG=local \
  DOCKERFILE=Dockerfile CONTEXT=. \
  ./scripts/build-image.sh
```

脚本会先输出实际执行的 `docker build` 命令，再开始构建。

## 本地运行

容器内默认监听 `0.0.0.0:18080`，本地可映射到同端口：

```sh
docker run --rm \
  -p 18080:18080 \
  -e ADDR=0.0.0.0:18080 \
  erzhuang-project:local
```

验证后端健康检查：

```sh
curl http://127.0.0.1:18080/health
```

如果需要持久化上传文件，建议把上传目录挂载为 volume：

```sh
docker run --rm \
  -p 18080:18080 \
  -e ADDR=0.0.0.0:18080 \
  -e UPLOAD_DIR=/app/uploads/design-plan \
  -v "$(pwd)/uploads:/app/uploads" \
  erzhuang-project:local
```

## 环境变量

| 变量 | 必填 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `ADDR` | 否 | `0.0.0.0:18080` | 服务监听地址；容器内应监听 `0.0.0.0`。 |
| `APP_BASE_PATH` | 否 | `/erzhuang-project` | 应用对外路径前缀；当前公司域名下配置为 `/erzhuang-project`。 |
| `DATABASE_URL` | 否 | 空 | PostgreSQL 连接串；为空时使用内存存储。不要写入镜像或 Git。 |
| `FRONTEND_DIR` | 否 | `/app/frontend/dist` | 前端生产构建产物目录，镜像内已预置。 |
| `UPLOAD_DIR` | 否 | `/app/uploads/design-plan` | 设计图上传和转换文件目录，生产建议挂载持久化存储。 |
| `OPENAI_API_KEY` | 视功能而定 | 空 | AI 识别接口密钥；必须通过运行时 Secret 注入。 |
| `OPENAI_MODEL` | 否 | `gpt-4o` | AI 识别模型。 |
| `OPENAI_BASE_URL` | 否 | `https://api.openai.com` | 兼容 OpenAI API 的服务地址。 |
| `OPENAI_API_STYLE` | 否 | `responses` | API 风格，兼容网关可按代码支持值配置。 |

## K8s 对接注意事项

- 镜像构建和推送时使用占位镜像仓库地址，真实仓库地址不要写入文档或代码。
- `DATABASE_URL`、`OPENAI_API_KEY` 等敏感配置应使用 K8s `Secret` 注入，不要做成镜像层、ConfigMap 明文或 Git 文件。
- `UPLOAD_DIR` 对应目录建议挂载 PVC，避免 Pod 重建后上传文件丢失。
- K8s 容器内 readiness/liveness probe 可使用 `GET /health`，端口为容器端口 `18080`。
- 经过公司域名/Ingress 的外部健康检查可使用 `GET /erzhuang-project/health`。
- 资源限制需要覆盖 PDF 转图片场景；`pdftoppm` 对大 PDF 可能有瞬时 CPU 和内存消耗。
- 当前镜像已包含 `/app/frontend/dist`，Go 服务会直接提供 `/erzhuang-project` 前端页面。
- 公司网关/Ingress 当前 path 配置建议为 `/erzhuang-project`，不依赖 rewrite/strip path。
- 如果未来新增其他项目，例如 `/aaa-project`，建议对应项目也使用自己的 base path，并让应用代码、前端 `base`、默认 API base、`APP_BASE_PATH` 保持一致。

## 公司发布系统流水线建议

截图中的现有流水线更像“纯前端项目模板”：先用 Node 镜像执行 `npm/pnpm` 构建，再由 `dockerfile-generator` 生成 nginx Dockerfile，最后用 `kaniko-executor` 构建 nginx 静态站点镜像。

本项目不是纯前端项目，而是前后端一体镜像：

- 前端：Vite + React，构建产物在镜像内的 `/app/frontend/dist`。
- 后端：Go HTTP 服务，容器启动后运行 `/app/erzhuang-project`。
- 运行时依赖：`poppler-utils` 提供 `pdftoppm`，用于 PDF 转图片。
- 数据库：不在镜像里，运行时通过 `DATABASE_URL` 连接外部 PostgreSQL。

因此，不建议直接套纯前端流水线模板。推荐流水线结构：

```yaml
steps:
  - name: git-clone
    remark: 从 Git 仓库克隆代码
    template: git-clone

  # 可选：只做轻量检查，不在这里做最终 npm/go 构建。
  # 真正的前端构建、Go 测试和 Go 编译由仓库根目录 Dockerfile 完成。
  - name: precheck
    remark: 检查仓库文件
    template: exec
    values:
      script: |-
        set -ex
        pwd
        ls -la
        test -f Dockerfile
        test -f go.mod
        test -f frontend/package.json

  # 不要使用纯前端模板生成 nginx Dockerfile。
  # 如果发布系统有 dockerfile-generator 步骤，建议删除或禁用，
  # 让后续 kaniko-executor 使用代码仓库中的 Dockerfile。

  - name: build
    remark: 使用仓库 Dockerfile 构建并推送前后端一体镜像
    template: kaniko-executor
    values:
      dockerfile: Dockerfile
      build-args: "[]"
      destinations: []
```

发布系统配置重点：

- 使用仓库根目录的 `Dockerfile`，不要生成 nginx 静态站点 Dockerfile。
- `kaniko-executor` 的构建上下文应为仓库根目录。
- 镜像运行端口为 `18080`，不是纯前端 nginx 常见的 `80`。
- K8s Service/Ingress 应转发到容器端口 `18080`。
- 容器内健康检查使用 `GET /health`；域名外部健康检查使用 `GET /erzhuang-project/health`。
- 页面路径默认为 `/erzhuang-project`；接口路径默认为 `/erzhuang-project/api/...`，Go 服务会转到内部 `/api/...`。

运行时环境变量建议：

| 变量 | 建议来源 | 说明 |
| --- | --- | --- |
| `ADDR` | 普通环境变量 | 建议固定为 `0.0.0.0:18080`。 |
| `APP_BASE_PATH` | 普通环境变量 | 建议固定为 `/erzhuang-project`；需要和发布系统域名 path 配置一致。 |
| `DATABASE_URL` | K8s Secret | 外部 PostgreSQL 连接串。 |
| `OPENAI_API_KEY` | K8s Secret | 如需 AI 识别图纸时配置。 |
| `OPENAI_MODEL` | 普通环境变量 | 可选，不配时使用代码默认值。 |
| `OPENAI_BASE_URL` | 普通环境变量或 Secret | 如走公司兼容网关，按公司地址配置。 |
| `OPENAI_API_STYLE` | 普通环境变量 | 如兼容网关需要，可设置为代码支持的风格。 |
| `UPLOAD_DIR` | 普通环境变量 | 建议 `/app/uploads/design-plan`，并挂载 PVC。 |

如果发布系统必须保留 `exec` 脚本步骤，也不要在该步骤里执行 `docker build`。截图中的镜像构建由 `kaniko-executor` 负责，`exec` 步骤通常没有 Docker daemon。`scripts/build-image.sh` 更适合本地或普通服务器验证；公司流水线优先使用 `kaniko-executor + 仓库 Dockerfile`。
