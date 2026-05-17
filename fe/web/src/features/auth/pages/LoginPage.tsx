import LoginForm from '../components/LoginForm'

export default function LoginPage() {
  return (
    <div className="min-h-screen bg-gray-50 flex items-center justify-center px-4">
      <div className="w-full max-w-sm">
        <div className="mb-8 text-center">
          <h1 className="text-2xl font-semibold text-gray-900">Property Management</h1>
          <p className="mt-1 text-sm text-gray-500">Sign in to your account</p>
        </div>
        <LoginForm />
      </div>
    </div>
  )
}
