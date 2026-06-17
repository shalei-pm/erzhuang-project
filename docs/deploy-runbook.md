# Deploy Runbook

## 发布术语速查

用户说“发布到公司”时，固定执行公司 GitLab 自动发布链路：

1. 将当前已确认代码 merge 到公司 GitLab 固定分支 `codex/containerize-single-image`。
2. 推送到 remote `gitlab`。
3. 等待公司 GitLab / K8s 自动发布，通常约 5 分钟。
4. 验证 `https://lite.sy.soyoung.com/erzhuang-project/health` 和页面版本号。

注意：

- 不操作韩国 Lighthouse。
- 不 force push 公司受保护分支。
- 公司分支如包含 Dockerfile、K8s 环境变量、数据库连接等运维调整，应保留公司配置，只合入业务代码和必要文档。

用户说“发布到韩国服务器”时，固定执行 GitHub + 韩国 Lighthouse 链路：

1. 将当前已确认代码推送到 GitHub `origin/main`。
2. 通过腾讯云 TAT 触发韩国 Lighthouse 服务器执行 `cd /opt/apps/erzhuang-project && ./scripts/deploy.sh`。
3. 服务器从 GitHub 拉取最新 `main`，执行测试、构建、重启服务。
4. 验证 `http://127.0.0.1:18081/health` 和公网 `/erzhuang/` 入口。

如果用户同时要求“公司”和“韩国服务器”，需要记录两个环境最终 commit，避免用户用页面版本号反馈问题时对不上。

本项目的服务器发布目标是个人腾讯云 Lighthouse：

- 部署目录：`/opt/apps/erzhuang-project`
- systemd 服务：`erzhuang-project.service`
- 健康检查：`http://127.0.0.1:18081/health`
- 公网入口：`https://43.155.237.46/erzhuang/`
- GitHub 访问方式：服务器 read-only Deploy Key
- Deploy Key 路径：`~/.ssh/erzhuang_project_deploy_key`

## 发布当前 main

在服务器执行：

```sh
cd /opt/apps/erzhuang-project
./scripts/deploy.sh
```

脚本会执行：

1. 使用 Deploy Key 拉取 `origin/main`。
2. 将本地 `main` 指向 `origin/main`。
3. `go test ./...`。
4. `go build -o erzhuang-project ./cmd/server`。
5. 如果存在 `frontend/package.json` 且服务器有 `npm`，执行 `npm install` 和 `npm run build`。
6. `sudo systemctl restart erzhuang-project.service`。
7. `curl -fsS http://127.0.0.1:18081/health`。

如果任一步失败，脚本会停止。

## Codex 通过 TAT 发布

主会话可以直接通过腾讯云 TAT 触发服务器发布，不需要用户手动登录服务器。

固定目标：

- Region: `ap-seoul`
- InstanceId: `lhins-rjfpwj1u`
- Username: `lighthouse`
- Server command: `cd /opt/apps/erzhuang-project && ./scripts/deploy.sh`

在 Codex 中执行时，优先使用本机环境变量读取腾讯云密钥：

- `TENCENTCLOUD_SECRET_ID`
- `TENCENTCLOUD_SECRET_KEY`

如果本机环境变量不存在，`tools/tat_run.py` 会用 `getpass` 提示输入腾讯云 `SecretId` 和 `SecretKey`。不要把密钥拼进命令，不要提交到 Git；项目 `.gitignore` 已忽略 `.env` 和 `.env.*`。

命令：

```sh
python3 tools/tat_run.py --region ap-seoul --instance-id lhins-rjfpwj1u --timeout 900 --username lighthouse "cd /opt/apps/erzhuang-project && ./scripts/deploy.sh"
```

预期成功信号：

- `TaskStatus: SUCCESS`
- 输出包含 `Deploy complete`
- 当前 commit 与 GitHub `main` 一致
- `go test ./...` 成功
- Go build 成功
- 前端 build 成功
- `curl http://127.0.0.1:18081/health` 返回健康 JSON

如果健康检查失败，不要立刻重复发布。先执行只读诊断：

```sh
python3 tools/tat_run.py --region ap-seoul --instance-id lhins-rjfpwj1u --timeout 120 --username lighthouse "cd /opt/apps/erzhuang-project && echo COMMIT=$(git rev-parse --short HEAD) && echo VERSION=$(cat VERSION) && sudo systemctl status erzhuang-project.service --no-pager || true && echo '--- journal ---' && sudo journalctl -u erzhuang-project.service -n 80 --no-pager && echo '--- listeners ---' && ss -ltnp | grep -E '18081|18080' || true && echo '--- health 18081 ---' && curl -sv http://127.0.0.1:18081/health || true"
```

常见失败：

- `database setup failed: timeout: context deadline exceeded`：数据库 schema 初始化超时。应检查 `cmd/server/main.go` 中 schema 初始化超时是否足够，修复后重新发布。
- `git@github.com: Permission denied (publickey)`：服务器未使用 deploy key。检查 `scripts/deploy.sh` 中 `GIT_SSH_COMMAND`。
- `curl 127.0.0.1:18081 failed` 且 journal 无监听日志：服务启动失败，先看 journal，不要只看 systemd 刚启动的一瞬间状态。

## 服务器依赖

设计图上传识别需要额外依赖：

```sh
sudo apt-get update
sudo apt-get install -y poppler-utils
```

用途：

- `pdftoppm`：把用户上传的 PDF 图纸转换成 PNG 预览图。

systemd 环境文件 `/etc/erzhuang-project.env` 需要包含：

```text
ASSET_STORE=local
UPLOAD_DIR=/opt/apps/erzhuang-project/uploads
OPENAI_API_KEY=...
OPENAI_MODEL=gpt-4o
OPENAI_BASE_URL=https://api.openai.com
OPENAI_API_STYLE=responses
```

萤石云录像机扫描需要配置：

```text
EZVIZ_ACCOUNTS_JSON=[{"name":"华北","app_key":"...","app_secret":"...","access_token":"..."},{"name":"华东","app_key":"...","app_secret":"..."},{"name":"华南","app_key":"...","app_secret":"..."},{"name":"华中","app_key":"...","app_secret":"..."}]
```

运行规则：

- `name` 必须使用前端需要展示的区域名，例如 `华北`、`华东`、`华南`、`华中`。
- 服务启动时会读取 `EZVIZ_ACCOUNTS_JSON`，自动把这些 `name` 同步到数据库 `ezviz_accounts`，状态设为 `available`。
- 数据库只保存区域账号展示记录；扫描、抓图仍使用运行时环境变量里的 `app_key` / `app_secret` / `access_token`。
- 公司内网环境当前允许把该变量临时写入内网 GitLab Dockerfile 做验证；长期建议迁移到 K8s Secret 或受保护 CI/CD Variables。

注意：

- `OPENAI_API_KEY` 只放服务器环境文件，不提交到 GitHub。
- 本地文件模式下，`uploads` 目录需要允许运行服务的 `lighthouse` 用户写入，并建议做持久化备份。
- 本地模式会兼容历史数据库路径：数据库里保存 `uploads/tmp_xxx/preview.png`，实际磁盘路径是 `UPLOAD_DIR/tmp_xxx/preview.png`。

公司 K8s 环境建议使用 Supabase Storage 存放设计图和通道截图：

```text
ASSET_STORE=supabase
SUPABASE_URL=https://<project-ref>.supabase.co
SUPABASE_SERVICE_ROLE_KEY=...
SUPABASE_STORAGE_BUCKET=design-plan-assets
UPLOAD_DIR=/tmp/erzhuang-work
```

注意：

- `SUPABASE_SERVICE_ROLE_KEY` 只能放后端 K8s Secret，不允许进入仓库、镜像或前端 `VITE_*` 变量。
- `UPLOAD_DIR` 在 Supabase Storage 模式下只作为 PDF 渲染临时工作目录，最终 `original.pdf`、`preview.png`、`thumbnail.png` 和通道截图会通过后端写入 Supabase Storage。
- Supabase bucket 推荐设为 private，由 Go 后端统一读取和转发，不让前端直连 Storage。
- Supabase Storage 对象 key 约定：
  - 设计图：`uploads/{upload_id}/original.pdf`、`uploads/{upload_id}/preview.png`、`uploads/{upload_id}/thumbnail.png`。
  - 通道截图：`channel-snapshots/{snapshot_name}.jpg`。

## 前端发布方向

当前前端工程使用 Vite + React + TypeScript，构建产物目录为：

```text
frontend/dist
```

Vite 已配置 `base: "/erzhuang/"`，适合部署在公网路径：

```text
https://43.155.237.46/erzhuang/
```

当前 nginx 接入方式：

- `/erzhuang/` 返回 `frontend/dist` 静态页面。
- `/erzhuang/api/` 反向代理到 Go 后端 `127.0.0.1:18081`。
- `/erzhuang/health` 反向代理到 Go 后端 `127.0.0.1:18081/health`，保留健康检查兼容路径。

当前部署脚本已支持在服务器存在 `npm` 时自动构建前端；如果服务器尚未安装 Node/npm，则会跳过前端构建，不影响 Go 后端发布。

公网验证：

```sh
curl -k https://43.155.237.46/erzhuang/
curl -k https://43.155.237.46/erzhuang/health
curl -k https://43.155.237.46/erzhuang/api/tasks
```

## 回滚到指定 commit 或 tag

在服务器执行：

```sh
cd /opt/apps/erzhuang-project
./scripts/rollback.sh <commit-or-tag>
```

例子：

```sh
./scripts/rollback.sh fbdb249
```

脚本会执行：

1. 使用 Deploy Key 拉取远程 refs 和 tags。
2. `git checkout <commit-or-tag>`。
3. `go test ./...`。
4. `go build -o erzhuang-project ./cmd/server`。
5. `sudo systemctl restart erzhuang-project.service`。
6. `curl -fsS http://127.0.0.1:18081/health`。

回滚到 commit 会让服务器进入 detached HEAD 状态。临时回滚可以接受；后续再用 `./scripts/deploy.sh` 可恢复到最新 `main` 并重新发布。

## 常用检查

```sh
git status -sb
git log --oneline -5
systemctl status erzhuang-project.service --no-pager
curl -s http://127.0.0.1:18081/health
curl -k https://43.155.237.46/erzhuang/health
```

## 安全边界

- 不使用腾讯云主账号。
- 不复制个人 GitHub SSH 私钥到服务器。
- 服务器只使用仓库级 read-only Deploy Key 拉取代码。
- nginx 只维护 `/erzhuang/`、`/erzhuang/health`、`/erzhuang/api/` 项目路径，不改 `/vless`。
- 默认不开放公网端口。
- 默认只重启 `erzhuang-project.service`。
