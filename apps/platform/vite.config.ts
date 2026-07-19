import { readFileSync } from 'node:fs';
import { fileURLToPath, URL } from 'node:url';
import tailwindcss from '@tailwindcss/vite';
import { tanstackRouter } from '@tanstack/router-plugin/vite';
import { seo } from '@tendersbay/vite-plugin-seo';
import react from '@vitejs/plugin-react-swc';
import { defineConfig } from 'vite';
import { DEFAULT_LOCALE, SUPPORTED_LOCALES } from './src/i18n/locales';

// Per-locale head/content copy for the seo plugin's dist/<locale>/index.html
// emission (localized meta + FAQPage JSON-LD + <noscript> hero/FAQ block). Read
// with fs at config time (not imported as modules — config-time module
// resolution is fragile for raw-TS/JSON workspace imports). Completeness across
// all 24 locales is guarded by the landing-*-keys tests.
type Card = { title?: string; body?: string };
const localeMeta: Record<string, { title: string; description: string }> = {};
const localeFaq: Record<string, { question: string; answer: string }[]> = {};
const localeHero: Record<string, { headline: string; subtitle: string }> = {};

for (const locale of SUPPORTED_LOCALES) {
  const file = fileURLToPath(
    new URL(`./src/assets/locales/${locale}/common.json`, import.meta.url),
  );
  const landing = JSON.parse(readFileSync(file, 'utf8')).landing;

  const meta = landing?.meta;
  if (!meta?.title || !meta?.description) {
    throw new Error(`${locale}/common.json is missing landing.meta.title/description`);
  }
  localeMeta[locale] = { title: meta.title as string, description: meta.description as string };

  const hero = landing?.hero;
  if (!hero?.titleLead || !hero?.titleHighlight || !hero?.subtitle) {
    throw new Error(`${locale}/common.json is missing landing.hero title/subtitle`);
  }
  localeHero[locale] = {
    headline: `${hero.titleLead} ${hero.titleHighlight}`,
    subtitle: hero.subtitle as string,
  };

  const items = landing?.assurance?.items as Card[] | undefined;
  if (!Array.isArray(items) || items.some((c) => !c.title || !c.body)) {
    throw new Error(`${locale}/common.json is missing a filled landing.assurance.items block`);
  }
  localeFaq[locale] = items.map((c) => ({ question: c.title as string, answer: c.body as string }));
}

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
      // Keep only genuinely indexable pages in the sitemap. Auth and workspace
      // routes are token-gated / transactional (login, signup, password reset,
      // email verification, invite acceptance): no SEO value, and Google flags
      // them as thin/soft-404. Re-add a specific route here if it should rank.
      exclude: ['/auth/*', '/workspace/*'],
      siteName: 'tendersbay',
      title: defaultMeta.title,
      description: defaultMeta.description,
      localeMeta,
      localeFaq,
      localeHero,
      themeColor: '#0f172a',
      organization: {
        name: 'tendersbay',
        url: 'https://tendersbay.xyz',
        logo: 'https://tendersbay.xyz/favicon.svg',
      },
      service: {
        name: 'tendersbay',
        description:
          'A team of AI agents that find the right public tenders across the EU, prepare the procurement paperwork, and help SMEs win the award.',
        serviceType: 'Public procurement tender discovery and bid preparation',
        areaServed: 'European Union',
      },
      llmsIntro:
        'tendersbay is a team of AI agents for SMEs and entrepreneurs. They find the best public tenders across Europe, prepare the document bureaucracy (including the ESPD and national equivalents), and help small teams win the award — across all 24 official EU languages. Not a translation tool: agents that hunt, prepare and win. Pre-launch.',
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
