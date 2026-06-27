import { useTranslation } from 'react-i18next';
import { type IconName, Reveal } from '~/features/landing/components/atoms';
import { ValueCard } from '~/features/landing/components/molecules';

type Item = { title: string; body: string };
const ICONS: IconName[] = ['check', 'document', 'layers', 'check'];

export function AssuranceSection() {
  const { t } = useTranslation();
  const items = t('landing.assurance.items', { returnObjects: true }) as Item[];

  return (
    <section
      id="assurance"
      aria-labelledby="assurance-title"
      className="scroll-mt-24 bg-cream-50 py-24 md:py-28"
    >
      <div className="mx-auto max-w-6xl px-6">
        <Reveal className="flex flex-col items-center text-center">
          <h2
            id="assurance-title"
            className="max-w-[22ch] font-display text-[2rem] leading-[1.05] tracking-tight text-ink-900 md:text-[2.7rem]"
          >
            {t('landing.assurance.title')}
          </h2>
        </Reveal>
        <div className="mt-12 grid gap-5 md:grid-cols-2">
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
