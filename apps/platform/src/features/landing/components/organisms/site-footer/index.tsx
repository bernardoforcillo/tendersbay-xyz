import { Link } from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import { Logo } from '~/features/landing/components/atoms';
import {
  FooterColumn,
  type FooterLink,
  LanguageSwitcher,
  SocialLinks,
} from '~/features/landing/components/molecules';

const CONTACT_EMAIL = 'me@bernardoforcillo.com';

// Hrefs paired by index with the localized labels in `landing.footer.columns`.
// Column 0 = Product (real in-page anchors); 1 = Company, 2 = Resources (placeholders).
const COLUMN_HREFS: string[][] = [
  ['#agents', '#coverage', '#vision'],
  ['#', '#', '#', '#'],
  ['#', '#', '#', '#'],
];

type Column = { heading: string; links: string[] };

export function SiteFooter() {
  const { t } = useTranslation();
  const columns = t('landing.footer.columns', { returnObjects: true }) as Column[];

  return (
    <footer id="site-footer" className="relative overflow-hidden bg-ink-950 text-ink-200">
      <div className="mx-auto max-w-6xl px-6 pb-44 pt-16 md:pt-20">
        <div className="grid gap-12 md:grid-cols-[1.5fr_1fr_1fr_1fr]">
          <div className="flex max-w-sm flex-col items-start gap-5">
            <Logo className="text-white [&>span]:text-brand-400" />
            <p className="text-sm leading-relaxed text-ink-300">{t('landing.footer.tagline')}</p>
            <SocialLinks label={t('landing.footer.social')} />
            <LanguageSwitcher />
            <Link
              href={`mailto:${CONTACT_EMAIL}`}
              className="text-sm font-semibold text-brand-300 no-underline outline-none data-[hovered]:text-brand-200 data-[focus-visible]:underline"
            >
              {t('landing.footer.contactLabel')}
            </Link>
          </div>
          {columns.map((column, i) => (
            <FooterColumn
              key={column.heading}
              heading={column.heading}
              links={column.links.map(
                (label, j): FooterLink => ({ label, href: COLUMN_HREFS[i]?.[j] ?? '#' }),
              )}
            />
          ))}
        </div>
        <div className="mt-16 border-t border-white/10 pt-8 text-xs text-ink-400">
          {t('landing.footer.copyright')}
        </div>
      </div>
      <div
        aria-hidden="true"
        className="pointer-events-none absolute inset-x-0 bottom-0 flex select-none justify-center overflow-hidden"
      >
        <span className="translate-y-1/4 whitespace-nowrap font-display leading-none text-[clamp(4rem,20vw,18rem)] text-white/[0.04]">
          tendersbay
        </span>
      </div>
    </footer>
  );
}
