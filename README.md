# TeamPulse

TeamPulse 是一个本地优先（local-first）的工程团队洞察仪表盘，用于同步 GitHub 仓库活动，分析 Pull Request 流程风险，并生成可直接发送给管理层、团队或站会使用的工程周报。

它的目标不是做另一个“开发者监控平台”，而是帮助团队快速回答几个关键问题：

- 最近团队在推进什么？
- 哪些 PR 正在阻塞交付？
- 哪些仓库、成员或流程信号需要关注？
- 这周可以如何向 stakeholders 汇报工程进展？

## 核心能力

### GitHub 活动同步

- 同步最近 30 天的 commits、pull requests、reviews 和 CI 状态。
- 支持选择多个仓库批量同步。
- 同步过程中显示当前阶段、当前仓库、仓库序号、耗时和失败原因。
- 支持 GitHub CLI 登录态或临时输入 fine-grained PAT。

### 工程仪表盘

- Overview：工程健康度、风险、PR、团队活动总览。
- Activity：按仓库、成员和活动类型查看近期工程事件。
- Pull Requests：查看 PR 状态、评审、CI、变更规模和风险提示。
- Team：按时间范围查看成员活动信号，避免简单排行榜式评估。
- Repositories：查看仓库活跃度、CI 健康度、open PR 和最近活动。
- Risks：聚合 stale PR、等待评审、CI failure、大 PR 等风险信号。
- Reports：生成 Markdown 周报、日报和风险报告。
- Settings：GitHub 连接、仓库同步、通知、数据与本地服务状态。

### 风险规则配置

当前支持配置：

- Waiting review threshold
- Stale PR days
- Large PR line threshold
- CI failure threshold

这些规则会保存到本地，用于帮助团队根据自己的交付节奏判断风险。

### 报告模板

报告支持多种结构：

- Executive summary
- Engineering detail
- Risk-focused
- Standup-ready

生成后的报告以 Markdown 保存，可复制或下载。

### 本地通知

支持浏览器本地通知：

- High risk detected
- Sync failed
- Sync success
- Weekly report reminder

## 技术栈

| Layer | Technology |
| --- | --- |
| Backend | Go 1.22, chi, SQLite |
| Frontend | React, TypeScript, Vite, Tailwind CSS |
| Data Fetching | TanStack Query |
| Desktop | SwiftUI menu bar app |
| Storage | Local SQLite database |
| Distribution | Embedded web assets + local Go server |

## 项目结构

```text
.
├── cmd/teampulse/              # Go server entrypoint
├── internal/app/               # API routes, GitHub sync, reports, risks, SQLite schema
├── web/                        # React + Vite frontend
│   ├── src/app/                # App shell and navigation
│   ├── src/components/         # Shared UI components
│   ├── src/lib/                # API client, formatting, preferences
│   └── src/pages/              # Dashboard pages
├── desktop/macos/              # SwiftUI menu-bar launcher
├── dist/                       # Packaged macOS app artifacts, when built
├── Makefile                    # Common build/run targets
└── README.md
```

## 环境要求

必需：

- Go 1.22+
- Node.js 20+
- npm

可选：

- GitHub CLI，用于 `gh auth login`
- Swift 5.9+ / Xcode Command Line Tools，用于构建 macOS menu bar app

## 快速开始

### 1. 登录 GitHub

推荐使用 GitHub CLI：

```bash
gh auth login
```

也可以在 TeamPulse 的 Settings 页面输入 fine-grained PAT。通过 UI 输入的 token 只保存在当前进程内存中，不会写入本地数据库。

### 2. 启动应用

```bash
make run
```

服务启动后会输出类似：

```json
{"event":"server_ready","url":"http://127.0.0.1:19421"}
```

打开该 URL，进入 Settings，选择仓库并开始同步。

## 开发模式

### 后端

```bash
make server
./build/teampulse-server
```

默认监听：

```text
http://127.0.0.1:19421
```

如果端口被占用，服务会在 `19421` 到 `19521` 之间自动寻找可用端口。

也可以指定参数：

```bash
./build/teampulse-server -host 127.0.0.1 -port 19421 -data-dir ./tmp/data
```

### 前端

```bash
cd web
npm install
npm run dev
```

Vite 开发服务会代理 `/api` 到本地 Go 服务。POST 请求需要保持同源校验通过，因此建议通过 Vite 页面访问，而不是直接从任意外部 Origin 调用 API。

### 构建前端生产资源

```bash
cd web
npm run build
```

构建产物会写入：

```text
internal/app/webdist/
```

Go 服务会通过 `embed.FS` 内嵌这些静态资源。

## 常用命令

```bash
make web       # 安装前端依赖并构建生产前端
make server    # 构建前端并编译 Go server
make run       # 构建并启动本地服务
make test      # 运行 Go 测试并构建前端
make macos     # 构建 server 和 SwiftUI macOS launcher
make clean     # 清理 build、node_modules 和 Swift 构建缓存
```

## macOS App

项目包含一个 SwiftUI menu bar app：

- 启动本地 TeamPulse server。
- 自动打开 Dashboard。
- 支持 Restart Server。
- 支持打开本地数据目录。
- 支持退出应用并停止服务。

构建：

```bash
make macos
```

当前仓库也可能包含已打包产物：

```text
dist/TeamPulse.app
dist/TeamPulse-macOS.zip
```

如果重新构建了前端或 Go 后端，需要重新打包 macOS app，确保 app 内置的是最新的 `teampulse-server` 和 web assets。

## 数据存储

TeamPulse 默认将数据保存在本机：

```text
~/Library/Application Support/TeamPulse/
```

主要文件和目录：

| Path | Description |
| --- | --- |
| `teampulse.db` | SQLite 主数据库 |
| `reports/` | 本地生成的 Markdown 报告 |
| `run/server.json` | 当前服务运行状态 |
| `run/server.pid` | 当前服务进程 ID |
| `logs/` | 预留日志目录 |
| `cache/` | 预留缓存目录 |
| `backups/` | 预留备份目录 |

## 安全与隐私

TeamPulse 的安全边界是“本地开发者工具”：

- HTTP 服务默认只绑定 `127.0.0.1`。
- 非本机访问会被拒绝。
- 非 GET 请求会做 Origin 校验，避免跨站请求。
- GitHub PAT 不会持久化保存。
- 数据默认只写入本地 SQLite。
- 报告生成在本地完成，不会上传到云端服务。

## API 概览

### System

| Method | Endpoint | Description |
| --- | --- | --- |
| `GET` | `/api/health` | 服务健康检查 |
| `GET` | `/api/app/status` | 应用运行状态 |
| `POST` | `/api/system/shutdown` | 请求关闭本地服务 |

### GitHub

| Method | Endpoint | Description |
| --- | --- | --- |
| `GET` | `/api/github/auth/status` | GitHub 认证状态 |
| `POST` | `/api/github/auth/token` | 设置临时 PAT |
| `DELETE` | `/api/github/auth` | 清除当前内存 token |
| `GET` | `/api/github/repositories` | 获取 GitHub 可访问仓库 |

### Sync & Data

| Method | Endpoint | Description |
| --- | --- | --- |
| `GET` | `/api/repositories` | 已同步仓库列表 |
| `POST` | `/api/repositories/sync` | 启动仓库同步任务 |
| `GET` | `/api/jobs` | 同步任务列表 |
| `GET` | `/api/jobs/{id}` | 单个同步任务详情 |
| `GET` | `/api/activity` | 活动列表 |
| `GET` | `/api/members` | 成员活动统计 |
| `GET` | `/api/pull-requests` | PR 列表 |

### Risks

| Method | Endpoint | Description |
| --- | --- | --- |
| `GET` | `/api/risks` | 风险信号列表 |
| `PATCH` | `/api/risks/{id}` | 更新风险状态 |
| `GET` | `/api/risk-rules` | 获取风险规则 |
| `PUT` | `/api/risk-rules` | 更新风险规则 |

### Reports

| Method | Endpoint | Description |
| --- | --- | --- |
| `POST` | `/api/reports/generate` | 生成报告 |
| `GET` | `/api/reports` | 报告历史 |
| `GET` | `/api/reports/{id}` | 获取报告详情 |

## 故障排查

### API 返回 403 Forbidden

常见原因：

- 请求不是从 `127.0.0.1` 或 `localhost` 发起。
- POST/PUT/PATCH/DELETE 请求的 `Origin` 与后端服务 URL 不一致。
- 直接绕过 Vite proxy 调用 API。

处理方式：

- 开发时优先通过 Vite 页面访问。
- 确认后端实际 URL，例如 `http://127.0.0.1:19421`。
- 如果端口发生 fallback，以 server 输出的 URL 为准。

### GitHub 仓库为空

可能原因：

- 未登录 GitHub CLI。
- PAT 权限不足。
- fine-grained token 没有授权目标 organization 或 repository。

处理方式：

```bash
gh auth status
gh auth login
```

或在 Settings 中输入具有 repository read 权限的 PAT。

### 同步长时间运行

同步耗时取决于：

- 仓库数量
- 最近 30 天 PR 数量
- GitHub API 响应速度
- GitHub rate limit

同步面板会显示当前阶段、当前仓库、仓库进度和失败原因。如果任务失败，优先检查 token 权限、仓库访问权限和 GitHub rate limit。

## 当前限制

- 当前重点是本地单用户使用场景。
- GitHub OAuth device flow 尚未接入。
- macOS Keychain 持久化 token 尚未接入。
- 自动后台定时同步仍是后续增强项。
- 已打包 app 如需发布给其他用户，还需要签名和 notarization。

## Roadmap

- GitHub OAuth device flow
- Keychain token storage
- 自动定时同步
- 更细粒度的 GitHub rate limit 展示
- SSE / WebSocket 实时同步事件
- 可配置报告导出格式
- macOS signed and notarized distribution

## License

当前仓库未声明开源许可证。发布或分发前请补充明确的 License。
