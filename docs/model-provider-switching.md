# AI 模型切换说明

最后更新：2026-06-26

本文档供 Codex（四喜）在需要切换模型 provider、更换 API Key 时参考。用户说"帮我换 minimax 的 key"或"切到 minimax"时，按此文档操作。

## 项目中用到模型的地方

项目有两个 AI recognizer，都支持 OpenAI 和 MiniMax 两种 provider：

1. **通道截图区域识别**（`internal/channelai/recognizer.go`）
   - 用途：识别监控截图里的业务区域类型和编号
   - provider 切换环境变量：`CHANNEL_AI_PROVIDER`

2. **设计图 PDF 识别**（`internal/designplan/recognizer.go`）
   - 用途：识别上传的 PDF 设计图里的区域标注
   - provider 切换环境变量：`DESIGN_PLAN_AI_PROVIDER`（不设则跟随 `CHANNEL_AI_PROVIDER`）

## 环境变量一览

### OpenAI（默认 provider）

| 环境变量 | 用途 | 默认值 |
|---|---|---|
| `OPENAI_API_KEY` | OpenAI API Key | 无，必须配置 |
| `OPENAI_BASE_URL` | OpenAI 基础地址 | `https://api.openai.com` |
| `OPENAI_MODEL` | designplan 使用的模型 | `gpt-4o` |
| `OPENAI_API_STYLE` | designplan 调用方式 | `responses`（可选 `chat_completions`）|
| `VISION_API_KEY` | channelai 专用 key（不设则回退 `OPENAI_API_KEY`）| 无 |
| `VISION_API_BASE_URL` | channelai 专用 base URL（不设则回退 `OPENAI_BASE_URL`）| 无 |
| `VISION_MODEL` | channelai 使用的模型 | `gpt-5.5` |

### MiniMax（备选 provider）

| 环境变量 | 用途 | 默认值 |
|---|---|---|
| `MINIMAX_API_KEY` | MiniMax API Key | 无，切换到 minimax 时必须配置 |
| `MINIMAX_BASE_URL` | MiniMax 基础地址 | `https://api.minimaxi.com` |
| `MINIMAX_MODEL` | MiniMax 模型名 | `MiniMax-M3` |

MiniMax provider 只读取 `MINIMAX_API_KEY`，不会复用 `OPENAI_API_KEY` 或 `VISION_API_KEY`。这样配置缺失时会明确失败，避免实际调用到错误供应商或错误账号。

### Provider 切换

| 环境变量 | 值 | 效果 |
|---|---|---|
| `CHANNEL_AI_PROVIDER` | `openai`（或不设）| 通道识别走 OpenAI |
| `CHANNEL_AI_PROVIDER` | `minimax` | 通道识别走 MiniMax HTTP API |
| `CHANNEL_AI_PROVIDER` | `minimax-script` | 通道识别走旧 Python 脚本（兜底，验收后可删）|
| `DESIGN_PLAN_AI_PROVIDER` | `openai`（或不设）| 设计图识别走 OpenAI |
| `DESIGN_PLAN_AI_PROVIDER` | `minimax` | 设计图识别走 MiniMax HTTP API |

## 切换操作步骤

### 从 OpenAI 切到 MiniMax

1. 确认 `MINIMAX_API_KEY` 已配置。
2. 设置 `CHANNEL_AI_PROVIDER=minimax`。
3. 设置 `DESIGN_PLAN_AI_PROVIDER=minimax`（或不设，自动跟随 `CHANNEL_AI_PROVIDER`）。
4. 重启服务。
5. 验证：触发一次通道识别和一次设计图识别，检查返回结果正常，并记录 provider、耗时和失败详情。

### 从 MiniMax 切回 OpenAI

1. 设置 `CHANNEL_AI_PROVIDER=openai`（或删除该变量）。
2. 设置 `DESIGN_PLAN_AI_PROVIDER=openai`（或删除该变量）。
3. 重启服务。

### 只换 Key 不换 Provider

**换 OpenAI Key：**
- 修改 `OPENAI_API_KEY`（如果 channelai 用了独立的 `VISION_API_KEY`，也要改）。
- 重启服务。

**换 MiniMax Key：**
- 修改 `MINIMAX_API_KEY`。
- 重启服务。

### 换默认模型

**OpenAI：**
- channelai：设置 `VISION_MODEL=<新模型名>`
- designplan：设置 `OPENAI_MODEL=<新模型名>`

**MiniMax：**
- 两个 recognizer 共用：设置 `MINIMAX_MODEL=<新模型名>`

## 冒烟验证清单

切换 provider 或更换模型后，需要做两次真实调用验证：

1. **通道截图识别**
   - 找一条已有最近截图的通道，触发单通道重新识别。
   - 预期：接口返回业务区域类型、编号/备注、provider 为 `minimax` 或 `openai`。
   - 失败时重点看：HTTP 状态码、模型返回体、是否是 key/权限/限流问题。

2. **设计图识别**
   - 使用一张已上传的设计图，触发“识别图纸区域”。
   - 预期：返回门店名、区域列表、相对坐标框。
   - 失败时重点看：PDF 预览图是否可读、模型返回是否是 JSON、是否被 markdown 代码块包裹。

也可以用本地 CLI 做同等验证：

```sh
CHANNEL_AI_PROVIDER=minimax \
DESIGN_PLAN_AI_PROVIDER=minimax \
MINIMAX_API_KEY=<运行时密钥> \
MINIMAX_MODEL=<模型名> \
go run ./cmd/ai-smoke --mode channel --image-url "https://example.com/snapshot.jpg"
```

```sh
CHANNEL_AI_PROVIDER=minimax \
DESIGN_PLAN_AI_PROVIDER=minimax \
MINIMAX_API_KEY=<运行时密钥> \
MINIMAX_MODEL=<模型名> \
go run ./cmd/ai-smoke --mode design --image-file /path/to/design-preview.png
```

命令输出会包含识别结果和 `elapsed_ms`。不要把真实 key 写进命令历史、脚本、Dockerfile 或文档；生产环境应通过 K8s Secret、systemd 环境文件或等效密钥管理方式注入。

## 各环境配置位置

### 公司环境（GitLab / K8s）

- 环境变量通过 K8s Secret 注入。
- 修改后等 K8s 自动重新部署（约 5 分钟）。
- 不要把密钥写入代码仓库、Dockerfile 或前端 `VITE_*` 变量。

### 韩国服务器（GitHub / Lighthouse）

- 环境变量在 `systemd` service 文件或部署脚本中配置。
- 修改后执行 `systemctl restart erzhuang-project.service`。

## 注意事项

- MiniMax 的 `minimax-script` 路径依赖外部 Python 脚本（`/root/.openclaw/workspace/skills/...`），只作为短期兜底。MiniMax HTTP 真实环境验收通过后，应删除该路径，彻底解耦 OpenClaw。
- MiniMax HTTP 当前按 Chat Completions 兼容格式调用（`/v1/chat/completions`），不使用 Responses API。2026-06-26 已用 `MiniMax-M3` 做过真实冒烟；`MiniMax-M1` 不支持图片输入，`MiniMax-01-vision` 在当前 key 的模型列表中不可用。
- 两个 recognizer 的 MiniMax HTTP 调用都是 Go 内置 HTTP，不依赖 openclaw、不依赖 Python 环境。
- 切换 provider 不需要改代码，只改环境变量。
- `MINIMAX_BASE_URL` 可以配置成 `https://api.minimaxi.com` 或带 `/v1` 的地址，代码会避免重复拼接 `/v1`。
