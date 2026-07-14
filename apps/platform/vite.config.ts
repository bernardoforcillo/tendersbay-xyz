import { readFileSync } from 'node:fs';
import { fileURLToPath, URL } from 'node:url';
import tailwindcss from '@tailwindcss/vite';
import { tanstackRouter } from '@tanstack/router-plugin/vite';
import { seo } from '@tendersbay/vite-plugin-seo';
import react from '@vitejs/plugin-react-swc';
import { defineConfig } from 'vite';
import { DEFAULT_LOCALE, SUPPORTED_LOCALES } from './src/i18n/locales';

// Per-locale <title>/description for the seo plugin's dist/<locale>/index.html
// emission. Read with fs at config time (not imported as modules — config-time
// module resolution is fragile for raw-TS/JSON workspace imports).
const localeMeta = Object.fromEntries(
  SUPPORTED_LOCALES.map((locale) => {
    const file = fileURLToPath(
      new URL(`./src/assets/locales/${locale}/common.json`, import.meta.url),
    );
    const meta = JSON.parse(readFileSync(file, 'utf8')).landing?.meta;
    if (!meta?.title || !meta?.description) {
      throw new Error(`${locale}/common.json is missing landing.meta.title/description`);
    }
    return [locale, { title: meta.title as string, description: meta.description as string }];
  }),
);

const defaultMeta = localeMeta[DEFAULT_LOCALE];
if (!defaultMeta) {
  throw new Error(`default locale ${DEFAULT_LOCALE} has no landing.meta entry`);
}

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
      title: defaultMeta.title,
      description: defaultMeta.description,
      localeMeta,
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
