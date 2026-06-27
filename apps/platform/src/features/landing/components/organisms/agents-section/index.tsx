import { useTranslation } from 'react-i18next';
import { Eyebrow, type IconName, Reveal } from '~/features/landing/components/atoms';
import { AgentStep } from '~/features/landing/components/molecules';

type Item = { title: string; body: string };
const ICONS: IconName[] = ['search', 'document', 'trophy'];

export function AgentsSection() {
  const { t } = useTranslation();
  const items = t('landing.agents.items', { returnObjects: true }) as Item[];

  return (
    <section
      id="agents"
      aria-labelledby="agents-title"
      className="scroll-mt-24 bg-brand-700 py-24 text-white md:py-28"
    >
      <div className="mx-auto max-w-6xl px-6">
        <Reveal>
          <Eyebrow icon="sparkle" className="border-white/25 bg-white/10 text-white">
            {t('landing.agents.eyebrow')}
          </Eyebrow>
          <h2
            id="agents-title"
            className="mt-5 max-w-[20ch] font-display text-[2rem] leading-[1.05] tracking-tight text-white md:text-[2.7rem]"
          >
            {t('landing.agents.title')}
          </h2>
        </Reveal>

        <div className="relative mt-14 grid gap-12 md:grid-cols-3 md:gap-8">
          {items.map((item, i) => (
            <Reveal key={item.title} delay={i * 0.1}>
              <AgentStep
                index={i + 1}
                icon={ICONS[i] ?? 'sparkle'}
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
