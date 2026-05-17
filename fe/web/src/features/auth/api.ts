import { request } from '../../shared/http'
import { setAccessToken, setRefreshToken, clearSession } from './session'
import type { User, AuthLoginResponse, RefreshTokensResponse } from './types'

export async function login(email: string, password: string) {
  const res = await request<AuthLoginResponse>('/api/v1/auth/login', {
    method: 'POST',
    body: JSON.stringify({ email, password }),
  })
  setAccessToken(res.data.access_token)
  setRefreshToken(res.data.refresh_token)
  return res.data
}

export async function refreshTokens(refreshToken: string) {
  const res = await request<RefreshTokensResponse>('/api/v1/auth/refresh', {
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
  clearSession()
}

export async function getMe(): Promise<User> {
  const res = await request<{ data: User }>('/api/v1/auth/me')
  return res.data
}
