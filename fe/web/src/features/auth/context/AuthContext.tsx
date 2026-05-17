import { createContext, useContext, useEffect, useState, type ReactNode } from 'react'
import { ApiError } from '../../../shared/http'
import { getRefreshToken, hasActiveSession, clearSession } from '../session'
import { login as apiLogin, logout as apiLogout, refreshTokens, getMe } from '../api'
import type { User } from '../types'

interface AuthState {
  user: User | null
  loading: boolean
  login: (email: string, password: string) => Promise<void>
  logout: () => Promise<void>
}

const AuthContext = createContext<AuthState | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    async function restoreSession() {
      const storedRefreshToken = getRefreshToken()

      if (!storedRefreshToken) {
        setLoading(false)
        return
      }

      try {
        if (!hasActiveSession()) {
          await refreshTokens(storedRefreshToken)
        }
        const me = await getMe()
        setUser(me)
      } catch {
        clearSession()
      } finally {
        setLoading(false)
      }
    }

    restoreSession()
  }, [])

  async function login(email: string, password: string) {
    // apiLogin only returns a partial user { id, email, role } — not sufficient
    // for the dashboard. getMe fetches the full User shape after tokens are set.
    await apiLogin(email, password)
    const me = await getMe()
    setUser(me)
  }

  async function logout() {
    const storedRefreshToken = getRefreshToken()
    if (storedRefreshToken) {
      try {
        await apiLogout(storedRefreshToken)
      } catch (err) {
        // 401 means the refresh token already expired — clear locally and continue.
        if (!(err instanceof ApiError && err.status === 401)) {
          throw err
        }
      }
    }
    clearSession()
    setUser(null)
  }

  return (
    <AuthContext.Provider value={{ user, loading, login, logout }}>
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth(): AuthState {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used within AuthProvider')
  return ctx
}
