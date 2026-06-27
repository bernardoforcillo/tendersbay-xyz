import { createRouter, RouterProvider } from '@tanstack/react-router';
import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { I18nextProvider } from 'react-i18next';
import {
  AnalyticsErrorBoundary,
  AnalyticsProvider,
  initAnalytics,
  useLocaleProperty,
} from '~/analytics';
import { ConsentProvider, CookieConsentBanner } from '~/features/consent';
import { i18n } from '~/i18n';
import { routeTree } from '~/routeTree.gen';
import '~/index.css';

initAnalytics();

const router = createRouter({ routeTree });

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router;
  }
}

function LocaleProperty() {
  useLocaleProperty();
  return null;
}

const root = document.getElementById('root');
if (!root) {
  throw new Error('root element not found');
}

createRoot(root).render(
  <StrictMode>
    <AnalyticsProvider>
      <AnalyticsErrorBoundary>
        <I18nextProvider i18n={i18n}>
          <ConsentProvider>
            <LocaleProperty />
            <RouterProvider router={router} />
            <CookieConsentBanner />
          </ConsentProvider>
        </I18nextProvider>
      </AnalyticsErrorBoundary>
    </AnalyticsProvider>
  </StrictMode>,
);
