import { useQueryState } from 'nuqs';
import { useCallback } from 'react';

export const REDIRECT_PARAM = 'redirect';

/**
 * Returns a safe same-origin path, defaulting to `/`. Rejects anything that
 * isn't a single-leading-slash relative path so a crafted `?redirect=` can never
 * bounce the user off-site after authentication (open-redirect guard):
 *   - protocol-relative (`//host`)
 *   - the backslash trick (`/\host`, `/\/host`) — browsers normalise `\` to `/`,
 *     turning it into a protocol-relative URL
 *   - absolute / scheme-qualified URLs (`https://…`, `javascript:…`) — they don't
 *     start with `/`
 */
export function sanitizeRedirect(path: string | null | undefined): string {
  if (!path) return '/';
  if (!path.startsWith('/')) return '/';
  // Second character must not turn the value protocol-relative once the browser
  // normalises backslashes: reject `//…` and `/\…`.
  if (path[1] === '/' || path[1] === '\\') return '/';
  return path;
}

/**
 * Reads/writes the `?redirect=` query param via nuqs, exposing the sanitized
 * internal target used after login/signup or when accepting an invite.
 */
export function useRedirectParam() {
  const [raw, setRaw] = useQueryState(REDIRECT_PARAM);
  const target = sanitizeRedirect(raw);
  const setTarget = useCallback((path: string | null) => setRaw(path), [setRaw]);
  return { raw, target, setTarget };
}
