import { useState, type FormEvent } from "react"
import { KeyRound, LockKeyhole } from "lucide-react"

import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"

interface AuthOverlayProps {
  loading: boolean
  onLogin: (username: string, password: string) => Promise<boolean>
}

export function AuthOverlay({ loading, onLogin }: AuthOverlayProps) {
  const [username, setUsername] = useState(
    () => localStorage.getItem("clipmesh-login-username") ?? "",
  )
  const [password, setPassword] = useState("")

  async function submit(event: FormEvent) {
    event.preventDefault()
    const normalizedUsername = username.trim()
    if (!normalizedUsername || !password) return

    if (await onLogin(normalizedUsername, password)) {
      localStorage.setItem("clipmesh-login-username", normalizedUsername)
      setPassword("")
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4 backdrop-blur-sm">
      <Card className="w-full max-w-md shadow-2xl">
        <CardHeader>
          <div className="mb-2 flex h-10 w-10 items-center justify-center rounded-xl bg-primary text-primary-foreground">
            <LockKeyhole className="h-5 w-5" />
          </div>
          <CardTitle>登录 Clipmesh</CardTitle>
          <CardDescription>
            请输入 NAS 上的账号和密码。每个账号只能看到自己的剪贴板数据。
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form className="space-y-3" onSubmit={(event) => void submit(event)}>
            <div className="space-y-2">
              <Label htmlFor="login-username">账号</Label>
              <Input
                id="login-username"
                autoFocus
                autoComplete="username"
                value={username}
                onChange={(event) => setUsername(event.target.value)}
                placeholder="账号"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="login-password">密码</Label>
              <Input
                id="login-password"
                type="password"
                autoComplete="current-password"
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                placeholder="密码"
              />
            </div>
            <Button
              type="submit"
              className="mt-2 w-full"
              disabled={loading || !username.trim() || !password}
            >
              <KeyRound className="h-4 w-4" />
              {loading ? "登录中…" : "登录并开始同步"}
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}
