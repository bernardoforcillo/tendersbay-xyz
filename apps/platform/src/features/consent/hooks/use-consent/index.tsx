import {
  createContext,
  type ReactNode,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
} from 'react';
import { getAnalytics } from '~/analytics';

export type ConsentStatus = 'granted' | 'denied' | null;

const STORAGE_KEY = 'tb_consent';

type ConsentContextValue = {
  status: ConsentStatus;
  grant: () => void;
  deny: () => void;
};

const ConsentContext = createContext<ConsentContextValue | null>(null);

function readStored(): ConsentStatus {
  try {
    const value = localStorage.getItem(STORAGE_KEY);
    return value === 'granted' || value === 'denied' ? value : null;
  } catch {
    return null;
  }
}

function persist(value: Exclude<ConsentStatus, null>): void {
  try {
    localStorage.setItem(STORAGE_KEY, value);
  } catch {
    // Ignore storage failures (private mode, quota); state still lives in memory.
  }
}

export function ConsentProvider({ children }: { children: ReactNode }) {
  const [status, setStatus] = useState<ConsentStatus>(() => readStored());

  // Re-apply a persisted choice to PostHog once, on mount.
  useEffect(() => {
    const posthog = getAnalytics();
    if (!posthog) {
      return;
    }
    const stored = readStored();
    if (stored === 'granted') {
      posthog.opt_in_capturing();
    } else if (stored === 'denied') {
      posthog.opt_out_capturing();
    }
  }, []);

  const grant = useCallback(() => {
    persist('granted');
    getAnalytics()?.opt_in_capturing();
    setStatus('granted');
  }, []);

  const deny = useCallback(() => {
    persist('denied');
    getAnalytics()?.opt_out_capturing();
    setStatus('denied');
  }, []);

  const value = useMemo(() => ({ status, grant, deny }), [status, grant, deny]);
  return <ConsentContext.Provider value={value}>{children}</ConsentContext.Provider>;
}

export function useConsent(): ConsentContextValue {
  const context = useContext(ConsentContext);
  if (!context) {
    throw new Error('useConsent must be used within a ConsentProvider');
  }
  return context;
}
