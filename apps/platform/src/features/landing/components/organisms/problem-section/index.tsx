import { useTranslation } from 'react-i18next';
import { Eyebrow, type IconName, Reveal } from '~/features/landing/components/atoms';
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
      className="scroll-mt-24 bg-cream-100 py-20"
    >
      <div className="mx-auto max-w-6xl px-6">
        <Reveal>
          <Eyebrow icon="layers">{t('landing.problem.eyebrow')}</Eyebrow>
          <h2
            id="problem-title"
            className="mt-5 max-w-[18ch] font-display text-[2rem] leading-[1.05] tracking-tight text-ink-900 md:text-[2.7rem]"
          >
            {t('landing.problem.title')}
          </h2>
        </Reveal>
        <div className="mt-10 grid gap-5 md:grid-cols-3">
          {items.map((item, i) => (
            <Reveal key={item.title} delay={i * 0.08}>
              <ValueCard icon={ICONS[i] ?? 'layers'} title={item.title} body={item.body} />
            </Reveal>
          ))}
        </div>
      </div>
    </section>
  );
}
