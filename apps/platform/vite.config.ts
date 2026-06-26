import { fileURLToPath, URL } from 'node:url';
import tailwindcss from '@tailwindcss/vite';
import { tanstackRouter } from '@tanstack/router-plugin/vite';
import { seo } from '@tendersbay/vite-plugin-seo';
import react from '@vitejs/plugin-react-swc';
import { defineConfig } from 'vite';
import { DEFAULT_LOCALE, SUPPORTED_LOCALES } from './src/i18n/locales';

export default defineConfig({
  plugins: [
    tanstackRouter({ target: 'react', autoCodeSplitting: true }),
    react(),
    tailwindcss(),
    seo({
      hostname: 'https://tendersbay.xyz',
      locales: SUPPORTED_LOCALES,
      defaultLocale: DEFAULT_LOCALE,
      siteName: 'tendersbay',
      description:
        'AI agents that find, prepare, and help you win EU public tenders — across all 24 official EU languages.',
      themeColor: '#0f172a',
      organization: {
        name: 'tendersbay',
        url: 'https://tendersbay.xyz',
        logo: 'https://tendersbay.xyz/favicon.svg',
      },
    }),
  ],
  resolve: {
    // `~` aliases the app's src/ directory: `~/App` -> `src/App`, bare `~` -> `src`.
    // Keep in sync with the `paths` entry in tsconfig.json.
    alias: {
      '~': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    port: 5173,
  },
  build: {
    outDir: 'dist',
    // Keep emptyOutDir off so the committed dist/.gitkeep placeholder (which
    // //go:embed all:dist needs to compile on a clean checkout) survives builds.
    emptyOutDir: false,
  },
});
