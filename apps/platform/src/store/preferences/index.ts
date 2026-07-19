import { create } from 'zustand';
import { persist } from 'zustand/middleware';

interface PreferencesState {
  /** Set of confirmation keys the user chose to skip. */
  skipConfirmations: Record<string, boolean>;
  setSkipConfirmation: (key: string, skip: boolean) => void;
  shouldSkip: (key: string) => boolean;
}

export const usePreferencesStore = create<PreferencesState>()(
  persist(
    (set, get) => ({
      skipConfirmations: {},
      setSkipConfirmation: (key, skip) =>
        set((s) => ({
          skipConfirmations: { ...s.skipConfirmations, [key]: skip },
        })),
      shouldSkip: (key) => get().skipConfirmations[key] === true,
    }),
    { name: 'tendersbay-preferences' },
  ),
);
