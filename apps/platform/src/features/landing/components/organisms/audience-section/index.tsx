import { useTranslation } from 'react-i18next';
import { type IconName, Reveal } from '~/features/landing/components/atoms';
import { ValueCard } from '~/features/landing/components/molecules';

type Item = { title: string; body: string };
const ICONS: IconName[] = ['clock', 'trophy', 'layers'];

export function AudienceSection() {
  const { t } = useTranslation();
  const items = t('landing.audience.items', { returnObjects: true }) as Item[];

  return (
    <section
      id="audience"
      aria-labelledby="audience-title"
      className="scroll-mt-24 bg-cream-100 py-28 md:py-32"
    >
      <div className="mx-auto max-w-6xl px-6">
        <Reveal className="flex flex-col items-center text-center">
          <h2
            id="audience-title"
            className="max-w-[20ch] font-display text-[2rem] leading-[1.05] tracking-[-0.015em] text-ink-900 md:text-[2.7rem]"
          >
            {t('landing.audience.title')}
          </h2>
          <span
            aria-hidden="true"
            className="mt-8 h-px w-16 bg-gradient-to-r from-brand-400 to-brand-600"
          />
        </Reveal>
        <div className="mt-12 grid gap-5 md:grid-cols-3">
          {items.map((item, i) => (
            <Reveal key={item.title} delay={i * 0.08}>
              <ValueCard
                tone="solution"
                icon={ICONS[i] ?? 'check'}
                title={item.title}
                body={item.body}
              />
            </Reveal>
          ))}
        </div>
      </div>
    </section>
  );
}
