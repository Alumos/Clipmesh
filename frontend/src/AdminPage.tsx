import { useCallback, useEffect, useMemo, useState } from "react"
import { KeyRound, Loader2, RefreshCw, Search, ShieldCheck, Trash2, UserPlus, Users } from "lucide-react"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { createUser, deleteUser, fetchUsers } from "@/lib/api"
import type { User } from "@/types"

interface AdminPageProps {
  currentUser: User
  onNotice: (message: string) => void
  onError: (message: string) => void
}

export function AdminPage({ currentUser, onNotice, onError }: AdminPageProps) {
  const [users, setUsers] = useState<User[]>([])
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [deleting, setDeleting] = useState("")
  const [username, setUsername] = useState("")
  const [password, setPassword] = useState("")
  const [role, setRole] = useState<User["role"]>("user")
  const [search, setSearch] = useState("")

  const loadUsers = useCallback(async () => {
    setLoading(true)
    try {
      setUsers(await fetchUsers())
    } catch (caught) {
      onError(caught instanceof Error ? caught.message : "无法加载用户列表")
    } finally {
      setLoading(false)
    }
  }, [onError])

  useEffect(() => {
    void loadUsers()
  }, [loadUsers])

  const visibleUsers = useMemo(() => {
    const query = search.trim().toLowerCase()
    if (!query) return users
    return users.filter((user) => `${user.username} ${user.role}`.toLowerCase().includes(query))
  }, [search, users])

  const adminCount = users.filter((user) => user.role === "admin").length

  async function handleCreate() {
    setSaving(true)
    onError("")
    try {
      const created = await createUser(username.trim(), password, role)
      setUsers((current) => [...current, created])
      setUsername("")
      setPassword("")
      setRole("user")
      onNotice(`已创建用户 ${created.username}`)
    } catch (caught) {
      onError(caught instanceof Error ? caught.message : "创建用户失败")
    } finally {
      setSaving(false)
    }
  }

  async function handleDelete(user: User) {
    if (user.id === currentUser.id || !window.confirm(`确定删除用户“${user.username}”及其全部剪贴板数据吗？`)) return
    setDeleting(user.id)
    try {
      await deleteUser(user.id)
      setUsers((current) => current.filter((item) => item.id !== user.id))
      onNotice(`已删除用户 ${user.username}`)
    } catch (caught) {
      onError(caught instanceof Error ? caught.message : "删除用户失败")
    } finally {
      setDeleting("")
    }
  }

  return (
    <main className="container space-y-6 py-5 sm:py-8">
      <section className="flex flex-col gap-6 xl:flex-row xl:items-end xl:justify-between">
        <div className="space-y-3">
          <Badge variant="secondary" className="gap-1.5"><ShieldCheck className="h-3.5 w-3.5" />管理中心</Badge>
          <div>
            <h1 className="text-3xl font-semibold tracking-tight sm:text-4xl">用户与权限</h1>
            <p className="mt-2 max-w-2xl text-sm leading-6 text-muted-foreground sm:text-base">在这里管理账号、角色和访问边界。每个账号只能看到自己的剪贴板与设备活动。</p>
          </div>
        </div>
        <div className="grid grid-cols-2 gap-2 sm:gap-3 xl:w-[26rem]">
          <AdminMetric label="账号总数" value={users.length.toString()} icon={<Users className="h-4 w-4" />} />
          <AdminMetric label="管理员" value={adminCount.toString()} icon={<ShieldCheck className="h-4 w-4" />} />
        </div>
      </section>

      <section className="grid items-start gap-5 xl:grid-cols-[minmax(20rem,0.72fr)_minmax(0,1.28fr)] xl:gap-6">
        <Card className="overflow-hidden shadow-sm">
          <CardHeader className="border-b bg-muted/35 pb-4">
            <CardTitle className="flex items-center gap-2 text-base"><UserPlus className="h-4 w-4" />创建账号</CardTitle>
            <CardDescription className="mt-1">创建后即可独立登录和同步内容</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4 pt-5">
            <div className="space-y-2"><Label htmlFor="new-username">用户名</Label><Input id="new-username" value={username} onChange={(event) => setUsername(event.target.value)} placeholder="例如：family" /></div>
            <div className="space-y-2"><Label htmlFor="new-password">初始密码</Label><Input id="new-password" type="password" value={password} onChange={(event) => setPassword(event.target.value)} placeholder="至少 8 位" /></div>
            <div className="space-y-2"><Label htmlFor="new-role">角色</Label><select id="new-role" value={role} onChange={(event) => setRole(event.target.value as User["role"])} className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-ring"><option value="user">普通用户</option><option value="admin">管理员</option></select></div>
            <Button className="w-full" onClick={() => void handleCreate()} disabled={saving || !username.trim() || password.length < 8}><UserPlus className="h-4 w-4" />{saving ? "创建中…" : "创建用户"}</Button>
            <div className="rounded-lg bg-muted/60 p-3 text-xs leading-5 text-muted-foreground"><p className="font-medium text-foreground">安全提示</p><p className="mt-1">密码只以 bcrypt 哈希形式保存，管理员无法读取原密码。公网使用前请启用 HTTPS。</p></div>
          </CardContent>
        </Card>

        <Card className="overflow-hidden">
          <CardHeader className="gap-4 border-b pb-4">
            <div className="flex items-start justify-between gap-3">
              <div><CardTitle className="flex items-center gap-2 text-base"><Users className="h-4 w-4" />现有账号</CardTitle><CardDescription className="mt-1">账号权限和创建时间</CardDescription></div>
              <Button variant="ghost" size="icon" onClick={() => void loadUsers()} disabled={loading} aria-label="刷新用户列表"><RefreshCw className={`h-4 w-4 ${loading ? "animate-spin" : ""}`} /></Button>
            </div>
            <div className="relative"><Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" /><Input value={search} onChange={(event) => setSearch(event.target.value)} placeholder="搜索用户名或角色" className="pl-9" /></div>
          </CardHeader>
          <CardContent className="p-4 sm:p-5">
            {loading ? <LoadingState /> : visibleUsers.length === 0 ? <EmptyUsers searched={Boolean(search.trim())} /> : <div className="space-y-2">
              <div className="hidden grid-cols-[minmax(0,1fr)_7rem_8rem_2.5rem] gap-3 px-3 pb-1 text-xs text-muted-foreground sm:grid"><span>账号</span><span>角色</span><span>创建时间</span><span /></div>
              {visibleUsers.map((user) => <div key={user.id} className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-3 rounded-lg border px-3 py-3 sm:grid-cols-[minmax(0,1fr)_7rem_8rem_2.5rem]">
                <div className="flex min-w-0 items-center gap-3"><div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-muted text-xs font-semibold">{user.username.slice(0, 1).toUpperCase()}</div><div className="min-w-0"><p className="truncate text-sm font-medium">{user.username}</p><p className="truncate text-xs text-muted-foreground sm:hidden">{user.role === "admin" ? "管理员" : "普通用户"} · {formatDate(user.createdAt)}</p></div></div>
                <Badge className="hidden w-fit sm:inline-flex" variant={user.role === "admin" ? "default" : "secondary"}>{user.role === "admin" ? "管理员" : "普通用户"}</Badge>
                <span className="hidden text-xs text-muted-foreground sm:block">{formatDate(user.createdAt)}</span>
                {user.id === currentUser.id ? <Badge variant="outline" className="hidden w-fit sm:inline-flex">当前账号</Badge> : <Button variant="ghost" size="icon" className="text-muted-foreground hover:text-foreground" disabled={deleting === user.id} onClick={() => void handleDelete(user)} aria-label={`删除 ${user.username}`}>{deleting === user.id ? <Loader2 className="h-4 w-4 animate-spin" /> : <Trash2 className="h-4 w-4" />}</Button>}
              </div>)}
            </div>}
          </CardContent>
        </Card>
      </section>

      <div className="flex items-center gap-2 text-xs text-muted-foreground"><KeyRound className="h-3.5 w-3.5" />当前登录：{currentUser.username} · 管理员权限</div>
    </main>
  )
}

function AdminMetric({ label, value, icon }: { label: string; value: string; icon: React.ReactNode }) {
  return <div className="rounded-xl border bg-card p-3 shadow-sm sm:p-4"><div className="mb-2 flex h-7 w-7 items-center justify-center rounded-lg bg-muted text-foreground">{icon}</div><p className="truncate text-[11px] text-muted-foreground sm:text-xs">{label}</p><p className="mt-1 truncate text-sm font-semibold sm:text-base">{value}</p></div>
}

function LoadingState() {
  return <div className="flex min-h-36 items-center justify-center rounded-lg border border-dashed text-sm text-muted-foreground"><Loader2 className="mr-2 h-4 w-4 animate-spin" />正在加载用户列表…</div>
}

function EmptyUsers({ searched }: { searched: boolean }) {
  return <div className="flex min-h-36 flex-col items-center justify-center rounded-lg border border-dashed px-6 text-center"><Users className="h-6 w-6 text-muted-foreground" /><p className="mt-3 text-sm font-medium">{searched ? "没有匹配的账号" : "还没有用户"}</p><p className="mt-1 text-xs text-muted-foreground">{searched ? "换一个用户名或角色试试" : "从左侧创建第一个独立账号"}</p></div>
}

function formatDate(value: string) {
  return new Date(value).toLocaleDateString("zh-CN", { year: "numeric", month: "numeric", day: "numeric" })
}
