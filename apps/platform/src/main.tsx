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
import { authClient } from '~/lib/api/client';
import { routeTree } from '~/routeTree.gen';
import { useAuthStore } from '~/store/auth';
import { isTokenExpired } from '~/store/auth/utils';
import '~/index.css';

initAnalytics();

const router = createRouter({
  routeTree,
  context: { auth: useAuthStore.getState() },
});

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router;
  }
}

function LocaleProperty() {
  useLocaleProperty();
  return null;
}

async function bootstrap() {
  const { accessToken, clearAuth, setAuth } = useAuthStore.getState();

  if (!accessToken || isTokenExpired(accessToken)) {
    try {
      const res = await authClient.refreshToken({});
      if (!res.user) throw new Error('refreshToken response missing user');
      setAuth(res.accessToken, {
        id: res.user.id,
        email: res.user.email,
        displayName: res.user.displayName,
      });
    } catch {
      clearAuth();
    }
  }

  router.update({ context: { auth: useAuthStore.getState() } });

  useAuthStore.subscribe((state) => {
    router.update({ context: { auth: state } });
    void router.invalidate();
  });

  const root = document.getElementById('root');
  if (!root) throw new Error('root element not found');

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
}

bootstrap();
