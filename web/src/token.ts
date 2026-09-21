const STORAGE_KEY = 'broffice_token'

/**
 * Reads the auth token from localStorage, bootstrapping from the URL on first
 * visit. Accepts a `?token=` query parameter (as encoded in the startup QR
 * code) or a `#token=` fragment. The parameter is stripped immediately so the
 * token never leaks into shared links or browser history.
 */
export function bootstrapToken(): string | null {
  const params = new URLSearchParams(window.location.search)
  const queryToken = params.get('token')
  if (queryToken !== null) {
    try {
      localStorage.setItem(STORAGE_KEY, queryToken)
    } catch {
      // Storage unavailable (private mode); the SPA keeps working from the URL.
    }
    params.delete('token')
    const qs = params.toString()
    history.replaceState(null, '', window.location.pathname + (qs ? `?${qs}` : '') + window.location.hash)
  }

  const match = window.location.hash.match(/[#&]token=([^&]+)/)
  if (match) {
    try {
      localStorage.setItem(STORAGE_KEY, decodeURIComponent(match[1]))
    } catch {
      // Storage unavailable (private mode); the SPA keeps working from the fragment.
    }
    history.replaceState(null, '', window.location.pathname + window.location.search)
  }
  return getToken()
}

export function getToken(): string | null {
  try {
    return localStorage.getItem(STORAGE_KEY)
  } catch {
    return null
  }
}

export function clearToken(): void {
  try {
    localStorage.removeItem(STORAGE_KEY)
  } catch {
    // ignore
  }
}