import { useTranslation } from 'react-i18next';
import { Reveal } from '~/features/landing/components/atoms';

type ProofItem = { value: string; label: string };

/**
 * Proof-of-the-prize strip. Where competitors flex proprietary database scale +
 * logos, tendersbay flexes the *public* reality of the prize — real, sourced EU
 * procurement figures with a visible citation. Light cream band that continues
 * the hero field, JetBrains Mono numbers (the data type per the type system).
 */
export function ProofStrip() {
  const { t } = useTranslation();
  const items = t('landing.proof.items', { returnObjects: true }) as ProofItem[];

  return (
    <section
      id="proof"
      aria-labelledby="proof-lead"
      className="scroll-mt-24 bg-cream-100 pb-20 md:pb-28"
    >
      <div className="mx-auto max-w-6xl px-6">
        <Reveal>
          <p
            id="proof-lead"
            className="max-w-[30ch] font-display text-2xl leading-[1.12] tracking-tight text-ink-900 md:max-w-[34ch] md:text-[2rem]"
          >
            {t('landing.proof.lead')}
          </p>
        </Reveal>

        <ul className="mt-12 grid gap-10 sm:grid-cols-3 sm:gap-8">
          {items.map((item, i) => (
            <li key={item.value}>
              <Reveal delay={i * 0.08}>
                <div className="border-brand-600/70 border-t-2 pt-4">
                  <p className="font-mono text-4xl font-semibold text-brand-700 tabular-nums tracking-tight md:text-5xl">
                    {item.value}
                  </p>
                  <p className="mt-2.5 max-w-[24ch] text-[15px] leading-snug text-ink-600">
                    {item.label}
                  </p>
                </div>
              </Reveal>
            </li>
          ))}
        </ul>

        <Reveal>
          <p className="mt-10 font-mono text-[11px] uppercase tracking-[0.14em] text-ink-400">
            {t('landing.proof.source')}
          </p>
        </Reveal>
      </div>
    </section>
  );
}
