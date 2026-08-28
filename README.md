# Clipmesh

Clipmesh 是一套面向个人 NAS 的全平台 Web 剪贴板同步服务。电脑、手机和平板打开同一个地址即可同步文本、富文本和临时文件；数据保存在自己的 NAS，不依赖第三方云盘。

## 当前能力

- 文本剪贴板：支持 `text/plain` 和可选的 `text/html`，每个账号按时间保留最近 N 条。
- 文件剪贴板：单文件上传、下载，服务端按 TTL 自动清理临时文件。
- 多用户：管理员可在“用户与权限”页面创建普通用户或管理员；每个账号的文本、文件、搜索结果和 SSE 实时事件完全独立。
- 安全登录：bcrypt 密码哈希、SQLite 持久化 Session、HttpOnly Cookie；健康检查接口保持匿名可访问。
- 设备识别：前端使用成熟的 UAParser.js 读取设备型号、操作系统和浏览器，用户仍可手动设置易读设备名。
- 响应式 UI：`components.json` + `src/components/ui` 本地 shadcn/ui 组件源码，配合 Radix UI Tabs/Label 原语；桌面端左侧快速同步、右侧最近剪贴板，手机端自动切换为单列；管理员使用统一的 `/admin/users` 页面。
- 实时连接：SSE 事件带有递增序号，短时断线可通过 `Last-Event-ID` 补发；重连成功后前端会主动刷新列表，避免只依赖推送状态。
- 发布与部署：Docker Compose、`linux/amd64` + `linux/arm64` 镜像、GitHub Actions CI / GHCR / Release。

## 技术结构

```text
frontend/       React + TypeScript + Vite + Tailwind + shadcn/Radix UI
backend/        Go + net/http + SQLite（modernc.org/sqlite，无 CGO）
deploy/         Nginx SPA 与 API 反向代理配置
scripts/        API 冒烟测试
.github/        CI、GHCR 多架构构建和 SemVer Release
```

## NAS 部署

```bash
cp .env.example .env
# 编辑 .env，至少修改 CLIPMESH_ADMIN_PASSWORD 和 CLIPMESH_IMAGE
docker compose pull
docker compose up -d
```

然后访问 `http://NAS-IP:7767`。首次启动会把 `.env` 中的管理员账号写入 SQLite；密码只保存为 bcrypt 哈希。数据库中已有同名管理员时，启动不会用环境变量覆盖其密码。

默认 Docker volume 包含：

```text
/data/clipmesh.db   SQLite 用户、Session 和剪贴板元数据
/data/files/        文件剪贴板临时内容
```

生产环境建议在 NAS 反向代理开启 HTTPS，将 `CLIPMESH_COOKIE_SECURE=true`，并只把服务暴露给需要的网络。浏览器读取/写回系统剪贴板也要求 HTTPS 或 localhost 安全上下文。

## 配置

| 环境变量 | 默认值 | 说明 |
| --- | --- | --- |
| `CLIPMESH_IMAGE` | `ghcr.io/alumos/clipmesh:latest` | Compose 使用的镜像地址 |
| `CLIPMESH_ADMIN_USERNAME` | 无 | 首次启动创建的管理员用户名 |
| `CLIPMESH_ADMIN_PASSWORD` | 无 | 首次启动管理员密码，必填；不会写入明文 |
| `CLIPMESH_COOKIE_SECURE` | `false` | HTTPS 部署时设为 `true` |
| `CLIPMESH_SESSION_TTL` | `168h` | 登录 Session 有效期 |
| `CLIPMESH_PORT` | `7767` | NAS 主机映射端口；容器内部仍使用 8080 |
| `CLIPMESH_TEXT_LIMIT` | `100` | 每个账号保留的文本条数 |
| `CLIPMESH_FILE_TTL` | `24h` | 文件临时保留时间，例如 `12h` |
| `CLIPMESH_CLEANUP_INTERVAL` | `1h` | 过期文件和 Session 扫描间隔 |
| `CLIPMESH_MAX_UPLOAD_SIZE` | `100MB` | 单个文件上传上限 |
| `CLIPMESH_CORS_ORIGINS` | 空 | 跨域部署时填写精确来源，逗号分隔 |

## 本地开发

需要 Go 1.23+、Node.js 22+ 和 npm。Docker 构建阶段会在容器内自动安装 Go 依赖，因此部署 NAS 不要求 NAS 主机安装 Go；本地运行后端才需要 Go。

启动 API（PowerShell）：

```powershell
cd backend
$env:CLIPMESH_DATA_DIR = "$PWD\tmp\clipmesh-data"
$env:CLIPMESH_ADMIN_USERNAME = "admin"
$env:CLIPMESH_ADMIN_PASSWORD = "local-dev-password"
go run ./cmd/server
```

启动前端开发服务器：

```powershell
cd frontend
npm ci
npm run dev
```

访问 `http://localhost:5173`。Vite 会把 `/api` 代理到 `http://localhost:9000`。

验证项目：

```powershell
cd backend
go test ./...
go vet ./...

cd ..\frontend
npm run build

cd ..
$env:CLIPMESH_ADMIN_PASSWORD = "local-dev-password"
node scripts/api-smoke.mjs
```

冒烟测试会覆盖健康检查、未登录拦截、错误密码、文本/文件 CRUD、文件下载、多用户创建、普通用户权限和账号数据隔离。

## API 概览

除 `/api/health` 和 `/api/auth/login` 外，接口都需要登录 Session Cookie。

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| `GET` | `/api/health` | 匿名健康检查 |
| `POST` | `/api/auth/login` | 用户名密码登录 |
| `POST` | `/api/auth/logout` | 注销当前 Session |
| `GET` | `/api/auth/me` | 当前用户 |
| `GET` | `/api/config` | 文本上限、文件 TTL、上传限制 |
| `GET` | `/api/clips?limit=200` | 当前用户剪贴板列表 |
| `POST` | `/api/clips` | 创建文本/富文本剪贴板 |
| `POST` | `/api/clips/file` | multipart 上传临时文件 |
| `GET` | `/api/clips/:id/file` | 下载当前用户文件 |
| `DELETE` | `/api/clips/:id` | 删除当前用户记录 |
| `GET` | `/api/events` | 当前用户 SSE 实时事件流，支持 `Last-Event-ID` 或 `since` 断线补偿 |
| `GET` | `/api/admin/users` | 管理员查看用户 |
| `POST` | `/api/admin/users` | 管理员创建用户，body 为 `username`、`password`、`role` |
| `DELETE` | `/api/admin/users/:id` | 管理员删除用户及其全部数据 |

## 数据库升级

从 `0.1.0` 升级时不需要删除 Docker volume。服务启动会自动创建 `users`、`sessions` 表，并为旧 `clips` 表增加 `user_id`；旧记录会归属首次配置的管理员。删除用户会同时删除其数据库记录、Session 和文件内容，最后一个管理员受到保护。

## 发布

1. 在 `CHANGELOG.md` 的 `Unreleased` 下记录变更，并准备对应的 SemVer 版本段落。
2. 推送代码和 tag，例如：

   ```bash
   git tag v0.2.0
   git push origin v0.2.0
   ```

3. GitHub Actions 会运行 Go/前端验证，构建并推送 `amd64`、`arm64` GHCR 镜像，再从对应版本段落创建 GitHub Release。
4. NAS 更新：

   ```bash
   docker compose pull
   docker compose up -d
   ```
