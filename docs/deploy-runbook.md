# Deploy Runbook

本项目的服务器发布目标是个人腾讯云 Lighthouse：

- 部署目录：`/opt/apps/erzhuang-project`
- systemd 服务：`erzhuang-project.service`
- 健康检查：`http://127.0.0.1:18081/health`
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
5. `sudo systemctl restart erzhuang-project.service`。
6. `curl -fsS http://127.0.0.1:18081/health`。

如果任一步失败，脚本会停止。

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
```

## 安全边界

- 不使用腾讯云主账号。
- 不复制个人 GitHub SSH 私钥到服务器。
- 服务器只使用仓库级 read-only Deploy Key 拉取代码。
- 默认不修改 nginx。
- 默认不开放公网端口。
- 默认只重启 `erzhuang-project.service`。
