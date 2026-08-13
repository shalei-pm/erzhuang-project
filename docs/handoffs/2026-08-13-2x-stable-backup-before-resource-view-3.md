# 2.x Stable Backup Before Resource View 3.0

日期：2026-08-13

## 备份目的

在二壮 3.0「门店空间资源查看」大版本开发前，固定当前 2.x 稳定状态，方便后续新会话、发布会话或回滚会话快速判断：3.0 之前线上可用版本在哪里、能回到哪里、哪些能力不应丢失。

## 备份点

- 当前分支：`codex/containerize-single-image`
- 稳定 tag：`v2.31-stable-before-resource-view-3`
- 稳定 commit：`7311bda`
- 稳定提交说明：`fix: fallback h5 player for unsupported h265 mse`
- 版本文件：`2.31.8`
- 公司发布分支：`gitlab/codex/containerize-single-image`
- GitHub 备份分支：`origin/codex/containerize-single-image`

## 2.x 稳定能力范围

- 公司运行时：MySQL + OSS。
- 登录：APISIX SSO + `tb_users` 授权。
- 权限：`admin` / `editor` / `viewer`，普通查看用户监控门店范围权限已实现。
- 门店空间资源旧主流程：门店列表、门店详情、设计图标注、通道映射、AI 通道识别、人工确认、门店/录像机/通道写接口。
- H5 Monitor：萤石云直播/回放、区域 Tab 返回保持、Windows H.265 fallback。
- 发布链路：只走公司 GitLab/K8s，不走韩国 Lighthouse。

## 3.0 开发边界

- 3.0 将后台主流程切换为只读「门店空间资源查看」。
- 3.0 不改 H5 Monitor 播放链路。
- 3.0 不删除旧 storespace/designplan 代码和数据；旧能力先作为回滚与历史兼容保留。
- 3.0 新增业务库只读连接时，禁止把业务库 DSN、账号、密码、token 写入仓库。

## 回滚方式

如果 3.0 发布后阻断核心使用，优先回滚到仍兼容 MySQL/OSS 的 2.x 稳定点：

```bash
git switch codex/containerize-single-image
git revert <3.0-merge-commit>
git push gitlab codex/containerize-single-image
```

如需查看 2.x 备份点：

```bash
git show --stat v2.31-stable-before-resource-view-3
git diff v2.31-stable-before-resource-view-3..HEAD --stat
```

如需从 tag 临时创建排查分支：

```bash
git switch -c codex/inspect-2x-stable v2.31-stable-before-resource-view-3
```

## 后续读取顺序

1. `docs/codex-learning-state.md`
2. `docs/decisions.md`
3. `work/current-plan.md`
4. `docs/superpowers/specs/2026-08-13-store-space-resource-view-3-design.md`
5. `docs/superpowers/plans/2026-08-13-store-space-resource-view-3-implementation.md`
6. 本文件

## 备份后验证清单

- `git show --stat v2.31-stable-before-resource-view-3` 可读取。
- 本文件已记录稳定 commit、版本、能力范围、回滚方式和读取顺序。
- 线上 `/erzhuang-project/health` 仍返回 `database=mysql`、`asset_store=oss`。
- H5 Monitor 样本门店仍可打开。
- 用户管理仍可打开，viewer 门店范围权限不回退。

## 注意事项

- 此 tag 是 3.0 开发前的稳定参考点，不代表必须用 `git reset` 回滚。
- 公司发布分支是受保护分支，回滚优先用 `git revert`，不要 force push。
- 韩国 Lighthouse 已废止，且该服务器上的二壮项目库表已删除，不具备回滚条件。
- 旧 PostgreSQL/Supabase 已不再作为运行时或回滚路径。
