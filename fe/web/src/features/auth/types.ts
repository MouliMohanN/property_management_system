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

// Returned by both login and token refresh endpoints.
export interface AuthTokenPair {
  access_token: string
  refresh_token: string
}

export interface AuthLoginResponse {
  data: AuthTokenPair & {
    user: { id: string; email: string; role: string }
  }
}

export interface RefreshTokensResponse {
  data: AuthTokenPair
}
