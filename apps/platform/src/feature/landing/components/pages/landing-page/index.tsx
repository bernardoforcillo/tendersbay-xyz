import { useTranslation } from 'react-i18next';
import { LanguageSwitcher } from '~/feature/landing/components/molecules';

export function LandingPage() {
  const { t } = useTranslation();

  return (
    <main className="relative flex min-h-screen flex-col items-center justify-center gap-4 bg-slate-50 text-slate-900">
      <div className="absolute top-4 right-4">
        <LanguageSwitcher />
      </div>
      <h1 className="text-4xl font-bold text-blue-600">{t('app.title')}</h1>
      <p className="text-slate-600">{t('app.subtitle')}</p>
    </main>
  );
}
