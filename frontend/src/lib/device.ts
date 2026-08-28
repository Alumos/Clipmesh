import UAParser from "ua-parser-js"
import type { DeviceProfile } from "@/types"

const DEVICE_ID_KEY = "clipmesh-device-id"
const DEVICE_NAME_KEY = "clipmesh-device-name"

export function getDeviceId() {
  const existing = localStorage.getItem(DEVICE_ID_KEY)
  if (existing) return existing
  const id = crypto.randomUUID?.() ?? `web-${Math.random().toString(36).slice(2)}`
  localStorage.setItem(DEVICE_ID_KEY, id)
  return id
}

export function detectDevice(): DeviceProfile {
  const result = new UAParser().getResult()
  const device = [result.device.vendor, result.device.model].filter(Boolean).join(" ")
  const kind = result.device.type === "mobile" || result.device.type === "tablet" ? "mobile" : "desktop"
  const os = [result.os.name, result.os.version].filter(Boolean).join(" ") || "未知系统"
  const browser = [result.browser.name, result.browser.version?.split(".").slice(0, 2).join(".")].filter(Boolean).join(" ") || "浏览器"
  const label = device ? `${device} · ${os}` : `${os} · ${browser}`
  return { label, kind, device: device || "桌面设备", os, browser }
}

export function getDeviceName() {
  return localStorage.getItem(DEVICE_NAME_KEY) ?? detectDevice().label
}

export function setDeviceName(name: string) {
  localStorage.setItem(DEVICE_NAME_KEY, name)
}

export function resetDeviceName() {
  localStorage.removeItem(DEVICE_NAME_KEY)
  return detectDevice().label
}
