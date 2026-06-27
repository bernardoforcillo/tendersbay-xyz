import { useReducedMotion } from 'motion/react';
import { useTranslation } from 'react-i18next';
import { CountryFlag, Eyebrow, Marquee } from '~/features/landing/components/atoms';
import {
  EU_COUNTRIES,
  type EuCountry,
} from '~/features/landing/components/atoms/country-flag/flags';
import { PORTALS } from '~/features/landing/components/atoms/country-flag/portals';

/** Live coverage. Empty now (teaser); add an ISO code to light up that flag. */
const AVAILABLE: ReadonlySet<EuCountry> = new Set<EuCountry>([]);

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
  const total = EU_COUNTRIES.length;

  const flags = EU_COUNTRIES.map((code) => {
    const available = AVAILABLE.has(code);
    return {
      code,
      name: countryName(i18n.language, code),
      portal: PORTALS[code],
      available,
      statusLabel: available ? availableLabel : comingSoonLabel,
    };
  });
  const availableCount = flags.filter((f) => f.available).length;
  const comingCount = total - availableCount;
  const rows = [flags.slice(0, 9), flags.slice(9, 18), flags.slice(18, 27)];

  return (
    <section
      id="coverage"
      aria-labelledby="coverage-title"
      className="scroll-mt-24 bg-ink-950 py-24 text-ink-100 md:py-28"
    >
      <div className="mx-auto max-w-6xl px-6">
        <div className="flex flex-col gap-8 md:flex-row md:items-end md:justify-between">
          <div className="max-w-[52ch]">
            <Eyebrow icon="globe" className="border-white/15 bg-white/5 text-brand-300">
              {t('landing.coverage.eyebrow')}
            </Eyebrow>
            <h2
              id="coverage-title"
              className="mt-5 font-display text-[2rem] leading-[1.05] tracking-tight text-white md:text-[2.7rem]"
            >
              {t('landing.coverage.title')}
            </h2>
            <p className="mt-5 text-lg leading-relaxed text-ink-200">
              {t('landing.coverage.body')}
            </p>
          </div>

          <dl className="grid shrink-0 gap-2.5 rounded-2xl border border-white/10 bg-white/5 px-5 py-4 shadow-soft backdrop-blur-sm">
            <div className="flex items-center justify-between gap-8">
              <dt className="flex items-center gap-2 font-mono text-[11px] font-semibold uppercase tracking-[0.12em] text-ink-100">
                <span className="h-2 w-2 rounded-full bg-brand-400" />
                {availableLabel}
              </dt>
              <dd className="font-mono text-sm tabular-nums text-white">{availableCount}</dd>
            </div>
            <div className="h-px bg-white/10" />
            <div className="flex items-center justify-between gap-8">
              <dt className="flex items-center gap-2 font-mono text-[11px] font-semibold uppercase tracking-[0.12em] text-ink-400">
                <span className="h-2 w-2 rounded-full bg-white/25" />
                {comingSoonLabel}
              </dt>
              <dd className="font-mono text-sm tabular-nums text-ink-400">{comingCount}</dd>
            </div>
          </dl>
        </div>

        {reduce ? (
          <ul className="mt-12 grid grid-cols-3 gap-3 sm:grid-cols-6 lg:grid-cols-9">
            {flags.map((f) => (
              <li key={f.code}>
                <CountryFlag
                  code={f.code}
                  name={f.name}
                  portal={f.portal}
                  available={f.available}
                  statusLabel={f.statusLabel}
                  className="w-full"
                />
              </li>
            ))}
          </ul>
        ) : (
          <div className="-mx-6 mt-12 flex flex-col gap-3 px-6 [mask-image:linear-gradient(to_right,transparent,#000_6%,#000_94%,transparent)]">
            {rows.map((row, i) => (
              <Marquee key={row[0]?.code ?? i} reverse={i % 2 === 1} durationSec={58 + i * 6}>
                {row.map((f) => (
                  <CountryFlag
                    key={f.code}
                    code={f.code}
                    name={f.name}
                    portal={f.portal}
                    available={f.available}
                    statusLabel={f.statusLabel}
                    className="w-36"
                  />
                ))}
              </Marquee>
            ))}
          </div>
        )}

        <p className="mt-8 font-mono text-[11px] font-semibold uppercase tracking-[0.16em] text-ink-400">
          {t('landing.coverage.note')}
        </p>
      </div>
    </section>
  );
}
