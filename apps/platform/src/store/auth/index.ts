import { create } from 'zustand';

interface AuthUser {
  id: string;
  email: string;
  displayName: string;
}

interface AuthStore {
  accessToken: string | null;
  user: AuthUser | null;
  isAuthenticated: boolean;
  setAuth: (token: string, user: AuthUser) => void;
  clearAuth: () => void;
}

// Memory-only by design: the access token is NOT persisted to web storage, so
// an XSS payload can't lift it out of sessionStorage/localStorage. On a reload
// the store starts empty and `bootstrap()` in main.tsx silently re-mints an
// access token from the HttpOnly `refresh_token` cookie (unreachable from JS)
// before the app renders, so this costs no visible session continuity.
export const useAuthStore = create<AuthStore>()((set) => ({
  accessToken: null,
  user: null,
  isAuthenticated: false,
  setAuth: (token, user) => set({ accessToken: token, user, isAuthenticated: true }),
  clearAuth: () => set({ accessToken: null, user: null, isAuthenticated: false }),
}));

export type { AuthStore, AuthUser };
