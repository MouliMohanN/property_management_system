import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { ApiError } from '../../../shared/http'
import { useAuth } from '../../auth/context/AuthContext'
import UserProfileField from '../components/UserProfileField'

export default function DashboardPage() {
  const { user, logout } = useAuth()
  const navigate = useNavigate()
  const [error, setError] = useState<string | null>(null)

  async function handleLogout() {
    try {
      await logout()
      navigate('/login')
    } catch (err) {
      if (err instanceof ApiError) {
        setError('Logout failed. Please try again.')
      }
    }
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

        {user && (
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
              <UserProfileField label="Role" value={user.role} />
              <UserProfileField label="Status" value={user.status} highlight={user.status === 'active'} />
              {user.phone_number && <UserProfileField label="Phone" value={user.phone_number} />}
              <UserProfileField label="User ID" value={user.id} mono />
            </div>

            <div className="px-6 py-4 grid grid-cols-2 gap-4">
              <UserProfileField label="Created" value={new Date(user.created_at).toLocaleString()} />
              <UserProfileField label="Updated" value={new Date(user.updated_at).toLocaleString()} />
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
