const BASE_URL = import.meta.env.VITE_API_URL ?? 'http://localhost:8080'
const REFRESH_TOKEN_KEY = 'pms_refresh_token'

// Access token lives only in memory — lost on page refresh but safe from XSS
// (unlike localStorage). On refresh, the stored refresh token re-issues it.
let accessToken: string | null = null

export function setAccessToken(token: string) {
  accessToken = token
}

export function getRefreshToken(): string | null {
  return localStorage.getItem(REFRESH_TOKEN_KEY)
}

function setRefreshToken(token: string) {
  localStorage.setItem(REFRESH_TOKEN_KEY, token)
}

export function hasAccessToken(): boolean {
  return accessToken !== null
}

export function clearTokens() {
  accessToken = null
  localStorage.removeItem(REFRESH_TOKEN_KEY)
}

// ── Error ─────────────────────────────────────────────────────────────────────

export class ApiError extends Error {
  status: number
  code: string

  constructor(status: number, code: string, message: string) {
    super(message)
    this.status = status
    this.code = code
  }
}

// ── Core fetch wrapper ────────────────────────────────────────────────────────

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(options.headers as Record<string, string>),
  }

  if (accessToken) {
    headers['Authorization'] = `Bearer ${accessToken}`
  }

  const res = await fetch(`${BASE_URL}${path}`, { ...options, headers })

  if (res.status === 204) return undefined as T

  const body = await res.json().catch(() => ({}))

  if (!res.ok) {
    throw new ApiError(
      res.status,
      body.error?.code ?? 'UNKNOWN',
      body.error?.message ?? 'Request failed',
    )
  }

  return body as T
}

// ── Types ─────────────────────────────────────────────────────────────────────

export interface User {
  id: string
  email: string
  phone_number?: string
  first_name: string
  last_name: string
  role: string
  status: string
  created_at: string
  updated_at: string
}

// ── Auth API ──────────────────────────────────────────────────────────────────

interface LoginResponse {
  data: {
    access_token: string
    refresh_token: string
    user: { id: string; email: string; role: string }
  }
}

export async function login(email: string, password: string) {
  const res = await request<LoginResponse>('/api/v1/auth/login', {
    method: 'POST',
    body: JSON.stringify({ email, password }),
  })
  setAccessToken(res.data.access_token)
  setRefreshToken(res.data.refresh_token)
  return res.data
}

interface RefreshResponse {
  data: { access_token: string; refresh_token: string }
}

export async function refreshTokens(refreshToken: string) {
  const res = await request<RefreshResponse>('/api/v1/auth/refresh', {
    method: 'POST',
    body: JSON.stringify({ refresh_token: refreshToken }),
  })
  setAccessToken(res.data.access_token)
  setRefreshToken(res.data.refresh_token)
  return res.data
}

export async function logout(refreshToken: string) {
  await request('/api/v1/auth/logout', {
    method: 'POST',
    body: JSON.stringify({ refresh_token: refreshToken }),
  })
  clearTokens()
}

export async function getMe(): Promise<User> {
  const res = await request<{ data: User }>('/api/v1/auth/me')
  return res.data
}
