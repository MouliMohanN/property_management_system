// In-memory store for the access token — shared between the HTTP client and auth session manager.
// Access token lives only in memory: lost on page refresh but safe from XSS unlike localStorage.
let accessToken: string | null = null

export const setAccessToken = (token: string) => { accessToken = token }
export const getAccessToken = () => accessToken
export const clearAccessToken = () => { accessToken = null }
