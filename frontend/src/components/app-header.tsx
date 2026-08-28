import type { ReactNode } from "react"
import { Clipboard, LogOut, RefreshCw, Users, Wifi, WifiOff } from "lucide-react"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"
import type { User } from "@/types"

export type AppView = "clipboard" | "users"
export type ConnectionState = "offline" | "connecting" | "connected" | "reconnecting"

interface AppHeaderProps {
  view: AppView
  user: User | null
  connection: ConnectionState
  lastSyncAt: Date | null
  loading: boolean
  onNavigate: (view: AppView) => void
  onRefresh: () => void
  onLogout: () => void
}

const connectionLabels: Record<ConnectionState, string> = {
  offline: "未连接",
  connecting: "连接中",
  connected: "实时同步",
  reconnecting: "重连中",
}

export function AppHeader({
  view,
  user,
  connection,
  lastSyncAt,
  loading,
  onNavigate,
  onRefresh,
  onLogout,
}: AppHeaderProps) {
  const connectionLabel = connectionLabels[connection]
  const connectionTitle = lastSyncAt
    ? `最近事件：${lastSyncAt.toLocaleTimeString("zh-CN")}`
    : connectionLabel

  return (
    <>
      <header className="sticky top-0 z-30 border-b bg-background/90 backdrop-blur-xl">
        <div className="container flex min-h-16 items-center justify-between gap-3">
          <div className="flex min-w-0 items-center gap-3">
            <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-primary text-primary-foreground">
              <Clipboard className="h-4 w-4" />
            </div>
            <div className="min-w-0">
              <div className="flex items-center gap-2">
                <span className="truncate text-sm font-semibold tracking-tight sm:text-base">
                  Clipmesh
                </span>
                <Badge variant="outline" className="hidden sm:inline-flex">
                  PRIVATE SYNC
                </Badge>
              </div>
              <p className="hidden text-xs text-muted-foreground sm:block">
                你的私有跨设备剪贴板
              </p>
            </div>
          </div>

          <nav
            aria-label="主导航"
            className="hidden items-center gap-1 rounded-lg border bg-muted/50 p-1 md:flex"
          >
            <NavItem active={view === "clipboard"} onClick={() => onNavigate("clipboard")}>
              <Clipboard className="h-4 w-4" />
              剪贴板
            </NavItem>
            {user?.role === "admin" && (
              <NavItem active={view === "users"} onClick={() => onNavigate("users")}>
                <Users className="h-4 w-4" />
                用户与权限
              </NavItem>
            )}
          </nav>

          {user && (
            <div className="flex items-center gap-1.5 sm:gap-2">
              <div
                title={connectionTitle}
                className={cn(
                  "flex items-center gap-1.5 rounded-full border px-2.5 py-1.5 text-xs sm:px-3",
                  connection === "connected" ? "text-foreground" : "text-muted-foreground",
                )}
              >
                <ConnectionIcon state={connection} />
                <span className="hidden sm:inline">{connectionLabel}</span>
              </div>
              <Button
                variant="ghost"
                size="icon"
                aria-label="刷新剪贴板"
                onClick={onRefresh}
              >
                <RefreshCw className={cn("h-4 w-4", loading && "animate-spin")} />
              </Button>
              <div className="hidden h-8 w-px bg-border sm:block" />
              <span className="hidden max-w-28 truncate text-xs text-muted-foreground sm:block">
                {user.username}
              </span>
              <Button variant="ghost" size="icon" aria-label="退出登录" onClick={onLogout}>
                <LogOut className="h-4 w-4" />
              </Button>
            </div>
          )}
        </div>
      </header>

      {user && (
        <div className="border-b bg-background md:hidden">
          <nav aria-label="移动端主导航" className="container flex gap-1 py-2">
            <NavItem active={view === "clipboard"} onClick={() => onNavigate("clipboard")}>
              <Clipboard className="h-4 w-4" />
              剪贴板
            </NavItem>
            {user.role === "admin" && (
              <NavItem active={view === "users"} onClick={() => onNavigate("users")}>
                <Users className="h-4 w-4" />
                用户与权限
              </NavItem>
            )}
          </nav>
        </div>
      )}
    </>
  )
}

function ConnectionIcon({ state }: { state: ConnectionState }) {
  if (state === "connected") {
    return <Wifi className="h-3.5 w-3.5" />
  }
  if (state === "reconnecting" || state === "connecting") {
    return <RefreshCw className="h-3.5 w-3.5 animate-spin" />
  }
  return <WifiOff className="h-3.5 w-3.5" />
}

function NavItem({
  active,
  onClick,
  children,
}: {
  active: boolean
  onClick: () => void
  children: ReactNode
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        "inline-flex h-9 items-center justify-center gap-2 rounded-md px-3 text-sm font-medium transition-colors",
        active
          ? "bg-background text-foreground shadow-sm"
          : "text-muted-foreground hover:bg-background/70 hover:text-foreground",
      )}
    >
      {children}
    </button>
  )
}
