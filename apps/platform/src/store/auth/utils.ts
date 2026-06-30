export function isTokenExpired(token: string): boolean {
  try {
    const payload = token.split('.')[1]
    if (!payload) return true
    const { exp } = JSON.parse(atob(payload)) as { exp: number }
    return Date.now() / 1000 > exp
  } catch {
    return true
  }
}
