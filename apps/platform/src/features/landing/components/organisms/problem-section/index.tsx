import { useTranslation } from 'react-i18next';
import { type IconName, Reveal } from '~/features/landing/components/atoms';
import { ValueCard } from '~/features/landing/components/molecules';

type Item = { title: string; body: string };
const ICONS: IconName[] = ['map', 'layers', 'clock'];

export function ProblemSection() {
  const { t } = useTranslation();
  const items = t('landing.problem.items', { returnObjects: true }) as Item[];

  return (
    <section
      id="problem"
      aria-labelledby="problem-title"
      className="scroll-mt-24 bg-ink-900 py-24 text-ink-100 md:py-28"
    >
      <div className="mx-auto max-w-6xl px-6">
        <Reveal>
          <h2
            id="problem-title"
            className="max-w-[18ch] font-display text-[2rem] leading-[1.05] tracking-tight text-white md:text-[2.7rem]"
          >
            {t('landing.problem.title')}
          </h2>
        </Reveal>
        <div className="mt-10 grid gap-5 md:grid-cols-3">
          {items.map((item, i) => (
            <Reveal key={item.title} delay={i * 0.08}>
              <ValueCard
                tone="solution"
                icon={ICONS[i] ?? 'layers'}
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
