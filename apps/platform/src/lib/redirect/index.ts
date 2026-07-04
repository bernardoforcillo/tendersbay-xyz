import { useQueryState } from 'nuqs';
import { useCallback } from 'react';

export const REDIRECT_PARAM = 'redirect';

/**
 * Returns a safe internal path, defaulting to `/`. Rejects protocol-relative
 * (`//host`) and absolute (`https://…`) URLs so a crafted `?redirect=` can never
 * bounce the user off-site after authentication (open-redirect guard).
 */
export function sanitizeRedirect(path: string | null | undefined): string {
  if (!path) return '/';
  if (!path.startsWith('/') || path.startsWith('//')) return '/';
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
