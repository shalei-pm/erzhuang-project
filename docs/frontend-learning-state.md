# Frontend Learning State

最后更新：2026-06-05

## 当前目标

为 `erzhuang-project` 创建独立的 `frontend/` 前端工程，用于练习真实研发里的前端初始化、本地开发、生产构建和后续静态资源接入流程。

## 技术栈

- Node.js：本地 Codex PATH 中可用 `v24.14.0`
- npm：本地 PATH 暂不可用；本次验证复用 WorkBuddy 内置 npm `10.9.7`
- pnpm：本地 PATH 暂不可用；暂不作为本项目包管理器
- 前端框架：Vite + React + TypeScript

本次验证时需要让 `npm` 和脚本里的 `node` 使用同一套 WorkBuddy Node 工具链，否则 Rollup 原生模块会被 macOS 签名策略拦截。验证命令前统一加：

```sh
PATH=/Applications/WorkBuddy.app/Contents/Resources/vendor/node/node-v22.22.2-darwin-arm64/bin:$PATH
```

## 目录结构

```text
frontend/
├── index.html
├── package.json
├── tsconfig.app.json
├── tsconfig.json
├── tsconfig.node.json
├── vite.config.ts
└── src
    ├── App.tsx
    ├── main.tsx
    ├── styles.css
    └── vite-env.d.ts
```

## 常用命令

进入前端目录：

```sh
cd frontend
```

安装依赖。本机如果 `npm` 不在 PATH，可临时使用 WorkBuddy 内置 npm：

```sh
npm install
```

或：

```sh
/Applications/WorkBuddy.app/Contents/Resources/vendor/node/node-v22.22.2-darwin-arm64/bin/node /Applications/WorkBuddy.app/Contents/Resources/vendor/node/node-v22.22.2-darwin-arm64/lib/node_modules/npm/bin/npm-cli.js install
```

启动本地开发服务器：

```sh
npm run dev
```

生产构建：

```sh
npm run build
```

预览生产构建：

```sh
npm run preview
```

## 验证记录

- 已验证：安装依赖
  - 命令：`npm install`
  - 结果：新增 68 个 packages，审计 0 个漏洞
  - 产物：`frontend/package-lock.json`
- 已验证：生产构建
  - 命令：`npm run build`
  - 结果：通过
  - 输出：`frontend/dist/index.html`、`frontend/dist/assets/*.css`、`frontend/dist/assets/*.js`
- 已验证：本地开发服务器
  - 命令：`npm run dev`
  - 地址：`http://127.0.0.1:5173/`
  - 结果：首页 HTML 可访问，`/src/App.tsx` 可被 Vite 编译并返回

本次遇到的环境问题：

- 当前 Codex 沙箱普通权限不能监听 `127.0.0.1:5173`，启动 dev server 时需要本机权限许可。
- 当前 Codex 沙箱普通权限不能直接访问本机 dev server，使用提权 `curl` 验证通过。
- 这些限制属于本地 Codex 运行环境，不是前端工程代码问题。

## Git 忽略规则

已明确忽略：

- `frontend/node_modules/`
- `frontend/dist/`

真实研发中，`node_modules` 是本机依赖目录，不应提交；`dist` 是构建产物，通常由 CI 或服务器构建生成，也不应提交。

## 后续接入方向

当前前端只负责工程基础，不修改后端发布脚本、systemd 或服务器 nginx 配置。

后续有两种常见接入方式：

1. nginx 直接托管 `frontend/dist`：
   - 服务器构建前端后，把静态文件目录指向 nginx 的 `/erzhuang/` 路径。
   - 优点是静态资源由 nginx 直接返回，性能和配置都比较清晰。

2. Go 后端内嵌或读取静态文件：
   - Go 服务同时提供 API 和前端静态页面。
   - 优点是部署单个服务更简单，适合小项目练习；缺点是前后端边界没有 nginx 托管那么清晰。

正式接入前，建议主会话先决定部署方式，再更新部署脚本和 runbook。

## Phase 2：设计图标记前端骨架

更新时间：2026-06-05

分支：

```text
codex/design-plan-frontend-phase2
```

本阶段范围：

- 已实现门店列表页骨架：添加门店、门店名称搜索、每页 20 条分页、缩略图 hover 浮层预览、空间数量统计、配置状态、更新时间、编辑和删除。
- 已实现添加/编辑共用 lightbox 大弹窗：顶部固定工具栏、左侧 60% 图纸预览区、右侧 40% 区域卡片列表。
- 已实现上传/识别 UI mock：初始、转换中、识别中、识别失败可手动维护、识别完成进入编辑确认。
- 已实现 mock API adapter：文件 `frontend/src/api.ts`，接口路径按 `/api/design-plan` 技术方案组织，后续可替换为真实 HTTP 请求。
- 已接入非敏感测试设计图 PNG：`testdata/design-plans/generated/sample-store-floor-plan.png`，作为 Phase 2 mock 图纸预览、缩略图和区域标注素材。
- 已实现右侧区域卡片维护：区域名称、区域类型、编号、状态、上移、下移、删除；点击卡片会联动左侧高亮框。
- 已实现左侧 mock 图纸与区域框展示：所有区域框可见，选中区域黄色高亮，点击图纸区域框会联动右侧卡片。
- 已实现矩形框基础拖动：拖动已存在区域框可以更新比例坐标并限制在图片边界内。
- 已实现前端保存校验：门店名必填、必须有已转换图纸、至少 1 个区域、区域名称/类型/框必填、治疗室和面诊室编号必填、编号必须为数字、同类型编号唯一。
- 已将页面色彩、区域类型色、选中色和状态色集中放在 `frontend/src/styles.css` 的 CSS 变量中。

本阶段暂未实现：

- 真实 PDF 上传、PDF 转图片、AI 识别调用。
- 通过拖拽新画一个矩形框的完整交互。
- 矩形框四角拉伸。
- 图纸放大、缩小、适应屏幕的真实缩放逻辑；当前先提供工具栏骨架。
- 与后端 CRUD 的真实 HTTP 联调。

本次验证记录：

- 已验证：安装依赖
  - 命令：`PATH=/Applications/WorkBuddy.app/Contents/Resources/vendor/node/node-v22.22.2-darwin-arm64/bin:$PATH npm install`
  - 结果：通过，新增本地 `frontend/node_modules/`
- 已验证：生产构建
  - 命令：`PATH=/Applications/WorkBuddy.app/Contents/Resources/vendor/node/node-v22.22.2-darwin-arm64/bin:$PATH npm run build`
  - 结果：通过
  - 输出：`frontend/dist/index.html`、`frontend/dist/assets/*.css`、`frontend/dist/assets/*.js`
- 已验证：本地开发服务器 HTML 访问
  - 命令：`PATH=/Applications/WorkBuddy.app/Contents/Resources/vendor/node/node-v22.22.2-darwin-arm64/bin:$PATH npm run dev -- --port 5173`
  - 地址：`http://127.0.0.1:5173/erzhuang/`
  - 访问验证：`curl -I http://127.0.0.1:5173/erzhuang/`
  - 结果：返回 `HTTP/1.1 200 OK`
  - 验证后已停止临时 Vite 进程

风险和后续接入点：

- 当前 mock adapter 使用前端内存数据，刷新页面会恢复默认示例数据；这是 Phase 2 骨架阶段预期行为。
- `frontend/src/api.ts` 已保留接口路径和数据类型，后端 Phase 1 合并后，建议将 mock adapter 替换为真实 `fetch` 实现。
- 后续做完整框编辑时，应补“画框模式”和“四角 resize handle”，并继续使用 0 到 1 的比例坐标，避免不同图片尺寸下坐标漂移。
