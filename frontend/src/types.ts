export type ClipKind = "text" | "file"

export interface User {
  id: string
  username: string
  role: "admin" | "user"
  createdAt: string
}

export interface Clip {
  id: string
  kind: ClipKind
  deviceId: string
  deviceName: string
  mimeType?: string
  name?: string
  size?: number
  formats?: Record<string, string>
  preview?: string
  createdAt: string
  expiresAt?: string
}

export interface AppConfig {
  textLimit: number
  fileTtlSeconds: number
  maxUploadBytes: number
  authEnabled: boolean
  pageSize: number
}

export interface ClipEvent {
  type: "created" | "deleted"
  clip?: Clip
  id?: string
}

export interface DeviceProfile {
  label: string
  device: string
  os: string
  browser: string
}
