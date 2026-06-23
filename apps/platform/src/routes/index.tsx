import { createFileRoute, redirect } from '@tanstack/react-router';
import { detectLocale } from '~/i18n/detect-locale';

export const Route = createFileRoute('/')({
  beforeLoad: () => {
    throw redirect({ to: '/$locale', params: { locale: detectLocale() } });
  },
});
