# Changelog

本文件记录每个可发布版本的重要变更，格式参考 [Keep a Changelog](https://keepachangelog.com/)，版本遵循 [Semantic Versioning](https://semver.org/)。

## [Unreleased]

### Planned

- 桌面常驻客户端：通过 GitHub Actions 构建 Windows、macOS 和 Linux 安装包。
- 手机端分享扩展和系统剪贴板接入。

## [0.4.0] - 2026-08-29

### Added

- 页面前台且未编辑输入框时，`Ctrl/Command + V` 可把本机剪贴板中的文本、HTML、图片或文件直接同步到当前账号。
- `Ctrl/Command + C` 默认把当前账号最新一条文本记录写入本机剪贴板；选中文本或编辑输入框时继续使用浏览器原生复制粘贴行为。
- 快速同步面板增加快捷键提示和“读取并同步”入口。

### Changed

- 将剪贴板读取、格式转换和写回集中到单一模块，按钮操作与键盘操作复用同一上传链路和文件大小校验。
- 移除手工 HTML 编辑区、重复统计区、Radix Tabs 组件及其依赖链，并启用 TypeScript 未使用代码检查。
- CI 合并 Compose 门禁并取消重复依赖下载步骤，新增并发取消，减少无效 Runner 时间。
- Release 要求对应提交的 main CI 已成功，不再重复执行前后端构建；Docker 使用 BuildKit 原生交叉编译，前端只构建一次，移除 QEMU，并缓存 npm、Go 模块和编译产物。

### Fixed

- 快捷键不会劫持输入框、可编辑区域或用户已经选中的页面文本。
- 文本记录在时间戳相同时改用 SQLite 写入顺序稳定排序，避免快速连续同步时误删或复制到非最新记录。

## [0.3.0] - 2026-08-29

### Added

- 历史记录中的图片文件提供紧凑缩略图，并支持点击查看自适应大图预览。

### Changed

- 重构前端状态流与组件职责：初始化、手动刷新和 SSE 重连各自只请求必要数据，移除重复网络请求，并修正普通用户误入管理路由时的导航状态。
- 后端按鉴权、HTTP 路由、剪贴板、用户和 Session 拆分职责；删除仅供旧测试使用的内存会话分支和无用户作用域存储接口，所有剪贴板读写均强制绑定账号。
- Release 流程增加版本元数据、`go vet` 和 Compose 校验，镜像构建器升级到 Go 1.27。
- Web 首页调整为历史剪贴板优先：桌面端历史记录作为主栏、快速同步作为右侧栏，手机端先显示历史记录再显示同步入口。
- 文本、文件和图片统一为高密度列表行，长文本限制为三行预览，减少滚动并提升首屏信息量。
- 搜索、类型筛选、分页、设备信息和快捷操作重新组织；设备设置默认折叠，移动端提供“新建”快速跳转。
- 移除重复的绿联 NAS 专用 Compose 子目录，统一使用根目录的 Linux 多架构 Compose 配置。
- 发布流水线增加 45 分钟超时保护，并降低 GitHub Actions 构建缓存导出开销。

### Fixed

- 将历史记录分页统一为每页 6 条，避免服务端旧值覆盖紧凑首屏布局。
- 文件名清理现在保持 UTF-8 和扩展名完整，中文或超长文件名可安全下载。

## [0.2.2] - 2026-08-29

> 此标签的多架构镜像构建被取消，未创建正式 GitHub Release；相关部署预设已由 `0.3.0` 清理。

### Added

- 增加绿联 NAS 成品 Compose 配置，固定映射 `7767:8080`，并提供独立 `.env.example`。
- NAS 配置默认锁定本次发布的 `ghcr.io/alumos/clipmesh:0.2.2` 镜像，限制日志大小并保留健康检查。

## [0.2.1] - 2026-08-29

### Changed

- NAS Docker Compose 默认端口调整为 `7767:8080`，部署后访问 `http://NAS-IP:7767`。

## [0.2.0] - 2026-08-29

### Added

- 管理员“用户与权限”页面：创建用户、分配角色、搜索账号、删除用户及其全部数据。
- SSE 事件序号、用户级短时事件历史和 `Last-Event-ID` 断线补偿。
- 实时连接状态展示：连接中、实时同步、重连中和未连接。

### Changed

- 管理功能从弹窗迁移到统一的 `/admin/users` 应用页面，桌面端导航和移动端导航保持一致。
- SSE 事件增加全局序号和短时用户级历史；客户端重连会携带 `Last-Event-ID` 并在连接恢复后重新拉取列表。
- 实时状态细分为连接中、实时同步、重连中和未连接，并显示最近事件时间。

### Fixed

- 统一 Go 模块和 GHCR 示例地址为 `github.com/Alumos/Clipmesh` / `ghcr.io/alumos/clipmesh`。

## [0.1.0] - 2026-08-28

### Added

- Go + SQLite 后端，支持文本、text/html 富文本和临时文件剪贴板。
- 文本数量限制、文件 TTL 清理、SSE 实时同步、搜索和筛选。
- 账号密码登录、HttpOnly Session Cookie 和匿名健康检查。
- React + TypeScript + Tailwind + shadcn/ui 风格响应式 Web 界面。
- Docker Compose NAS 部署、GitHub Actions CI、GHCR 多架构镜像和 tag 驱动 Release。
