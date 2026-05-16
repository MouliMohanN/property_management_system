import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { getMe, logout, refreshTokens, getRefreshToken, hasAccessToken, clearTokens, ApiError, type User } from '../lib/api'

export default function HomePage() {
  const navigate = useNavigate()
  const [user, setUser] = useState<User | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    async function init() {
      const storedRefreshToken = getRefreshToken()

      // No refresh token means the user has never logged in or already logged out.
      if (!storedRefreshToken) {
        navigate('/login')
        return
      }

      try {
        // Only refresh if the in-memory access token is gone (cold page load / hard refresh).
        // If it's already in memory (e.g. navigated here from login), skip the round-trip.
        // This also prevents React StrictMode's double useEffect invocation from consuming
        // the refresh token twice, which would fail due to server-side rotation.
        if (!hasAccessToken()) {
          await refreshTokens(storedRefreshToken)
        }
        const me = await getMe()
        setUser(me)
      } catch (err) {
        // Refresh token is expired or invalid — force re-login.
        clearTokens()
        navigate('/login')
      }
    }

    init()
  }, [navigate])

  async function handleLogout() {
    const storedRefreshToken = getRefreshToken()
    if (storedRefreshToken) {
      try {
        await logout(storedRefreshToken)
      } catch (err) {
        if (err instanceof ApiError && err.status !== 401) {
          setError('Logout failed. Please try again.')
          return
        }
        // 401 means token already expired — still clear locally and redirect.
      }
    }
    navigate('/login')
  }

  if (!user) {
    return (
      <div className="min-h-screen bg-gray-50 flex items-center justify-center">
        <p className="text-sm text-gray-500">Loading…</p>
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-gray-50 px-4 py-12">
      <div className="max-w-lg mx-auto space-y-6">
        <div className="flex items-center justify-between">
          <h1 className="text-xl font-semibold text-gray-900">Dashboard</h1>
          <button
            onClick={handleLogout}
            className="text-sm text-gray-500 hover:text-gray-700 underline transition-colors"
          >
            Sign out
          </button>
        </div>

        {error && (
          <div className="rounded-lg bg-red-50 border border-red-200 px-4 py-3 text-sm text-red-700">
            {error}
          </div>
        )}

        <div className="bg-white rounded-xl border border-gray-200 shadow-sm divide-y divide-gray-100">
          <div className="px-6 py-4">
            <p className="text-xs font-medium uppercase tracking-wide text-gray-400 mb-3">
              Authenticated User
            </p>
            <p className="text-lg font-medium text-gray-900">
              {user.first_name} {user.last_name}
            </p>
            <p className="text-sm text-gray-500">{user.email}</p>
          </div>

          <div className="px-6 py-4 grid grid-cols-2 gap-4">
            <Field label="Role" value={user.role} />
            <Field label="Status" value={user.status} highlight={user.status === 'active'} />
            {user.phone_number && <Field label="Phone" value={user.phone_number} />}
            <Field label="User ID" value={user.id} mono />
          </div>

          <div className="px-6 py-4 grid grid-cols-2 gap-4">
            <Field label="Created" value={new Date(user.created_at).toLocaleString()} />
            <Field label="Updated" value={new Date(user.updated_at).toLocaleString()} />
          </div>
        </div>
      </div>
    </div>
  )
}

function Field({
  label,
  value,
  mono = false,
  highlight = false,
}: {
  label: string
  value: string
  mono?: boolean
  highlight?: boolean
}) {
  return (
    <div>
      <p className="text-xs font-medium text-gray-400 mb-0.5">{label}</p>
      <p
        className={`text-sm break-all ${mono ? 'font-mono text-xs' : ''} ${highlight ? 'text-green-600 font-medium' : 'text-gray-800'}`}
      >
        {value}
      </p>
    </div>
  )
}
