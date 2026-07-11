import { defineConfig } from 'vitest/config';

export default defineConfig({
  test: {
    environment: 'jsdom',
    environmentOptions: {
      jsdom: {
        // jsdom 25+ requires a url before it initializes Storage.
        url: 'http://localhost',
      },
    },
    setupFiles: ['./vitest.setup.ts'],
  },
});
