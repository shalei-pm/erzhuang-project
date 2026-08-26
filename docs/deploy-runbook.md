# Deploy Runbook

## 公司 GitLab 自动发布

公司环境已经接入 GitLab + K8s 自动发布。后续二壮项目发布只走本节流程。韩国 Lighthouse 发布链路已经终止，且该服务器上关于二壮项目的库表已经完全删除；不得再作为发布、回滚、排查或备用验证路径。

固定信息：

- GitLab remote：`gitlab`
- GitLab 仓库：`https://gitlab.sy.soyoung.com/pm/shalei-pm/erzhuang-project.git`
- 测试发布分支：`codex/containerize-single-image`
- 测试 Wharf pipeline：`https://wharf.sy.soyoung.com/dev/app/pm/erzhuang-project/build?page=1&pageSize=20&pipeline_id=752`
- 正式发布分支：`main`
- 正式 Wharf pipeline：`https://wharf.sy.soyoung.com/dev/app/pm/erzhuang-project/build?page=1&pageSize=20&pipeline_id=771`
- 当前迁移目标：从历史测试环境发布切到公司正式环境发布；正式环境是在 GitLab `main` 分支提交代码后自动触发 Wharf pipeline `771` 构建，构建成功后还需要在 Wharf 点部署并走审批，审批通过并部署成功才算正式发布完成。
- 测试入口：`https://lite.sy.soyoung.com/erzhuang-project/`
- 测试健康检查：`https://lite.sy.soyoung.com/erzhuang-project/health`
- 线上入口：`http://lite.soyoung.com/erzhuang-project`
- 线上健康检查：`http://lite.soyoung.com/erzhuang-project/health`
- 线上发布：对接主干分支代码、主干实例机器和线上数据库；不得把测试分支发布等同于线上发布。
- 本机 GitLab HTTPS token 文件：`/Users/sylar/.codex/secrets/gitlab-erzhuang-project.token`

环境隔离口径：

- `*.sy.soyoung.com` 默认是内网可见测试环境。
- 测试 `https://lite.sy.soyoung.com/erzhuang-project` 对接测试分支代码、测试实例机器、测试数据库。
- 线上 `http://lite.soyoung.com/erzhuang-project` 对接主干分支代码、主干实例机器、线上数据库。
- 主会话在执行“发布到公司”时，必须先区分测试还是正式；当前项目目标已经切到正式环境，因此不能再机械套用测试分支发布。
- 若要发布线上，当前已知发布链路为：代码提交到 GitLab `main`，自动触发 Wharf pipeline `771` 构建；构建成功后在 Wharf 点部署，走审批；审批通过并部署成功后访问 `http://lite.soyoung.com/erzhuang-project` 验收。
- 若只需要测试环境验证，则继续使用 `codex/containerize-single-image`，触发 Wharf pipeline `752`，访问 `https://lite.sy.soyoung.com/erzhuang-project` 验收。

数据库环境口径：

- 二壮运行库测试环境：host `polar-dev.rwlb.rds.aliyuncs.com`，port `3306`，db `db_pm_erzhuang`，user `u_pm_erzhuang_rw`，密码由 K8s Secret 或安全渠道管理，禁止写入仓库。
- 二壮运行库正式环境：host `polar-ops.rwlb.rds.aliyuncs.com`，port `3306`，db `db_pm_erzhuang`，user `u_pm_erzhuang_rw`，密码由 K8s Secret 或安全渠道管理，禁止写入仓库。
- 运行时应通过 `K8S_SECRET_MYSQL_DSN` 或 `MYSQL_DSN` 注入上述连接；不得把 DSN、账号密码写入 Dockerfile、前端变量、文档或命令历史。
- 3.0「门店空间资源查看」复用二壮运行库 MySQL 连接，只读查询已同步到 `db_pm_erzhuang` 的 `tb_crm_*` 资源表；不再配置独立业务库 DSN。

数据库客户端与 DDL 操作边界：

- TablePro 等本机数据库客户端只允许直连测试库 `polar-dev.rwlb.rds.aliyuncs.com:3306/db_pm_erzhuang` 做开发、排查和只读验收。
- 线上库 `polar-ops.rwlb.rds.aliyuncs.com:3306/db_pm_erzhuang` 不通过本机客户端访问；线上数据库只允许由线上代码、K8s/运维链路或公司批准的后台通道访问。
- 测试库与线上库当前按结构一致口径管理。开发过程中如果需要调整线上表结构，Codex 只产出可审查 SQL、影响说明、验证 SQL 和回滚建议；由用户提交给运维，运维在正式库执行。
- 禁止把线上或测试数据库密码写入仓库、文档、命令行参数、临时脚本、长期 TablePro 连接配置或截图。
- 2026-08-18 新口径：已在测试库确认原 `db_groupbuy` 的四张资源查看表同步到二壮自有库 `db_pm_erzhuang`，用户确认线上库也已同步。3.0 已改为复用主库连接；测试环境验收通过后，可由运维清理不再使用的独立业务库 Secret。
- NVR 缩略图回填自有表 DDL 位于 `db/nvr_camera_snapshots.sql`。必须由 DBA/运维分别在测试和正式库执行，Web 服务不得自行建表；Job 账号只获来源表 `SELECT` 及自有表 `SELECT, INSERT, UPDATE`，Web 账号仅获自有表 `SELECT`。

GitLab 推送认证：

- 公司 GitLab 推送默认使用临时 `GIT_ASKPASS` 读取上述 token 文件，用户名使用 `oauth2`。
- 不要把 token 内容打印到终端、拼进命令、写进仓库、写进文档或保存到长期脚本。
- 临时 askpass 脚本只放 `/private/tmp`，权限设为 `700`，推送完成后立即删除。
- 如果 `git push gitlab codex/containerize-single-image` 在非交互环境报 `could not read Username`，不要再让用户手输；直接使用本节 token 文件和临时 askpass。

测试发布标准流程：

```sh
git fetch gitlab
git switch codex/containerize-single-image
git merge main
go test ./...
cd frontend && npm run build && npm test
git push gitlab codex/containerize-single-image
```

正式发布标准流程：

```sh
git fetch gitlab
# 按公司允许的方式让已验证代码提交到 GitLab main：
# 可以是本地切到 main 后提交/合并并推送，也可以是分支 MR 合并到 main
go test ./...
cd frontend && npm run build && npm test
# main 更新后观察 Wharf pipeline 771 构建
# 构建成功后，在 Wharf 点部署并走审批
```

注意：

- `codex/containerize-single-image` 是公司受保护分支，不要 force push。
- `main` 是正式发布分支，也不要 force push；正式环境不是“构建成功即上线”，还需要 Wharf 手动部署和审批。
- 合并时保留公司分支上的 Dockerfile、数据库、K8s 运行配置和路径前缀设置；不要用个人 Lighthouse 配置覆盖公司配置。
- 公司运行时密钥必须通过 K8s Secret 或运行时环境变量注入，不要提交到仓库、Dockerfile、文档或前端 `VITE_*`。
- 公司数据库当前应保持为运维配置的 MySQL，资产存储应保持为 OSS。发布后 `/health` 应返回 `database:"mysql"`、`asset_store:"oss"`。
- 公司容器构建需要注入前端版本号。Dockerfile 应从 `VERSION` 和 `GIT_VERSION` 生成 `VITE_APP_VERSION`；线上页面不应显示 `local-dev`。
- 推送后等待自动发布，再检查页面底部版本号。若构建系统传入 commit，预期为 `2.x.x (<short-sha>)`；若未传入，至少应显示 `2.x.x (container)`。

公司发布失败或版本未更新时，优先排查：

1. GitLab 分支是否已经是最新 commit。
2. 公司流水线是否从正确分支构建：测试看 `codex/containerize-single-image` / pipeline `752`，正式看 `main` / pipeline `771`。
3. 流水线是否使用仓库 Dockerfile，而不是另一个外部构建脚本。
4. 构建缓存或镜像缓存是否导致旧静态资源未更新。
5. 页面静态资源和 API 请求路径是否仍使用 `/erzhuang-project/` 前缀。

## 发布术语速查

默认规则：GitHub 的代码备份能力依然保留；除非用户明确说明“不要同步 GitHub”或“只推公司 GitLab”，已确认准备发布的代码仍应同步到 GitHub 作为主代码备份。二壮项目的实际发布只走公司 GitLab/K8s。韩国 Lighthouse/TAT 发布已经废止，历史说明仅作为早期学习记录，不是当前操作手册。

用户说“发布到公司”时，必须先确认目标是测试还是正式。历史默认含义曾是测试环境，但 2026-08-14 起项目正在切到公司正式环境，不得再默认把测试发布当成最终发布。

用户说“发布测试环境”时，固定执行公司 GitLab 测试自动发布链路：

1. 将当前已确认代码 merge 到公司 GitLab 固定分支 `codex/containerize-single-image`。
2. 推送到 remote `gitlab`。
3. 等待 Wharf pipeline `752` / K8s 自动发布，通常约 5 分钟。
4. 验证 `https://lite.sy.soyoung.com/erzhuang-project/health` 和页面版本号。

注意：

- 测试环境不以 Wharf 页面“部署”按钮作为常规步骤。该按钮可用于排障或用户明确要求的人工补救；正常情况下，GitLab 推送已触发构建并由 K8s 自动部署。
- 构建成功后先等待最多约 5 分钟，再核对测试实例的最近部署 commit 和页面版本。页面短暂保持旧版本时，不得仅据此重复手工部署。
- GitLab 分支更新、构建触发或 `/health` 入口可达都不等于页面已更新；测试发布完成必须至少由 Wharf 实例部署记录或已登录页面的实际版本号确认。
- 不操作韩国 Lighthouse。
- 不 force push 公司受保护分支。
- 公司分支如包含 Dockerfile、K8s 环境变量、数据库连接等运维调整，应保留公司配置，只合入业务代码和必要文档。
- 不操作线上主干实例、线上数据库或线上域名，除非用户明确提出线上发布并完成发布前确认。

用户说“发布线上/生产/主干/正式环境”时，不能套用测试发布流程。当前已知正式流程：

1. 从 GitLab `main` 构建，Wharf pipeline `771`。
2. 代码提交到 `main` 后自动触发正式构建，不直接把测试分支当正式。
3. 构建成功后需要在 Wharf 点部署，并完成审批。
4. 审批通过且部署成功后，线上入口 `http://lite.soyoung.com/erzhuang-project` 对接主干实例和线上数据库。
5. 发布前确认正式实例环境变量、SSO、网关、Cookie、回调路径、本次版本回滚 commit/tag 和线上验证清单。

正式环境切换前置清单：

- 代码分支：正式环境从 GitLab `main` 构建；不要默认使用测试分支 `codex/containerize-single-image`。
- 实例：确认正式实例名称、命名空间、Pod/容器名和 Wharf/K8s 页面入口。
- 数据库：确认正式 `K8S_SECRET_MYSQL_DSN` 指向 `polar-ops.rwlb.rds.aliyuncs.com:3306/db_pm_erzhuang`；确认测试 `K8S_SECRET_MYSQL_DSN` 指向 `polar-dev.rwlb.rds.aliyuncs.com:3306/db_pm_erzhuang`；确认四张 `tb_crm_*` 资源表已同步且 3.0 只读 API 可用，不再保留独立业务库连接。
- 资产：确认正式 OSS bucket/endpoint/access key 已配置且不是测试桶，执行 smoke 前需明确是否有写副作用。
- SSO：确认 `lite.soyoung.com` 的 APISIX SSO 已保护 `/erzhuang-project/`，支持 `/erzhuang-project/_/auth/callback` 与 `/erzhuang-project/logout`；如设置 `SSO_EXPECTED_SUB`，应与正式 token 的 `sub` 一致。
- 前端：确认前端 `BASE_URL` 仍是 `/erzhuang-project/`，公司域名判断包含 `lite.soyoung.com`，页面静态资源仍从 `/erzhuang-project/assets/...` 加载。
- 回滚：确认可回滚到 2.x 稳定 tag `v2.31-stable-before-resource-view-3` 或正式环境当前稳定 commit；回滚版本必须兼容线上 MySQL/OSS，不能依赖旧 Supabase/PostgreSQL。
- 验收：正式发布后至少验证 health、SSO 登录、退出登录、用户管理只读/写权限、门店列表、H5 Monitor 样本门店、3.0 资源查看列表和详情。

用户说“发布到韩国服务器”时，应先提醒：该链路已废止，韩国服务器上的二壮项目库表已经完全删除，不具备二壮项目发布和回滚条件。

以下韩国 Lighthouse 信息仅为历史学习记录，不用于当前二壮项目发布：

- 部署目录：`/opt/apps/erzhuang-project`
- systemd 服务：`erzhuang-project.service`
- 健康检查：`http://127.0.0.1:18081/health`
- 公网入口：`https://43.155.237.46/erzhuang/`
- GitHub 访问方式：服务器 read-only Deploy Key
- 本机 SSH 登录服务器 key：`~/.ssh/erzhuang_lighthouse`
- 服务器内部拉取 GitHub deploy key：`~/.ssh/erzhuang_project_deploy_key`

## 发布当前 main

本节为历史 Lighthouse 练习链路记录，当前二壮项目不得使用该链路发布。

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

本节为历史 Lighthouse 练习链路记录，当前二壮项目不得使用 TAT 发布、回滚或诊断韩国服务器。

以下内容只说明早期个人练习时的 TAT 用法。当前二壮项目不得通过腾讯云 TAT 触发韩国服务器发布。

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
