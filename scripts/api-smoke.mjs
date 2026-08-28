import { readFile } from "node:fs/promises"

const baseUrl = process.env.CLIPMESH_TEST_BASE_URL ?? "http://127.0.0.1:9000"
const username = process.env.CLIPMESH_ADMIN_USERNAME ?? "admin"
const password = process.env.CLIPMESH_ADMIN_PASSWORD

if (!password) {
  throw new Error("Set CLIPMESH_ADMIN_PASSWORD before running the smoke test")
}

async function expectStatus(response, expected, label) {
  if (response.status !== expected) {
    throw new Error(`${label}: expected ${expected}, got ${response.status}: ${await response.text()}`)
  }
}

const health = await fetch(`${baseUrl}/api/health`)
await expectStatus(health, 200, "health")

const unauthenticated = await fetch(`${baseUrl}/api/config`)
await expectStatus(unauthenticated, 401, "unauthenticated config")

const badLogin = await fetch(`${baseUrl}/api/auth/login`, {
  method: "POST",
  headers: { "content-type": "application/json" },
  body: JSON.stringify({ username, password: `${password}-wrong` }),
})
await expectStatus(badLogin, 401, "bad login")

const login = await fetch(`${baseUrl}/api/auth/login`, {
  method: "POST",
  headers: { "content-type": "application/json" },
  body: JSON.stringify({ username, password }),
})
await expectStatus(login, 200, "login")
const setCookie = login.headers.getSetCookie?.()[0] ?? login.headers.get("set-cookie")
if (!setCookie) throw new Error("login did not return a session cookie")
const cookie = setCookie.split(";", 1)[0]
const adminUser = await login.json()

const authenticatedHeaders = { Cookie: cookie }
const textResponse = await fetch(`${baseUrl}/api/clips`, {
  method: "POST",
  headers: { ...authenticatedHeaders, "content-type": "application/json" },
  body: JSON.stringify({
    kind: "text",
    deviceId: "smoke-test",
    deviceName: "Smoke test",
    formats: {
      "text/plain": "Clipmesh API smoke test",
      "text/html": "<strong>Clipmesh API smoke test</strong>",
    },
  }),
})
await expectStatus(textResponse, 201, "text create")
const textClip = await textResponse.json()

const fileBytes = await readFile(new URL("../frontend/package.json", import.meta.url))
const form = new FormData()
form.append("file", new Blob([fileBytes], { type: "application/json" }), "package.json")
form.append("deviceId", "smoke-test")
form.append("deviceName", "Smoke test")
const fileResponse = await fetch(`${baseUrl}/api/clips/file`, {
  method: "POST",
  headers: authenticatedHeaders,
  body: form,
})
await expectStatus(fileResponse, 201, "file create")
const fileClip = await fileResponse.json()

const download = await fetch(`${baseUrl}/api/clips/${fileClip.id}/file`, { headers: authenticatedHeaders })
await expectStatus(download, 200, "file download")
const downloadedBytes = (await download.arrayBuffer()).byteLength
if (downloadedBytes !== fileClip.size) throw new Error(`download size mismatch: ${downloadedBytes} != ${fileClip.size}`)

const deleted = await fetch(`${baseUrl}/api/clips/${textClip.id}`, { method: "DELETE", headers: authenticatedHeaders })
await expectStatus(deleted, 204, "text delete")
const deletedFile = await fetch(`${baseUrl}/api/clips/${fileClip.id}`, { method: "DELETE", headers: authenticatedHeaders })
await expectStatus(deletedFile, 204, "file delete")

// Verify the multi-user boundary: each account receives its own clip list,
// while only an administrator can create or remove accounts.
const userUsername = `smoke-${Date.now()}`
const userPassword = "smoke-user-password"
const createUser = await fetch(`${baseUrl}/api/admin/users`, {
  method: "POST",
  headers: { ...authenticatedHeaders, "content-type": "application/json" },
  body: JSON.stringify({ username: userUsername, password: userPassword, role: "user" }),
})
await expectStatus(createUser, 201, "admin create user")
const createdUser = await createUser.json()

const userLogin = await fetch(`${baseUrl}/api/auth/login`, {
  method: "POST",
  headers: { "content-type": "application/json" },
  body: JSON.stringify({ username: userUsername, password: userPassword }),
})
await expectStatus(userLogin, 200, "user login")
const userSetCookie = userLogin.headers.getSetCookie?.()[0] ?? userLogin.headers.get("set-cookie")
if (!userSetCookie) throw new Error("user login did not return a session cookie")
const userCookie = userSetCookie.split(";", 1)[0]
const userHeaders = { Cookie: userCookie }

const userClipResponse = await fetch(`${baseUrl}/api/clips`, {
  method: "POST",
  headers: { ...userHeaders, "content-type": "application/json" },
  body: JSON.stringify({
    kind: "text",
    deviceId: "user-smoke-test",
    deviceName: "User smoke test",
    formats: { "text/plain": "private user clipboard" },
  }),
})
await expectStatus(userClipResponse, 201, "user text create")
const userClip = await userClipResponse.json()

const userList = await fetch(`${baseUrl}/api/clips`, { headers: userHeaders })
await expectStatus(userList, 200, "user clip list")
const userItems = (await userList.json()).items
if (!userItems.some((item) => item.id === userClip.id)) throw new Error("user cannot see its own clip")

const adminList = await fetch(`${baseUrl}/api/clips`, { headers: authenticatedHeaders })
await expectStatus(adminList, 200, "admin isolated clip list")
const adminItems = (await adminList.json()).items
if (adminItems.some((item) => item.id === userClip.id)) throw new Error("admin clip list leaked another user's clip")

const userAdminAccess = await fetch(`${baseUrl}/api/admin/users`, { headers: userHeaders })
await expectStatus(userAdminAccess, 403, "普通用户后台权限")

const deleteUser = await fetch(`${baseUrl}/api/admin/users/${createdUser.id}`, { method: "DELETE", headers: authenticatedHeaders })
await expectStatus(deleteUser, 204, "admin delete user")
const revokedUserSession = await fetch(`${baseUrl}/api/clips`, { headers: userHeaders })
await expectStatus(revokedUserSession, 401, "deleted user session revoked")

console.log(JSON.stringify({
  health: health.status,
  unauthenticated: unauthenticated.status,
  badLogin: badLogin.status,
  login: login.status,
  textCreate: textResponse.status,
  fileCreate: fileResponse.status,
  fileDownload: download.status,
  downloadedBytes,
  cleanup: [deleted.status, deletedFile.status],
  multiUser: { admin: adminUser.username, user: userUsername, isolated: true },
}))
