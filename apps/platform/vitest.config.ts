import { fileURLToPath, URL } from 'node:url';
import { defineConfig } from 'vitest/config';

export default defineConfig({
  resolve: {
    alias: {
      '~': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  test: {
    environment: 'jsdom',
    environmentOptions: {
      jsdom: {
        // jsdom 25+ requires a url to enable localStorage/sessionStorage.
        url: 'http://localhost',
      },
    },
    setupFiles: ['./vitest.setup.ts'],
  },
});
