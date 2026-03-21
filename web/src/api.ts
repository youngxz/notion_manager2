import type { DashboardData } from './types'

// --- Auth API ---

async function sha256hex(message: string): Promise<string> {
  if (!globalThis.crypto?.subtle) {
    throw new Error('当前浏览器环境不支持登录校验，请使用 localhost 或现代浏览器')
  }
  const data = new TextEncoder().encode(message)
  const hash = await crypto.subtle.digest('SHA-256', data)
  return Array.from(new Uint8Array(hash)).map(b => b.toString(16).padStart(2, '0')).join('')
}

async function readJson<T>(resp: Response, fallbackMessage: string): Promise<T> {
  const text = await resp.text()
  if (!text) {
    throw new Error(fallbackMessage)
  }
  try {
    return JSON.parse(text) as T
  } catch {
    throw new Error(fallbackMessage)
  }
}

export interface AuthStatus {
  authenticated: boolean
  required: boolean
}

export async function checkAuth(): Promise<AuthStatus> {
  const resp = await fetch('/dashboard/auth/check', {
    headers: { Accept: 'application/json' },
    credentials: 'same-origin',
  })
  if (!resp.ok) throw new Error(`HTTP ${resp.status}`)
  return readJson<AuthStatus>(resp, '认证状态接口返回了无效响应')
}

export async function fetchSalt(): Promise<{ salt: string; required: boolean }> {
  const resp = await fetch('/dashboard/auth/salt', {
    headers: { Accept: 'application/json' },
    credentials: 'same-origin',
  })
  if (!resp.ok) throw new Error(`HTTP ${resp.status}`)
  return readJson<{ salt: string; required: boolean }>(resp, '登录校验配置返回了无效响应')
}

export async function login(password: string): Promise<{ ok: boolean; error?: string }> {
  const { salt } = await fetchSalt()
  const hash = await sha256hex(salt + password)
  const resp = await fetch('/dashboard/auth/login', {
    method: 'POST',
    headers: {
      Accept: 'application/json',
      'Content-Type': 'application/json',
    },
    credentials: 'same-origin',
    body: JSON.stringify({ hash }),
  })
  const data = await readJson<{ error?: string }>(resp, '登录接口返回了无效响应')
  if (!resp.ok) return { ok: false, error: data.error || 'Login failed' }
  return { ok: true }
}

export async function logout(): Promise<void> {
  await fetch('/dashboard/auth/logout', { method: 'POST', credentials: 'same-origin' })
}

// --- Dashboard API ---

export async function fetchDashboardData(): Promise<DashboardData> {
  // Uses dashboard session cookie for auth (not API key)
  const resp = await fetch('/admin/accounts')
  if (!resp.ok) throw new Error(`HTTP ${resp.status}`)
  return resp.json()
}

export async function triggerRefresh(): Promise<{ started: boolean; message?: string }> {
  // Uses dashboard session cookie for auth (not API key)
  const resp = await fetch('/admin/refresh', { method: 'POST' })
  return resp.json()
}

export function openProxy(email: string) {
  window.open(`/proxy/start?email=${encodeURIComponent(email)}`, '_blank')
}

export function openBestProxy() {
  window.open('/proxy/start?best=true', '_blank')
}

// --- Settings API ---

export interface SearchSettings {
  enable_web_search: boolean
  enable_workspace_search: boolean
  disable_notion_prompt: boolean
  debug_logging: boolean
}

export async function fetchSettings(): Promise<SearchSettings> {
  // Uses dashboard session cookie for auth (not API key)
  const resp = await fetch('/admin/settings')
  if (!resp.ok) throw new Error(`HTTP ${resp.status}`)
  return resp.json()
}

export async function updateSettings(settings: Partial<Pick<SearchSettings, 'enable_web_search' | 'enable_workspace_search' | 'debug_logging'>>): Promise<SearchSettings> {
  // Uses dashboard session cookie for auth (not API key)
  const resp = await fetch('/admin/settings', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(settings),
  })
  if (!resp.ok) throw new Error(`HTTP ${resp.status}`)
  return resp.json()
}
