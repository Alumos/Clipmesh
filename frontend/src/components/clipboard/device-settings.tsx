import { ChevronDown, Laptop, RotateCcw, Settings2, Smartphone } from "lucide-react"

import { Button } from "@/components/ui/button"
import { Card } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import type { DeviceProfile } from "@/types"

interface DeviceSettingsProps {
  name: string
  profile: DeviceProfile
  onNameChange: (name: string) => void
  onSave: () => void
  onRestore: () => void
}

export function DeviceSettings({
  name,
  profile,
  onNameChange,
  onSave,
  onRestore,
}: DeviceSettingsProps) {
  return (
    <Card className="overflow-hidden">
      <details className="group">
        <summary className="flex cursor-pointer list-none items-center gap-3 p-4">
          <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted">
            {profile.kind === "desktop" ? (
              <Laptop className="h-4 w-4" />
            ) : (
              <Smartphone className="h-4 w-4" />
            )}
          </div>
          <div className="min-w-0 flex-1">
            <p className="truncate text-sm font-medium">{name}</p>
            <p className="truncate text-xs text-muted-foreground">
              {profile.os} · {profile.browser}
            </p>
          </div>
          <ChevronDown className="h-4 w-4 text-muted-foreground transition-transform group-open:rotate-180" />
        </summary>

        <div className="space-y-4 border-t p-4">
          <div className="space-y-2">
            <Label htmlFor="device-name">当前设备名称</Label>
            <Input
              id="device-name"
              value={name}
              onChange={(event) => onNameChange(event.target.value)}
              placeholder="例如：办公室 Mac"
            />
            <p className="text-xs leading-5 text-muted-foreground">
              系统和浏览器由 UA 自动识别，也可以自定义易读名称。
            </p>
          </div>
          <div className="flex flex-col gap-2 sm:flex-row lg:flex-col">
            <Button variant="outline" className="flex-1" onClick={onSave}>
              <Settings2 className="h-4 w-4" />
              保存设备名称
            </Button>
            <Button variant="ghost" onClick={onRestore}>
              <RotateCcw className="h-4 w-4" />
              恢复自动识别
            </Button>
          </div>
          <p className="rounded-lg bg-muted/60 p-3 text-xs leading-5 text-muted-foreground">
            数据按账号隔离；公网部署请开启 HTTPS 和 Secure Cookie。
          </p>
        </div>
      </details>
    </Card>
  )
}
