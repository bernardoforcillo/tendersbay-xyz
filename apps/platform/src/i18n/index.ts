import i18next from 'i18next';
import { initReactI18next } from 'react-i18next';
import { DEFAULT_LOCALE, SUPPORTED_LOCALES } from './detect-locale';

// Eagerly bundle every locale's `common` namespace as a static import.
const modules = import.meta.glob('../assets/locales/*/common.json', {
  eager: true,
  import: 'default',
}) as Record<string, Record<string, unknown>>;

const resources: Record<string, { common: Record<string, unknown> }> = {};
for (const [filePath, message] of Object.entries(modules)) {
  const locale = filePath.split('/').at(-2);
  if (locale) {
    resources[locale] = { common: message };
  }
}

const i18n = i18next.createInstance();

i18n.use(initReactI18next).init({
  resources,
  lng: DEFAULT_LOCALE,
  fallbackLng: DEFAULT_LOCALE,
  supportedLngs: [...SUPPORTED_LOCALES],
  ns: ['common'],
  defaultNS: 'common',
  // Keep locale codes lowercase to match our resource keys (e.g. 'en-ie' not 'en-IE').
  lowerCaseLng: true,
  // Synchronous init so `i18n.t` works immediately with the bundled resources.
  // (renamed from `initImmediate: false` in i18next v26)
  initAsync: false,
  interpolation: { escapeValue: false },
  react: { useSuspense: false },
});

export { i18n };
