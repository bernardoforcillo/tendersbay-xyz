import { create } from 'zustand';
import { createJSONStorage, persist } from 'zustand/middleware';

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

export const useAuthStore = create<AuthStore>()(
  persist(
    (set) => ({
      accessToken: null,
      user: null,
      isAuthenticated: false,
      setAuth: (token, user) => set({ accessToken: token, user, isAuthenticated: true }),
      clearAuth: () => set({ accessToken: null, user: null, isAuthenticated: false }),
    }),
    {
      name: 'auth',
      storage: createJSONStorage(() => sessionStorage),
    },
  ),
);

export type { AuthStore, AuthUser };
