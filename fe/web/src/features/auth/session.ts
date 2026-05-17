import { setAccessToken, getAccessToken, clearAccessToken } from '../../shared/token-store'

export { setAccessToken }

const REFRESH_TOKEN_KEY = 'pms_refresh_token'

export const getRefreshToken = () => localStorage.getItem(REFRESH_TOKEN_KEY)
export const setRefreshToken = (token: string) => localStorage.setItem(REFRESH_TOKEN_KEY, token)

// Returns true when an access token is already in memory (warm state — e.g. navigated from login).
// Used to skip an unnecessary refresh round-trip and to guard against React StrictMode's
// double-invoke consuming the rotation-based refresh token twice.
export const hasActiveSession = () => getAccessToken() !== null

export function clearSession() {
  clearAccessToken()
  localStorage.removeItem(REFRESH_TOKEN_KEY)
}
