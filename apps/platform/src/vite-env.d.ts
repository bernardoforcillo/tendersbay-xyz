/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_POSTHOG_KEY?: string;
  readonly VITE_POSTHOG_HOST?: string;
  readonly VITE_API_URL?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}

interface Window {
  /**
   * Runtime configuration injected into index.html by the Go server
   * (apps/platform/internal/server). Empty under Vite dev, where the app falls
   * back to import.meta.env.
   */
  __ENV__?: {
    API_URL?: string;
    POSTHOG_KEY?: string;
    POSTHOG_HOST?: string;
  };
}
