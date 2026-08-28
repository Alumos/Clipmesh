# Changelog

本文件记录每个可发布版本的重要变更，格式参考 [Keep a Changelog](https://keepachangelog.com/)，版本遵循 [Semantic Versioning](https://semver.org/)。

## [Unreleased]

### Changed

- Docker Compose 默认将 NAS 主机端口调整为 `7767`，容器内部端口保持 `8080`。

### Planned

- 桌面常驻客户端：通过 GitHub Actions 构建 Windows、macOS 和 Linux 安装包。
- 手机端分享扩展和系统剪贴板接入。

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
