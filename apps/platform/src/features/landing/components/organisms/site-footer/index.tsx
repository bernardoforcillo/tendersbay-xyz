import { Link } from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import { Logo } from '~/features/landing/components/atoms';
import { LanguageSwitcher } from '~/features/landing/components/molecules';

const CONTACT_EMAIL = 'me@bernardoforcillo.com';

export function SiteFooter() {
  const { t } = useTranslation();
  return (
    <footer className="border-t border-white/10 bg-ink-950 py-12 text-ink-200">
      <div className="mx-auto flex max-w-6xl flex-col gap-8 px-6 md:flex-row md:items-start md:justify-between">
        <div className="max-w-sm">
          <Logo className="text-white [&>span]:text-brand-400" />
          <p className="mt-3 text-sm leading-relaxed text-ink-300">{t('landing.footer.tagline')}</p>
        </div>
        <div className="flex flex-col items-start gap-4 md:items-end">
          <LanguageSwitcher />
          <Link
            href={`mailto:${CONTACT_EMAIL}`}
            className="text-sm font-semibold text-brand-300 no-underline outline-none data-[hovered]:text-brand-200 data-[focus-visible]:underline"
          >
            {t('landing.footer.contactLabel')}
          </Link>
        </div>
      </div>
      <div className="mx-auto mt-10 max-w-6xl px-6 text-xs text-ink-400">
        {t('landing.footer.copyright')}
      </div>
    </footer>
  );
}
