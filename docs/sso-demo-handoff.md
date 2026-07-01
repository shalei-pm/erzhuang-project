# APISIX-SSO 接入说明

最后更新：2026-07-01

本文记录二壮项目接入公司 SSO 的当前口径。根据公司文档《内部系统接入APISIX-SSO使用方式》，本项目统一使用公司推荐的 APISIX 网关 `security-sso` 插件，不自建 OAuth2 登录流程。

## 1. 公司推荐方案

需要使用新版网关及单点登录的业务系统，应向运维同事申请，使服务经由 APISIX 网关转发，并在路由配置中使用插件：`security-sso`。

插件参数示例：

```json
{
  "auth": true,
  "allow_public": false,
  "compatible": false,
  "owner": "xxx"
}
```

参数含义：

- `auth`：是否开启认证，默认 `true`。
- `allow_public`：是否允许外网访问，默认 `false`。
- `compatible`：是否兼容 secgate 网关的 `secgate_token`，默认 `false`。
- `owner`：负责人，建议填写。

需要认证的路由中，需要同时包含插件使用的默认路径：

- `/_/auth/callback`
- `/logout`

还需要安全同事配置 SSO 认证域名白名单。这里的“域名白名单”是安全侧允许本系统域名作为 SSO 登录、回调和 JWT `sub` 签发目标的配置，不是项目代码里的密钥。

## 2. 身份凭证

APISIX-SSO 通过 cookie 携带身份凭证：

- cookie 名称：`sy_sso_token`
- cookie 结构：JWT
- JWT 签名方式：RS256
- JWT payload：

```json
{
  "data": {
    "display": "花名（真名）",
    "mail": "xxx@soyoung.com",
    "open_id": "open_id",
    "user_id": "user_id",
    "phone": "13800112233",
    "username": "xxx",
    "login_way": "lark"
  },
  "exp": 1705046664,
  "sub": "xxx.sy.soyoung.com"
}
```

字段口径：

- `data.mail`：第一版作为本项目用户授权表的唯一登录标识，必须存在。
- `data.open_id`、`data.user_id`：预留飞书身份映射。
- `data.display`、`data.username`、`data.phone`、`data.login_way`：用于展示、审计和后续扩展。
- `exp`：JWT 有效期，后端必须校验。
- `sub`：JWT 签发目标域名。项目支持通过 `SSO_EXPECTED_SUB` 开启严格校验。

## 3. 项目当前实现

已实现：

- 后端可配置开关：`SSO_ENABLED`。
- 登录态查询：`GET /api/auth/me`。
- APISIX 默认回调路径：`GET /_/auth/callback`。
- APISIX 默认登出路径：`GET /logout`。
- API 登出兼容路径：`POST /api/auth/logout`。
- `/erzhuang-project/_/auth/callback`、`/erzhuang/_/auth/callback` 前缀兼容。
- `/erzhuang-project/logout`、`/erzhuang/logout` 前缀兼容。
- 默认读取 SSO cookie：`sy_sso_token`。
- 校验 `sy_sso_token`：
  - JWT 格式必须正确。
  - `alg` 必须为 `RS256`。
  - 使用公司文档公钥或 `SSO_JWT_PUBLIC_KEY` 验签。
  - `exp` 必须未过期。
  - `SSO_EXPECTED_SUB` 配置后必须与 JWT `sub` 一致。
  - `data.mail` 必须存在。
- 从 JWT payload 映射企业邮箱、飞书 open id、飞书 user id、手机号、用户名、展示名和登录方式。
- 前端登录欢迎页：仅在 SSO 启用且未登录时展示。
- 默认关闭时，`/api/auth/me` 返回本地 admin 兼容态，现有后台不被登录页阻断。

暂未实现：

- 用户表 `tb_users` 校验。
- 角色、机构范围、页面/Tab/操作权限。
- 登录成功/拒绝审计。
- session 落 MySQL 或接入公司统一 session。

这些属于下一阶段权限模型实现。

## 4. 环境变量

```sh
SSO_ENABLED=false
SSO_COOKIE_NAME=sy_sso_token
SSO_JWT_PUBLIC_KEY=
SSO_EXPECTED_SUB=
```

说明：

- `SSO_ENABLED=false` 时，后台沿用现有访问方式。
- `SSO_ENABLED=true` 时，后端要求请求携带有效的 `sy_sso_token`。
- `SSO_JWT_PUBLIC_KEY` 不配置时，使用公司 APISIX-SSO 文档中公布的 RS256 公钥；如果安全侧轮换公钥，可以通过该环境变量覆盖。
- `SSO_EXPECTED_SUB` 建议在公司测试/正式环境配置为当前访问域名，开启签发目标校验。
- 不需要在项目里配置 `client_id`、`client_secret`、`authorize_url`、`token_url` 作为主流程。
- 不要记录、打印或持久化原始 `sy_sso_token`。

## 5. 当前行为

### 5.1 SSO 未启用

`GET /api/auth/me` 返回：

```json
{
  "enabled": false,
  "authenticated": true,
  "user": {
    "email": "local-admin@example.com",
    "username": "local-admin",
    "display_name": "本地管理员",
    "role": "admin"
  },
  "permissions": ["admin"]
}
```

前端直接进入现有后台。

### 5.2 SSO 启用但未登录

`GET /api/auth/me` 返回 401：

```json
{
  "enabled": true,
  "authenticated": false,
  "login_url": "/erzhuang-project/_/auth/callback"
}
```

前端展示登录欢迎页，点击按钮进入网关 SSO 回调路径，由 APISIX 插件处理登录。

### 5.3 SSO 启用且有有效 `sy_sso_token`

`GET /api/auth/me` 返回：

```json
{
  "enabled": true,
  "authenticated": true,
  "user": {
    "email": "xxx@soyoung.com",
    "username": "xxx",
    "display_name": "花名（真名）",
    "open_id": "open_id",
    "feishu_user_id": "user_id",
    "phone": "13800112233",
    "login_way": "lark",
    "subject": "xxx.sy.soyoung.com",
    "role": "admin"
  },
  "permissions": ["admin"]
}
```

当前 `role=admin` 和 `permissions=["admin"]` 是 SSO 骨架阶段的兼容态。正式权限上线后，必须改为后端查询 `tb_users`、角色和机构范围。

## 6. 下一阶段联调目标

1. 运维为二壮项目公司测试环境配置 APISIX 路由和 `security-sso` 插件。
2. 安全同事配置 SSO 认证域名白名单。
3. 公司环境配置 `SSO_ENABLED=true`。
4. 建议配置 `SSO_EXPECTED_SUB` 为公司测试环境访问域名。
5. 验证未登录访问会跳转到公司 SSO。
6. 验证飞书登录后 `/api/auth/me` 能返回企业邮箱和飞书身份字段。
7. 接入 `tb_users`：邮箱存在且 enabled 才允许进入。
8. 在后端接口上逐步加权限校验，不只靠前端隐藏。

## 7. 验收口径

- `SSO_ENABLED=false` 时，线上运营后台行为不变。
- `SSO_ENABLED=true` 且无 `sy_sso_token` 时，后台显示登录欢迎页，不加载门店业务数据。
- 网关 SSO 路径使用 `/_/auth/callback` 和 `/logout`。
- `sy_sso_token` 必须完成 RS256 验签和 `exp` 校验。
- `data.mail` 为空时拒绝登录。
- 配置 `SSO_EXPECTED_SUB` 后，`sub` 不匹配时拒绝登录。
- 项目不保存 SSO 密钥，不把 token 打到日志，不把真实用户凭证暴露到前端存储。
- 后续权限判断必须落后端；前端隐藏只作为体验。
