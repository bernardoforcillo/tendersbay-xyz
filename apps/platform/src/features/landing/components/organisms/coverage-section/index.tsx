import { motion, useReducedMotion, type Variants } from 'motion/react';
import { useTranslation } from 'react-i18next';
import { CountryFlag, Eyebrow } from '~/features/landing/components/atoms';
import {
  EU_COUNTRIES,
  type EuCountry,
} from '~/features/landing/components/atoms/country-flag/flags';
import { PORTALS } from '~/features/landing/components/atoms/country-flag/portals';

/** Live coverage. Empty now (teaser); add an ISO code to light up that flag. */
const AVAILABLE: ReadonlySet<EuCountry> = new Set<EuCountry>([]);

const containerVariants: Variants = {
  hidden: {},
  show: { transition: { staggerChildren: 0.03 } },
};
const itemVariants: Variants = {
  hidden: { opacity: 0, y: 12 },
  show: { opacity: 1, y: 0, transition: { duration: 0.4, ease: 'easeOut' } },
};

function countryName(locale: string, code: string): string {
  try {
    return new Intl.DisplayNames([locale], { type: 'region' }).of(code) ?? code;
  } catch {
    return code;
  }
}

export function CoverageSection() {
  const { t, i18n } = useTranslation();
  const reduce = useReducedMotion();
  const availableLabel = t('landing.coverage.statusAvailable');
  const comingSoonLabel = t('landing.coverage.statusComingSoon');

  return (
    <section
      id="coverage"
      aria-labelledby="coverage-title"
      className="scroll-mt-24 bg-cream-50 py-20"
    >
      <div className="mx-auto max-w-6xl px-6">
        <div className="max-w-[58ch]">
          <Eyebrow icon="globe">{t('landing.coverage.eyebrow')}</Eyebrow>
          <h2
            id="coverage-title"
            className="mt-5 font-display text-[2rem] leading-[1.05] tracking-tight text-ink-900 md:text-[2.7rem]"
          >
            {t('landing.coverage.title')}
          </h2>
          <p className="mt-5 text-lg leading-relaxed text-ink-600">{t('landing.coverage.body')}</p>
        </div>

        <motion.ul
          className="mt-10 grid grid-cols-4 gap-3 sm:grid-cols-6 lg:grid-cols-9"
          variants={reduce ? undefined : containerVariants}
          initial={reduce ? undefined : 'hidden'}
          whileInView={reduce ? undefined : 'show'}
          viewport={{ once: true, margin: '-80px' }}
        >
          {EU_COUNTRIES.map((code) => {
            const isAvailable = AVAILABLE.has(code);
            return (
              <CountryFlag
                key={code}
                code={code}
                name={countryName(i18n.language, code)}
                portal={PORTALS[code]}
                available={isAvailable}
                statusLabel={isAvailable ? availableLabel : comingSoonLabel}
                variants={reduce ? undefined : itemVariants}
              />
            );
          })}
        </motion.ul>

        <p className="mt-8 font-mono text-[11px] font-semibold uppercase tracking-[0.16em] text-ink-500">
          {t('landing.coverage.note')}
        </p>
      </div>
    </section>
  );
}
